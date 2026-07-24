package handler

import (
	"byone-arena/internal/delivery/websocket"
	"byone-arena/internal/domain/entity"
	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"
	"byone-arena/pkg/spname"
	"byone-arena/pkg/validator"
	"fmt"
	"time"

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

// GetAllBySession godoc
// @Summary      Ambil SEMUA pembayaran untuk satu sesi + ringkasan total
// @Description  Satu sesi bisa punya lebih dari satu payment (payment awal + setiap perpanjangan/extend). Endpoint ini mengembalikan seluruh payment beserta ringkasan total dibayar/pending, agar frontend tidak salah hitung total tagihan.
// @Tags         Pembayaran
// @Produce      json
// @Security     BearerAuth
// @Param        session_id  path      string  true  "Session ID (UUID)"
// @Success      200         {object}  response.Response
// @Failure      400         {object}  response.ErrorResponse
// @Router       /api/v1/sessions/{session_id}/payments [get]
func (h *PaymentHandler) GetAllBySession(c *fiber.Ctx) error {
	sessionID, err := uuid.Parse(c.Params("session_id"))
	if err != nil {
		return response.BadRequest(c, "Format Session ID tidak valid")
	}

	payments, err := h.paymentUC.GetPaymentsBySessionID(c.Context(), sessionID)
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil data pembayaran")
	}

	var totalPaid, totalPending, totalAmount float64
	for _, p := range payments {
		totalAmount += p.TotalPayment
		switch p.PaymentStatus {
		case entity.PaymentStatusPaid:
			totalPaid += p.TotalPayment
		case entity.PaymentStatusPending:
			totalPending += p.TotalPayment
		}
	}

	return response.OK(c, "Data pembayaran sesi berhasil diambil", fiber.Map{
		"payments":     payments,
		"totalAmount":  totalAmount,  // jumlah seluruh payment (paid + pending), tidak termasuk refunded
		"totalPaid":    totalPaid,    // yang sudah lunas
		"totalPending": totalPending, // yang masih menunggu konfirmasi
	})
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

// ConfirmPaymentRequest adalah body opsional untuk konfirmasi pembayaran pending.
// Jika cashReceived diisi, backend akan menghitung ulang kembalian (changeAmount)
// berdasarkan uang tunai yang benar-benar diterima saat konfirmasi.
type ConfirmPaymentRequest struct {
	CashReceived *float64 `json:"cashReceived"`
}

// Confirm godoc
// @Summary      Konfirmasi pembayaran pending (admin)
// @Description  Admin mengkonfirmasi pembayaran tambahan (extend session) yang masih pending menjadi paid. Sertakan `cashReceived` agar kembalian dihitung dengan benar.
// @Tags         Pembayaran
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                 true  "Payment ID (UUID)"
// @Param        body  body      ConfirmPaymentRequest  false "Uang tunai yang diterima (opsional)"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.ErrorResponse
// @Router       /api/v1/payments/{id}/confirm [post]
func (h *PaymentHandler) Confirm(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	req := new(ConfirmPaymentRequest)
	// body opsional — abaikan error parsing kalau body kosong
	_ = c.BodyParser(req)

	type spResult struct {
		PaymentID     uuid.UUID  `gorm:"column:payment_id"`
		PaymentStatus string     `gorm:"column:payment_status"`
		PaidAt        *time.Time `gorm:"column:paid_at"`
		TotalPayment  float64    `gorm:"column:total_payment"`
		CashReceived  float64    `gorm:"column:cash_received"`
		ChangeAmount  float64    `gorm:"column:change_amount"`
	}

	var result spResult
	tx := h.db.WithContext(c.Context()).Raw(
		fmt.Sprintf(`SELECT * FROM %s(?, ?)`, spname.Ident("ConfirmExtendPayment")), id, req.CashReceived,
	).Scan(&result)

	if tx.Error != nil {
		return response.BadRequest(c, tx.Error.Error())
	}

	return response.OK(c, "Pembayaran berhasil dikonfirmasi", fiber.Map{
		"paymentId":    result.PaymentID,
		"status":       result.PaymentStatus,
		"paidAt":       result.PaidAt,
		"totalPayment": result.TotalPayment,
		"cashReceived": result.CashReceived,
		"changeAmount": result.ChangeAmount,
	})
}

// ConfirmSessionPendingRequest adalah body untuk melunasi semua pembayaran pending
// milik satu sesi sekaligus dengan satu nilai uang tunai.
type ConfirmSessionPendingRequest struct {
	CashReceived float64 `json:"cashReceived" validate:"required,gt=0"`
}

// ConfirmSessionPending godoc
// @Summary      Lunasi semua pembayaran pending sebuah sesi sekaligus (admin)
// @Description  Menggabungkan seluruh pembayaran pending (misal dari beberapa kali extend) menjadi satu transaksi tunai, lalu menghitung kembalian dari total gabungan.
// @Tags         Pembayaran
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        session_id  path      string                        true  "Session ID (UUID)"
// @Param        body        body      ConfirmSessionPendingRequest  true  "Uang tunai yang diterima"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.ErrorResponse
// @Router       /api/v1/sessions/{session_id}/payments/confirm-pending [post]
func (h *PaymentHandler) ConfirmSessionPending(c *fiber.Ctx) error {
	sessionID, err := uuid.Parse(c.Params("session_id"))
	if err != nil {
		return response.BadRequest(c, "Format Session ID tidak valid")
	}

	req := new(ConfirmSessionPendingRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	type spResult struct {
		SessionID      uuid.UUID `gorm:"column:session_id"`
		ConfirmedCount int       `gorm:"column:confirmed_count"`
		TotalPaid      float64   `gorm:"column:total_paid"`
		CashReceived   float64   `gorm:"column:cash_received"`
		ChangeAmount   float64   `gorm:"column:change_amount"`
	}

	var result spResult
	tx := h.db.WithContext(c.Context()).Raw(
		fmt.Sprintf(`SELECT * FROM %s(?, ?)`, spname.Ident("ConfirmSessionPendingPayments")), sessionID, req.CashReceived,
	).Scan(&result)

	if tx.Error != nil {
		return response.BadRequest(c, tx.Error.Error())
	}

	h.hub.Broadcast(websocket.NewEvent(websocket.EventPaymentCreated, fiber.Map{
		"sessionId": result.SessionID,
		"totalPaid": result.TotalPaid,
	}))

	return response.OK(c, "Semua pembayaran pending berhasil dilunasi", fiber.Map{
		"sessionId":      result.SessionID,
		"confirmedCount": result.ConfirmedCount,
		"totalPaid":      result.TotalPaid,
		"cashReceived":   result.CashReceived,
		"changeAmount":   result.ChangeAmount,
	})
}

// GetPendingExtensions godoc
// @Summary      List pembayaran extend yang belum dibayar (admin)
// @Description  Mengembalikan semua payment pending dari extend session. Frontend polling ini untuk menampilkan alert.
// @Tags         Pembayaran
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response
// @Router       /api/v1/payments/pending [get]
func (h *PaymentHandler) GetPendingExtensions(c *fiber.Ctx) error {
	var list []entity.Payment
	h.db.WithContext(c.Context()).
		Preload("Session").Preload("Session.Console").
		Where("payment_status = ?", entity.PaymentStatusPending).
		Order("created_at DESC").
		Find(&list)

	return response.OK(c, "Data pembayaran pending", fiber.Map{
		"pendingCount": len(list),
		"payments":     list,
	})
}

