package handler

import (
	"byone-arena/internal/delivery/websocket"
	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"
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
}

// SessionHandler menangani HTTP request untuk manajemen sesi rental
type SessionHandler struct {
	sessionUC usecase.SessionUseCase
	validator *validator.Validator
	hub       *websocket.Hub
	db        *gorm.DB
}

// NewSessionHandler membuat instance baru SessionHandler
func NewSessionHandler(sessionUC usecase.SessionUseCase, v *validator.Validator, hub *websocket.Hub, db *gorm.DB) *SessionHandler {
	return &SessionHandler{sessionUC: sessionUC, validator: v, hub: hub, db: db}
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

	result, err := h.sessionUC.StartSession(c.Context(), req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	h.hub.Broadcast(websocket.NewEvent(websocket.EventSessionStarted, result))
	// Auto-wake TV
	h.hub.Broadcast(websocket.NewEvent(websocket.EventTVWake, fiber.Map{
		"consoleId": req.ConsoleID,
	}))

	return response.Created(c, "Sesi rental berhasil dimulai dan pembayaran lunas", result)
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

	// Cek unpaid pending payments (dari extend yg belum bayar)
	var unpaidRow struct{ Count int64 `gorm:"column:count"` }
	h.db.WithContext(c.Context()).Raw(
		"SELECT COUNT(*) as count FROM payments WHERE session_id = ? AND payment_status = 'pending'", id,
	).Scan(&unpaidRow)

	if unpaidRow.Count > 0 {
		// Insert notifikasi: ada perpanjangan yg belum dibayar
		consoleIDStr := session.ConsoleID.String()
		h.db.WithContext(c.Context()).Exec(`
			INSERT INTO tv_notifications (id, title, message, target_all, target_console_ids, active_sessions_only, loop_enabled, is_active, priority, created_at, updated_at)
			VALUES (uuid_generate_v4(), 'Pembayaran Tertunda', ?, false, ?, false, false, true, 'high', NOW(), NOW())
		`, fmt.Sprintf("Unit %s memiliki %d perpanjangan sesi yang belum dibayar. Segera konfirmasi pembayaran.", session.Console.Name, unpaidRow.Count),
			fmt.Sprintf(`["%s"]`, consoleIDStr))
	}

	h.hub.Broadcast(websocket.NewEvent(websocket.EventSessionEnded, session))
	// Auto-sleep TV
	h.hub.Broadcast(websocket.NewEvent(websocket.EventTVSleep, fiber.Map{
		"consoleId": session.ConsoleID,
	}))

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

	h.hub.Broadcast(websocket.NewEvent(websocket.EventSessionCancelled, fiber.Map{"session_id": id}))

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
		`SELECT * FROM "byoneExtendSession"(?, ?, ?, ?, ?, ?)`,
		id, req.AdditionalMinutes, req.CashReceived, req.PayNow, req.VoucherCode, req.Notes,
	).Scan(&result)

	if tx.Error != nil {
		return response.BadRequest(c, tx.Error.Error())
	}

	// Broadcast ke TV bahwa waktu ditambah
	// Ambil console_id dulu (pakai struct untuk GORM Scan)
	var consoleRow struct{ ConsoleID uuid.UUID `gorm:"column:console_id"` }
	h.db.WithContext(c.Context()).Raw("SELECT console_id FROM sessions WHERE id = ?", id).Scan(&consoleRow)
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

	h.hub.Broadcast(websocket.NewEvent(websocket.EventSessionExtended, map[string]interface{}{
		"type":              "session_extended",
		"sessionId":         result.SessionID,
		"consoleId":         consoleID,
		"message":           msg,
		"additionalMinutes": req.AdditionalMinutes,
		"totalBookedMinutes": result.SessionBookedMinutes,
		"endScheduledAt":    result.SessionEndScheduled,
	}))

	// Insert notifikasi untuk client polling
	consoleIDStr := consoleID.String()
	h.db.WithContext(c.Context()).Exec(`
		INSERT INTO tv_notifications (id, title, message, target_all, target_console_ids, active_sessions_only, loop_enabled, is_active, priority, created_at, updated_at)
		VALUES (uuid_generate_v4(), 'Sesi Diperpanjang', ?, false, ?, true, false, true, 'high', NOW(), NOW())
	`, msg, fmt.Sprintf(`["%s"]`, consoleIDStr))

	return response.OK(c, "Waktu sewa berhasil ditambah", fiber.Map{
		"session": fiber.Map{
			"id":                result.SessionID,
			"bookedDurationMinutes": result.SessionBookedMinutes,
			"endScheduledAt":    result.SessionEndScheduled,
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
