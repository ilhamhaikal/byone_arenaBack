package handler

import (
	"byone-arena/internal/delivery/websocket"
	"byone-arena/internal/license"
	"byone-arena/internal/promo"
	"byone-arena/internal/usecase"
	"byone-arena/pkg/audit"
	"byone-arena/pkg/response"
	"byone-arena/pkg/spname"
	"byone-arena/pkg/validator"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
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
		rule, ruleErr := promo.ActiveRuleToday(h.db.WithContext(c.Context()))
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

	result, err := h.sessionUC.StartSession(c.Context(), req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	if result.Payment != nil {
		finalizePaymentMethod(h.db, result.Payment.ID, method)
		if isCashless(method) {
			result.Payment.CashReceived = result.Payment.TotalPayment
			result.Payment.ChangeAmount = 0
		}
	}

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

	// Catat pelaku transaksi (tanpa ini selisih kas tidak bisa ditelusuri).
	if uid := audit.CurrentUserID(c); uid != nil {
		h.db.WithContext(c.Context()).Exec(
			"SELECT "+spname.Ident("SetSessionCreatedBy")+"(?::uuid, ?::uuid)", result.Session.ID, *uid)
		h.db.WithContext(c.Context()).Exec(
			"SELECT "+spname.Ident("SetPaymentCreatedBy")+"(?::uuid, ?::uuid)", result.Session.ID, *uid)
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

	if unpaidRow.Count > 0 {
		// Insert notifikasi: ada perpanjangan yg belum dibayar
		h.db.WithContext(c.Context()).Exec(
			"SELECT "+spname.Ident("InsertTvNotification")+"(?::text, ?::text, ?::text, ?::boolean, ?::varchar)",
			"Pembayaran Tertunda",
			fmt.Sprintf("Unit %s memiliki %d perpanjangan sesi yang belum dibayar. Segera konfirmasi pembayaran.", session.Console.Name, unpaidRow.Count),
			consoleIDStr, false, "high")
	}

	// Cleanup notifikasi "Sesi Diperpanjang" — sudah tidak relevan setelah sesi berakhir
	h.db.WithContext(c.Context()).Exec(
		"SELECT "+spname.Ident("DeactivateTvNotification")+"(?::text, ?::text)",
		"Sesi Diperpanjang", consoleIDStr)

	// Broadcast realtime — sesi berakhir + auto-sleep TV (lihat broadcaster.go)
	h.hub.BroadcastSessionEnded(session, session.ConsoleID)

	return response.OK(c, "Sesi rental berhasil diakhiri", session)
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

	h.hub.BroadcastSessionCancelled(id)

	return response.OK(c, "Sesi rental berhasil dibatalkan", nil)
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
	tx := h.db.WithContext(c.Context()).Raw(
		fmt.Sprintf(`SELECT * FROM %s(?, ?, ?, ?, ?, ?)`, spname.Ident("ExtendSession")),
		id, req.AdditionalMinutes, req.CashReceived, req.PayNow, req.VoucherCode, req.Notes,
	).Scan(&result)

	if tx.Error != nil {
		return response.BadRequest(c, tx.Error.Error())
	}

	if req.PayNow {
		finalizePaymentMethod(h.db, result.PaymentID, method)
		if isCashless(method) {
			result.PaymentCashReceived = result.PaymentTotal
			result.PaymentChange = 0
		}
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
	})

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
