package handler

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"byone-arena/internal/delivery/websocket"
	"byone-arena/internal/domain/entity"
	"byone-arena/internal/license"
	"byone-arena/internal/promo"
	"byone-arena/internal/usecase"
	"byone-arena/pkg/audit"
	"byone-arena/pkg/response"
	"byone-arena/pkg/spname"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ExtendSessionRequest payload untuk menambah waktu sewa
type ExtendSessionRequest struct {
	AdditionalMinutes int     `json:"additionalMinutes" validate:"required,min=1" example:"120"`
	PayNow            bool    `json:"payNow"                                        example:"true"`  // true = langsung bayar, false = tunda bayar
	CashReceived      float64 `json:"cashReceived"      validate:"omitempty,gte=0"  example:"50000"` // wajib jika payNow=true
	VoucherCode       string  `json:"voucherCode"                                    example:""`
	Notes             string  `json:"notes"                                          example:"Tambah 2 jam"`
	// Metode pembayaran: cash (default), qris, atau transfer.
	PaymentMethod string `json:"paymentMethod" validate:"omitempty,oneof=cash qris transfer" example:"cash"`
}

// SessionHandler menangani HTTP request untuk manajemen sesi rental
type SessionHandler struct {
	sessionUC usecase.SessionUseCase
	validator *validator.Validator
	hub       *websocket.Hub
	db        *gorm.DB
	license   *license.Service
}

// NewSessionHandler membuat instance baru SessionHandler
func NewSessionHandler(sessionUC usecase.SessionUseCase, v *validator.Validator, hub *websocket.Hub, db *gorm.DB, licenseSvc *license.Service) *SessionHandler {
	return &SessionHandler{sessionUC: sessionUC, validator: v, hub: hub, db: db, license: licenseSvc}
}

// deactivateSessionTvNotifications menonaktifkan notifikasi sistem/transient
// milik satu konsol. Notifikasi tetap tersimpan sebagai riwayat, tetapi tidak
// akan muncul lagi saat sesi baru dimulai. Notifikasi buatan admin (promo/
// himbauan) tidak termasuk karena judulnya berbeda.
//
// Seluruh logika query berada di stored procedure
// "__SP__DeactivateSessionTvNotifications" (migrasi 000047). Jangan menambah
// raw SQL di handler: perbaikan aturan query cukup lewat CREATE OR REPLACE
// FUNCTION di migrasi baru, tanpa build ulang backend.
func deactivateSessionTvNotifications(db *gorm.DB, consoleID uuid.UUID) {
	if db == nil {
		return
	}
	db.Exec(
		"SELECT "+spname.Ident("DeactivateSessionTvNotifications")+"(?::text)",
		consoleID.String())
}

// wibLocation mengembalikan zona Asia/Jakarta (fallback UTC+7).
func wibLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err == nil {
		return loc
	}
	return time.FixedZone("WIB", 7*60*60)
}

// isWeekendWIBAt true bila tanggal t (dalam WIB) jatuh pada Sabtu/Minggu.
func isWeekendWIBAt(t time.Time) bool {
	t = t.In(wibLocation())
	return t.Weekday() == time.Saturday || t.Weekday() == time.Sunday
}

// isWeekendWIB true bila hari ini Sabtu/Minggu menurut zona Asia/Jakarta.
func isWeekendWIB() bool {
	return isWeekendWIBAt(time.Now())
}

// isPackageRecoveryWindowWIBAt true bila t adalah Sabtu/Minggu pukul 21:00 WIB.
func isPackageRecoveryWindowWIBAt(t time.Time) bool {
	t = t.In(wibLocation())
	return isWeekendWIBAt(t) && t.Hour() == 21
}

// isPackageRecoveryWindowWIB true bila sekarang Sabtu/Minggu pukul 21:00 WIB.
func isPackageRecoveryWindowWIB() bool {
	return isPackageRecoveryWindowWIBAt(time.Now())
}

// saveTimeResult adalah hasil stored procedure SaveNonMemberTime.
type saveTimeResult struct {
	Saved            bool       `gorm:"column:saved"`
	Reason           string     `gorm:"column:reason"`
	SessionID        *uuid.UUID `gorm:"column:session_id"`
	ConsoleID        *uuid.UUID `gorm:"column:console_id"`
	RemainingMinutes int        `gorm:"column:remaining_minutes"`
	ExpiresAt        *time.Time `gorm:"column:expires_at"`
}

// saveNonMemberTime menjalankan SP simpan waktu. Semua aturan (member,
// pembayaran pending, sisa waktu, expiry 24 jam) ada di stored procedure.
func (h *SessionHandler) saveNonMemberTime(c *fiber.Ctx, id uuid.UUID, reason string) (*saveTimeResult, error) {
	var res saveTimeResult
	err := h.db.WithContext(c.Context()).Raw(
		"SELECT * FROM "+spname.Ident("SaveNonMemberTime")+"(?::uuid, ?::text)",
		id, reason,
	).Scan(&res).Error
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// saveTimeReasonMessage menerjemahkan alasan penolakan SP menjadi pesan UI.
func saveTimeReasonMessage(reason string) string {
	switch reason {
	case "member":
		return "Sesi member memakai Bank Waktu. Tombol Simpan Waktu hanya untuk non-member."
	case "pending":
		return "Masih ada pembayaran tertunda. Lunasi pembayaran terlebih dahulu."
	case "unpaid":
		return "Sesi gratis tidak memiliki waktu berbayar untuk disimpan."
	case "no_remaining":
		return "Tidak ada sisa waktu yang bisa disimpan."
	case "not_active":
		return "Sesi sudah tidak aktif."
	case "not_found":
		return "Sesi tidak ditemukan."
	default:
		return "Sisa waktu tidak dapat disimpan."
	}
}

// GetAll godoc
// @Summary      Ambil semua sesi rental
// @Description  Mengembalikan daftar seluruh sesi rental (aktif, selesai, dibatalkan)
// @Tags         Sesi Rental
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]entity.Session}
// @Failure      401  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /api/v1/sessions [get]
func (h *SessionHandler) GetAll(c *fiber.Ctx) error {
	sessions, err := h.sessionUC.GetAllSessions(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil data sesi")
	}
	return response.OK(c, "Data sesi berhasil diambil", sessions)
}

// GetActive godoc
// @Summary      Ambil semua sesi aktif
// @Description  Mengembalikan sesi yang sedang berjalan saat ini di semua konsol
// @Tags         Sesi Rental
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]entity.Session}
// @Failure      401  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /api/v1/sessions/active [get]
func (h *SessionHandler) GetActive(c *fiber.Ctx) error {
	sessions, err := h.sessionUC.GetActiveSessions(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil data sesi aktif")
	}
	return response.OK(c, "Sesi aktif berhasil diambil", sessions)
}

// GetByID godoc
// @Summary      Ambil sesi berdasarkan ID
// @Description  Mengembalikan detail satu sesi rental beserta data konsol dan pelanggan
// @Tags         Sesi Rental
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Session ID (UUID)"
// @Success      200  {object}  response.Response{data=entity.Session}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/sessions/{id} [get]
func (h *SessionHandler) GetByID(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	session, err := h.sessionUC.GetSessionByID(c.Context(), id)
	if err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.OK(c, "Data sesi berhasil diambil", session)
}

// Start godoc
// @Summary      Mulai sesi rental + pembayaran di depan
// @Description  Memulai sesi rental untuk konsol dan langsung menyelesaikan pembayaran di depan dalam satu transaksi.\n\n**Alur:**\n1. Konsol harus berstatus `available`\n2. Harga dihitung dari `bookedDurationMinutes × pricePerHour / 60`\n3. Diskon otomatis (happy hour, member) diterapkan\n4. Voucher opsional diterapkan jika `voucherCode` diberikan\n5. `cashReceived` harus ≥ harga setelah diskon\n\nResponse mencakup `session` (dengan `endScheduledAt` untuk countdown) dan `payment` (dengan `changeAmount` kembalian).\n\nEvent realtime `session_started` dikirim ke semua client WebSocket.
// @Tags         Sesi Rental
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      usecase.StartSessionRequest   true  "Data sesi dan pembayaran"
// @Success      201   {object}  response.Response{data=usecase.StartSessionResponse}
// @Failure      400   {object}  response.ErrorResponse  "Konsol tidak tersedia, uang kurang, atau voucher tidak valid"
// @Failure      401   {object}  response.ErrorResponse
// @Failure      500   {object}  response.ErrorResponse
// @Router       /api/v1/sessions/start [post]
func (h *SessionHandler) Start(c *fiber.Ctx) error {
	req := new(usecase.StartSessionRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	// Lisensi: blokir sesi baru bila sudah lewat masa tenggang.
	if h.license != nil {
		if err := h.license.EnsureCanStartBusiness(c.Context()); err != nil {
			return response.Forbidden(c, err.Error())
		}
	}

	// Validasi awal promo agar pengguna mendapat pesan jelas sebelum sesi dibuat.
	if req.UsePromoBonus {
		rule, ruleErr := promo.ActiveRuleForCustomer(
			h.db.WithContext(c.Context()), req.CustomerID)
		if ruleErr != nil {
			return response.BadRequest(c, "Tidak ada promo bonus waktu yang berlaku hari ini")
		}
		if !promo.Eligible(rule, req.BookedDurationMinutes) {
			return response.BadRequest(c, fmt.Sprintf(
				"Promo %s butuh minimal %d menit (bonus %d menit)",
				rule.Name, rule.MinPaidMinutes, rule.BonusMinutes))
		}
	}

	method := normalizePaymentMethod(req.PaymentMethod)
	if isCashless(method) {
		// QRIS/transfer: uang diterima = total (tidak ada kembalian).
		req.CashReceived = paymentMethodSentinel
	}
	if uid, ok := c.Locals("userID").(uuid.UUID); ok {
		req.CreatedBy = &uid
	}

	result, err := h.sessionUC.StartSession(c.Context(), req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	if result.Payment != nil && isCashless(method) {
		// Metode sudah difinalisasi atomik di repository; koreksi nilai
		// response agar sesuai dengan baris payment yang tersimpan.
		result.Payment.PaymentMethod = entity.PaymentMethod(method)
		result.Payment.CashReceived = result.Payment.TotalPayment
		result.Payment.ChangeAmount = 0
	}

	// Sesi baru dimulai → live manual tanpa sesi (jika ada) berakhir.
	h.db.WithContext(c.Context()).Model(&entity.Console{}).
		Where("id = ?", req.ConsoleID).
		Update("manual_live", false)

	// Bersihkan notifikasi sistem dari sesi sebelumnya supaya tidak muncul
	// lagi di sesi yang baru. Riwayat tetap tersimpan (is_active=false).
	deactivateSessionTvNotifications(h.db.WithContext(c.Context()), req.ConsoleID)

	promoMessage := ""
	if req.UsePromoBonus {
		if updated, rule, applyErr := promo.Apply(h.db.WithContext(c.Context()), result.Session.ID.String()); applyErr == nil {
			result.Session = updated
			promoMessage = fmt.Sprintf(" + bonus %d menit (%s)", rule.BonusMinutes, rule.Name)
			audit.Log(h.db, c, "session.promo_bonus", "session", &result.Session.ID, map[string]any{
				"ruleName": rule.Name, "bonusMinutes": rule.BonusMinutes,
			})
		} else {
			promoMessage = " (bonus promo gagal diterapkan: " + applyErr.Error() + ")"
		}
	}

	audit.Log(h.db, c, "session.start", "session", &result.Session.ID, map[string]any{
		"consoleId":     result.Session.ConsoleID,
		"bookedMinutes": result.Session.BookedDurationMinutes,
		"totalPrice":    result.Session.TotalPrice,
	})

	// Broadcast realtime — sesi dimulai + auto-wake TV (lihat broadcaster.go)
	h.hub.BroadcastSessionStarted(result, req.ConsoleID)

	return response.Created(c, "Sesi rental berhasil dimulai dan pembayaran lunas"+promoMessage, result)
}

// End godoc
// @Summary      Akhiri sesi rental
// @Description  Mengakhiri sesi rental aktif, menghitung durasi dan total harga, membebaskan konsol. Event realtime dikirim.
// @Tags         Sesi Rental
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Session ID (UUID)"
// @Success      200  {object}  response.Response{data=entity.Session}
// @Failure      400  {object}  response.ErrorResponse  "Sesi tidak aktif atau tidak ditemukan"
// @Failure      401  {object}  response.ErrorResponse
// @Router       /api/v1/sessions/{id}/end [patch]
func (h *SessionHandler) End(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	// Non-member: "Akhiri" otomatis menyimpan sisa waktu berbayar 24 jam.
	// Member tetap memakai jalur EndSession (bank waktu tanpa kedaluwarsa).
	saved, saveErr := h.saveNonMemberTime(c, id, "ended_saved")
	if saveErr == nil && saved != nil && !saved.Saved && saved.Reason == "pending" {
		// Jangan akhiri sesi non-member yang masih punya tagihan pending:
		// sisa waktunya tidak boleh hilang. UI EndSessionDialog sudah
		// melunasi pending sebelum memanggil endpoint ini.
		return response.BadRequest(c, saveTimeReasonMessage("pending"))
	}
	if saveErr == nil && saved != nil && saved.Saved {
		var session entity.Session
		if err := h.db.WithContext(c.Context()).
			Preload("Console").First(&session, "id = ?", id).Error; err != nil {
			return response.InternalServerError(c, "Sesi tersimpan tetapi gagal dibaca ulang")
		}
		audit.Log(h.db, c, "session.end", "session", &id, map[string]any{
			"durationMinutes": session.DurationMinutes,
			"totalPrice":      session.TotalPrice,
			"savedMinutes":    saved.RemainingMinutes,
			"expiresAt":       saved.ExpiresAt,
		})
		if uid := audit.CurrentUserID(c); uid != nil {
			h.db.WithContext(c.Context()).Exec(
				"SELECT "+spname.Ident("SetSessionEndedBy")+"(?::uuid, ?::uuid)", id, *uid)
		}
		deactivateSessionTvNotifications(h.db.WithContext(c.Context()), session.ConsoleID)
		h.hub.BroadcastSessionEnded(&session, session.ConsoleID)
		return response.OK(c, fmt.Sprintf(
			"Sesi diakhiri. Sisa %d menit disimpan.", saved.RemainingMinutes), session)
	}

	session, err := h.sessionUC.EndSession(c.Context(), id)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	// Atribusi: siapa yang mengakhiri sesi.
	if uid := audit.CurrentUserID(c); uid != nil {
		h.db.WithContext(c.Context()).Exec(
			"SELECT "+spname.Ident("SetSessionEndedBy")+"(?::uuid, ?::uuid)", id, *uid)
	}
	audit.Log(h.db, c, "session.end", "session", &id, map[string]any{
		"durationMinutes": session.DurationMinutes,
		"totalPrice":      session.TotalPrice,
	})

	// Cek unpaid pending payments (dari extend yg belum bayar)
	var unpaidRow struct {
		Count int64 `gorm:"column:count"`
	}
	h.db.WithContext(c.Context()).Raw(
		"SELECT * FROM "+spname.Ident("CountPendingPayments")+"(?::uuid)", id,
	).Scan(&unpaidRow)

	consoleIDStr := session.ConsoleID.String()

	// Bersihkan notifikasi transient dari sesi yang baru berakhir sebelum
	// memasang notifikasi baru. Ini mencegah "Pesanan Disiapkan"/"Sesi
	// Diperpanjang"/"Pembayaran Tertunda" lama tampil lagi di sesi berikutnya.
	deactivateSessionTvNotifications(h.db.WithContext(c.Context()), session.ConsoleID)

	if unpaidRow.Count > 0 {
		// Insert notifikasi: ada perpanjangan yg belum dibayar
		h.db.WithContext(c.Context()).Exec(
			"SELECT "+spname.Ident("InsertTvNotification")+"(?::text, ?::text, ?::text, ?::boolean, ?::varchar)",
			"Pembayaran Tertunda",
			fmt.Sprintf("Unit %s memiliki %d perpanjangan sesi yang belum dibayar. Segera konfirmasi pembayaran.", session.Console.Name, unpaidRow.Count),
			consoleIDStr, false, "high")
	}

	// Broadcast realtime — sesi berakhir + auto-sleep TV (lihat broadcaster.go)
	h.hub.BroadcastSessionEnded(session, session.ConsoleID)

	return response.OK(c, "Sesi rental berhasil diakhiri", session)
}

// SaveTime godoc
// @Summary      Simpan sisa waktu sesi non-member
// @Description  Mengakhiri sesi non-member dan menyimpan sisa waktu berbayar (24 jam reguler, 7 hari paket). Member tetap memakai Bank Waktu.
// @Tags         Sesi Rental
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Session ID (UUID)"
// @Success      200 {object} response.Response{data=entity.Session}
// @Failure      400 {object} response.ErrorResponse
// @Router       /api/v1/sessions/{id}/save-time [post]
func (h *SessionHandler) SaveTime(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	res, err := h.saveNonMemberTime(c, id, "saved_by_admin")
	if err != nil {
		return response.InternalServerError(c,
			"Gagal menyimpan waktu. Coba lagi atau hubungi admin.")
	}
	if res == nil || !res.Saved {
		reason := "unknown"
		if res != nil {
			reason = res.Reason
		}
		return response.BadRequest(c, saveTimeReasonMessage(reason))
	}

	var session entity.Session
	if err := h.db.WithContext(c.Context()).
		Preload("Console").First(&session, "id = ?", id).Error; err != nil {
		return response.InternalServerError(c, "Sesi tersimpan tetapi gagal dibaca ulang")
	}
	audit.Log(h.db, c, "session.save_time", "session", &id, map[string]any{
		"remainingMinutes": res.RemainingMinutes,
		"expiresAt":        res.ExpiresAt,
	})
	if uid := audit.CurrentUserID(c); uid != nil {
		h.db.WithContext(c.Context()).Exec(
			"SELECT "+spname.Ident("SetSessionEndedBy")+"(?::uuid, ?::uuid)", id, *uid)
	}
	deactivateSessionTvNotifications(h.db.WithContext(c.Context()), session.ConsoleID)
	h.hub.BroadcastSessionEnded(&session, session.ConsoleID)
	return response.OK(c, fmt.Sprintf(
		"Sisa %d menit disimpan.", res.RemainingMinutes), session)
}

// Cancel godoc
// @Summary      Batalkan sesi rental
// @Description  Membatalkan sesi rental aktif tanpa tagihan. Konsol dikembalikan ke status available. Event realtime dikirim.
// @Tags         Sesi Rental
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Session ID (UUID)"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.ErrorResponse  "Sesi tidak aktif atau tidak ditemukan"
// @Failure      401  {object}  response.ErrorResponse
// @Router       /api/v1/sessions/{id}/cancel [patch]
func (h *SessionHandler) Cancel(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	if err := h.sessionUC.CancelSession(c.Context(), id); err != nil {
		return response.BadRequest(c, err.Error())
	}

	if uid := audit.CurrentUserID(c); uid != nil {
		h.db.WithContext(c.Context()).Exec(
			"SELECT "+spname.Ident("SetSessionCancelledBy")+"(?::uuid, ?::uuid)", id, *uid)
	}
	audit.Log(h.db, c, "session.cancel", "session", &id, map[string]any{
		"note": "dibatalkan tanpa tagihan",
	})

	var consoleRow struct {
		ConsoleID uuid.UUID `gorm:"column:console_id"`
	}
	h.db.WithContext(c.Context()).Raw(
		"SELECT * FROM "+spname.Ident("SessionConsoleID")+"(?::uuid)", id).Scan(&consoleRow)

	// Sesi dibatalkan → jangan bawa notifikasi sistem sesi ini ke sesi baru.
	deactivateSessionTvNotifications(h.db.WithContext(c.Context()), consoleRow.ConsoleID)

	h.hub.BroadcastSessionCancelled(id, consoleRow.ConsoleID)

	return response.OK(c, "Sesi rental berhasil dibatalkan", nil)
}

// Recover godoc
// @Summary      Pulihkan sesi yang terputus (listrik mati / TV offline)
// @Description  Memulai sesi baru dengan sisa waktu dari sesi interrupted dan
// voucher GRATIS (100%). Ditautkan ke sesi asal dan dicatat di audit log.
// @Tags         Sesi Rental
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Session ID (sesi interrupted)"
// @Success      201 {object} response.Response
// @Router       /api/v1/sessions/{id}/recover [post]
func (h *SessionHandler) Recover(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	if h.license != nil {
		if err := h.license.EnsureCanStartBusiness(c.Context()); err != nil {
			return response.Forbidden(c, err.Error())
		}
	}

	var original entity.Session
	var result *usecase.StartSessionResponse

	// Row lock pada sesi asal mencegah dua request recover paralel membuat
	// dua sesi pemulihan. Semua update relasi dilakukan dalam transaksi ini.
	err = h.db.WithContext(c.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&original, "id = ?", id).Error; err != nil {
			return err
		}
		if original.Status != entity.SessionStatusInterrupted {
			return errors.New("Sesi ini bukan sesi terputus")
		}
		if original.RecoveredBySessionID != nil {
			return errors.New("Sesi ini sudah dipulihkan")
		}
		if original.RemainingRecoveryMinutes <= 0 {
			return errors.New("Tidak ada sisa waktu yang bisa dipulihkan")
		}
		if original.SavedTimeExpiresAt != nil &&
			!original.SavedTimeExpiresAt.After(time.Now()) {
			return errors.New("Waktu simpan sudah kedaluwarsa")
		}
		if original.IsPackage && !isPackageRecoveryWindowWIB() {
			return errors.New("Sesi paket hanya bisa dipulihkan pada Sabtu/Minggu pukul 21:00 WIB")
		}

		var activeCount int64
		if err := tx.Model(&entity.Session{}).
			Where("console_id = ? AND status = 'active'", original.ConsoleID).
			Count(&activeCount).Error; err != nil {
			return err
		}
		if activeCount > 0 {
			return errors.New("Konsol masih dipakai sesi aktif. Akhiri sesi tersebut dulu.")
		}

		req := &usecase.StartSessionRequest{
			ConsoleID:             original.ConsoleID,
			CustomerID:            original.CustomerID,
			BookedDurationMinutes: original.RemainingRecoveryMinutes,
			CashReceived:          0,
			VoucherCode:           "GRATIS",
			PaymentMethod:         "cash",
			IsPackage:             original.IsPackage,
			Notes: fmt.Sprintf("Pemulihan sesi %s (terputus) — sisa %d menit",
				original.ID, original.RemainingRecoveryMinutes),
		}
		if uid, ok := c.Locals("userID").(uuid.UUID); ok {
			req.CreatedBy = &uid
		}
		started, startErr := h.sessionUC.StartSession(c.Context(), req)
		if startErr != nil {
			return startErr
		}
		result = started

		// Sesi paket yang dipulihkan harus mulai pada slot 21:00 WIB, lalu
		// berjalan selama sisa menit (tidak dipaksa sampai 06:00).
		if original.IsPackage {
			nowWIB := time.Now().In(wibLocation())
			slotStart := time.Date(nowWIB.Year(), nowWIB.Month(), nowWIB.Day(),
				21, 0, 0, 0, wibLocation())
			slotEnd := slotStart.Add(time.Duration(original.RemainingRecoveryMinutes) * time.Minute)
			if !slotEnd.After(time.Now()) {
				return errors.New("Sisa waktu paket sudah habis untuk slot 21:00 ini")
			}
			if err := tx.Model(&entity.Session{}).
				Where("id = ?", result.Session.ID).
				Updates(map[string]interface{}{
					"start_time":       slotStart,
					"end_scheduled_at": slotEnd,
				}).Error; err != nil {
				return err
			}
			result.Session.StartTime = slotStart
			result.Session.EndScheduledAt = &slotEnd
		}

		if err := tx.Model(&entity.Session{}).
			Where("id = ?", result.Session.ID).
			Update("recovered_from_session_id", original.ID).Error; err != nil {
			return err
		}
		if err := tx.Model(&entity.Session{}).
			Where("id = ?", original.ID).
			Updates(map[string]interface{}{
				"status":                  entity.SessionStatusRecovered,
				"recovered_by_session_id": result.Session.ID,
				"updated_at":              time.Now(),
			}).Error; err != nil {
			return err
		}
		return tx.Model(&entity.Console{}).
			Where("id = ?", original.ConsoleID).
			Update("manual_live", false).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "Sesi tidak ditemukan")
		}
		return response.BadRequest(c, err.Error())
	}
	if result == nil {
		return response.InternalServerError(c, "Sesi pemulihan tidak terbentuk")
	}

	audit.Log(h.db, c, "session.recovery", "session", &original.ID, map[string]any{
		"recoveredSessionId": result.Session.ID,
		"remainingMinutes":   original.RemainingRecoveryMinutes,
		"voucher":            "GRATIS",
	})

	// Bersihkan notifikasi sistem dari sesi lama; sesi pemulihan adalah sesi
	// baru sehingga pesan lama tidak boleh ikut muncul.
	deactivateSessionTvNotifications(h.db.WithContext(c.Context()), original.ConsoleID)

	// TV langsung masuk sesi baru (HDMI + overlay).
	h.hub.BroadcastSessionStarted(result, original.ConsoleID)

	return response.Created(c, "Sesi berhasil dipulihkan dengan voucher GRATIS", result)
}

// Extend godoc
// @Summary      Tambah waktu sewa (extend session)
// @Description  Menambah durasi sewa untuk sesi yang sedang aktif.\n//\n**payNow:**\n- `true` — langsung bayar (cashReceived wajib > 0, status PAID)\n- `false` — tunda bayar (cashReceived opsional, status PENDING)\n//\nMinimal tambahan 1 menit. Voucher opsional.
// @Tags         Sesi Rental
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string               true  "Session ID"
// @Param        body  body      handler.ExtendSessionRequest  true  "Data tambah waktu"
// @Success      200   {object}  response.Response
// @Failure      400   {object}  response.ErrorResponse
// @Router       /api/v1/sessions/{id}/extend [post]
func (h *SessionHandler) Extend(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	req := new(ExtendSessionRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	method := normalizePaymentMethod(req.PaymentMethod)
	if req.PayNow && isCashless(method) {
		// QRIS/transfer: uang diterima dianggap pas (tidak ada kembalian).
		req.CashReceived = paymentMethodSentinel
	}

	// Validasi: jika payNow=true, cashReceived wajib > 0
	if req.PayNow && req.CashReceived <= 0 {
		return response.BadRequest(c, "cashReceived harus > 0 jika payNow=true (langsung bayar)")
	}

	type spResult struct {
		SessionID            uuid.UUID  `gorm:"column:session_id"`
		SessionBookedMinutes int        `gorm:"column:session_booked_minutes"`
		SessionEndScheduled  *time.Time `gorm:"column:session_end_scheduled"`
		PaymentID            uuid.UUID  `gorm:"column:payment_id"`
		PaymentAmount        float64    `gorm:"column:payment_amount"`
		PaymentDiscount      float64    `gorm:"column:payment_discount"`
		PaymentTotal         float64    `gorm:"column:payment_total"`
		PaymentCashReceived  float64    `gorm:"column:payment_cash_received"`
		PaymentChange        float64    `gorm:"column:payment_change"`
		PaymentVoucherID     *uuid.UUID `gorm:"column:payment_voucher_id"`
		PaymentStatus        string     `gorm:"column:payment_status"`
	}

	var result spResult
	var createdBy *uuid.UUID
	if uid, ok := c.Locals("userID").(uuid.UUID); ok {
		createdBy = &uid
	}
	err = h.db.WithContext(c.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(
			fmt.Sprintf(`SELECT * FROM %s(?, ?, ?, ?, ?, ?)`, spname.Ident("ExtendSession")),
			id, req.AdditionalMinutes, req.CashReceived, req.PayNow, req.VoucherCode, req.Notes,
		).Scan(&result).Error; err != nil {
			return err
		}
		if createdBy != nil {
			if err := tx.Model(&entity.Payment{}).
				Where("id = ?", result.PaymentID).
				Update("created_by", *createdBy).Error; err != nil {
				return err
			}
		}
		if req.PayNow {
			return finalizePaymentMethod(tx, result.PaymentID, method)
		}
		return nil
	})
	if err != nil {
		if strings.Contains(err.Error(), "PACKAGE_NO_EXTEND") {
			return response.BadRequest(c,
				"Sesi paket tidak bisa diperpanjang. Waktu paket mengikuti jadwal.")
		}
		return response.BadRequest(c, err.Error())
	}

	if req.PayNow && isCashless(method) {
		result.PaymentCashReceived = result.PaymentTotal
		result.PaymentChange = 0
	}

	// Broadcast ke TV bahwa waktu ditambah
	// Ambil console_id dulu (pakai struct untuk GORM Scan)
	var consoleRow struct {
		ConsoleID uuid.UUID `gorm:"column:console_id"`
	}
	h.db.WithContext(c.Context()).Raw(
		"SELECT * FROM "+spname.Ident("SessionConsoleID")+"(?::uuid)", id).Scan(&consoleRow)
	consoleID := consoleRow.ConsoleID

	hours := req.AdditionalMinutes / 60
	mins := req.AdditionalMinutes % 60
	var msg string
	if hours > 0 && mins > 0 {
		msg = fmt.Sprintf("Waktu Anda ditambah %d jam %d menit", hours, mins)
	} else if hours > 0 {
		msg = fmt.Sprintf("Waktu Anda ditambah %d jam", hours)
	} else {
		msg = fmt.Sprintf("Waktu Anda ditambah %d menit", mins)
	}

	h.hub.BroadcastSessionExtended(map[string]interface{}{
		"type":               "session_extended",
		"sessionId":          result.SessionID,
		"consoleId":          consoleID,
		"message":            msg,
		"additionalMinutes":  req.AdditionalMinutes,
		"totalBookedMinutes": result.SessionBookedMinutes,
		"endScheduledAt":     result.SessionEndScheduled,
	}, consoleID)

	// Insert notifikasi untuk client polling
	consoleIDStr := consoleID.String()
	// Deactivate old "Sesi Diperpanjang" notif untuk console ini sebelum insert baru
	h.db.WithContext(c.Context()).Exec(
		"SELECT "+spname.Ident("DeactivateTvNotification")+"(?::text, ?::text)",
		"Sesi Diperpanjang", consoleIDStr)
	h.db.WithContext(c.Context()).Exec(
		"SELECT "+spname.Ident("InsertTvNotification")+"(?::text, ?::text, ?::text, ?::boolean, ?::varchar)",
		"Sesi Diperpanjang", msg, consoleIDStr, true, "high")

	return response.OK(c, "Waktu sewa berhasil ditambah", fiber.Map{
		"session": fiber.Map{
			"id":                    result.SessionID,
			"bookedDurationMinutes": result.SessionBookedMinutes,
			"endScheduledAt":        result.SessionEndScheduled,
		},
		"payment": fiber.Map{
			"id":             result.PaymentID,
			"amount":         result.PaymentAmount,
			"discountAmount": result.PaymentDiscount,
			"totalPayment":   result.PaymentTotal,
			"cashReceived":   result.PaymentCashReceived,
			"changeAmount":   result.PaymentChange,
			"voucherId":      result.PaymentVoucherID,
			"status":         result.PaymentStatus,
		},
	})
}
