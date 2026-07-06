package handler

import (
	"byone-arena/internal/delivery/websocket"
	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PaymentHandler menangani HTTP request untuk manajemen pembayaran
type PaymentHandler struct {
	paymentUC usecase.PaymentUseCase
	validator *validator.Validator
	hub       *websocket.Hub
	db        *gorm.DB
}

// NewPaymentHandler membuat instance baru PaymentHandler
func NewPaymentHandler(paymentUC usecase.PaymentUseCase, v *validator.Validator, hub *websocket.Hub, db *gorm.DB) *PaymentHandler {
	return &PaymentHandler{paymentUC: paymentUC, validator: v, hub: hub, db: db}
}

// GetByID godoc
// @Summary      Ambil pembayaran berdasarkan ID
// @Description  Mengembalikan detail transaksi pembayaran tunai
// @Tags         Pembayaran
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Payment ID (UUID)"
// @Success      200  {object}  response.Response{data=entity.Payment}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/payments/{id} [get]
func (h *PaymentHandler) GetByID(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	payment, err := h.paymentUC.GetPaymentByID(c.Context(), id)
	if err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.OK(c, "Data pembayaran berhasil diambil", payment)
}

// GetBySession godoc
// @Summary      Ambil pembayaran berdasarkan Session ID
// @Description  Mengembalikan data pembayaran untuk sesi rental tertentu
// @Tags         Pembayaran
// @Produce      json
// @Security     BearerAuth
// @Param        session_id  path      string  true  "Session ID (UUID)"
// @Success      200         {object}  response.Response{data=entity.Payment}
// @Failure      400         {object}  response.ErrorResponse
// @Failure      401         {object}  response.ErrorResponse
// @Failure      404         {object}  response.ErrorResponse  "Belum ada pembayaran untuk sesi ini"
// @Router       /api/v1/sessions/{session_id}/payment [get]
func (h *PaymentHandler) GetBySession(c *fiber.Ctx) error {
	sessionID, err := uuid.Parse(c.Params("session_id"))
	if err != nil {
		return response.BadRequest(c, "Format Session ID tidak valid")
	}

	payment, err := h.paymentUC.GetPaymentBySessionID(c.Context(), sessionID)
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil data pembayaran")
	}
	if payment == nil {
		return response.NotFound(c, "Belum ada pembayaran untuk sesi ini")
	}
	return response.OK(c, "Data pembayaran berhasil diambil", payment)
}

// Create godoc
// @Summary      Buat pembayaran tunai
// @Description  Memproses pembayaran tunai untuk sesi yang sudah selesai. Sertakan `voucherCode` untuk mendapatkan diskon. SP otomatis menghitung diskon, kembalian, dan mengubah status menjadi paid.
// @Tags         Pembayaran
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      usecase.CreateCashPaymentRequest  true  "Data pembayaran — voucherCode opsional"
// @Success      201   {object}  response.Response{data=entity.Payment}
// @Failure      400   {object}  response.ErrorResponse  "Uang kurang / voucher tidak valid / sesi tidak valid"
// @Failure      401   {object}  response.ErrorResponse
// @Failure      409   {object}  response.ErrorResponse  "Pembayaran sudah ada untuk sesi ini"
// @Router       /api/v1/payments [post]
func (h *PaymentHandler) Create(c *fiber.Ctx) error {
	req := new(usecase.CreateCashPaymentRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	payment, err := h.paymentUC.CreateCashPayment(c.Context(), req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	h.hub.Broadcast(websocket.NewEvent(websocket.EventPaymentCreated, payment))

	return response.Created(c, "Pembayaran tunai berhasil diproses", payment)
}

// Refund godoc
// @Summary      Proses refund pembayaran
// @Description  Membatalkan pembayaran dan mengembalikan status ke refunded. Hanya pembayaran berstatus paid yang bisa direfund.
// @Tags         Pembayaran
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Payment ID (UUID)"
// @Success      200  {object}  response.Response{data=entity.Payment}
// @Failure      400  {object}  response.ErrorResponse  "Pembayaran tidak dalam status paid"
// @Failure      401  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/payments/{id}/refund [patch]
func (h *PaymentHandler) Refund(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	payment, err := h.paymentUC.RefundPayment(c.Context(), id)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	h.hub.Broadcast(websocket.NewEvent(websocket.EventPaymentRefunded, payment))

	return response.OK(c, "Pembayaran berhasil direfund", payment)
}

// Confirm godoc
// @Summary      Konfirmasi pembayaran pending (admin)
// @Description  Admin mengkonfirmasi pembayaran tambahan (extend session) yang masih pending menjadi paid.
// @Tags         Pembayaran
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Payment ID (UUID)"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.ErrorResponse
// @Router       /api/v1/payments/{id}/confirm [post]
func (h *PaymentHandler) Confirm(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	type spResult struct {
		PaymentID     uuid.UUID  `gorm:"column:payment_id"`
		PaymentStatus string     `gorm:"column:payment_status"`
		PaidAt        interface{} `gorm:"column:paid_at"`
	}

	var result spResult
	tx := h.db.WithContext(c.Context()).Raw(
		`SELECT * FROM "byoneConfirmExtendPayment"(?)`, id,
	).Scan(&result)

	if tx.Error != nil {
		return response.BadRequest(c, tx.Error.Error())
	}

	return response.OK(c, "Pembayaran berhasil dikonfirmasi", fiber.Map{
		"paymentId": result.PaymentID,
		"status":    result.PaymentStatus,
		"paidAt":    result.PaidAt,
	})
}

