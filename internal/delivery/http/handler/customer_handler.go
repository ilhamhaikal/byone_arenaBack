package handler

import (
	"fmt"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"
	"byone-arena/pkg/spname"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CustomerHandler menangani HTTP request untuk manajemen pelanggan
type CustomerHandler struct {
	customerUC usecase.CustomerUseCase
	validator  *validator.Validator
	db         *gorm.DB
}

// NewCustomerHandler membuat instance baru CustomerHandler
func NewCustomerHandler(customerUC usecase.CustomerUseCase, v *validator.Validator, db *gorm.DB) *CustomerHandler {
	return &CustomerHandler{customerUC: customerUC, validator: v, db: db}
}

// GetAll godoc
// @Summary      Ambil semua pelanggan
// @Description  Mengembalikan daftar seluruh pelanggan yang terdaftar, diurutkan berdasarkan nama
// @Tags         Pelanggan
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]entity.Customer}
// @Failure      401  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /api/v1/customers [get]
func (h *CustomerHandler) GetAll(c *fiber.Ctx) error {
	customers, err := h.customerUC.GetAllCustomers(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil data pelanggan")
	}
	return response.OK(c, "Data pelanggan berhasil diambil", customers)
}

// GetByID godoc
// @Summary      Ambil pelanggan berdasarkan ID
// @Description  Mengembalikan detail data satu pelanggan
// @Tags         Pelanggan
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Customer ID (UUID)"
// @Success      200  {object}  response.Response{data=entity.Customer}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/customers/{id} [get]
func (h *CustomerHandler) GetByID(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	customer, err := h.customerUC.GetCustomerByID(c.Context(), id)
	if err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.OK(c, "Data pelanggan berhasil diambil", customer)
}

// Create godoc
// @Summary      Daftarkan pelanggan baru
// @Description  Mendaftarkan pelanggan baru ke sistem. Nomor telepon harus unik.
// @Tags         Pelanggan
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      usecase.CreateCustomerRequest  true  "Data pelanggan baru"
// @Success      201   {object}  response.Response{data=entity.Customer}
// @Failure      400   {object}  response.ErrorResponse
// @Failure      401   {object}  response.ErrorResponse
// @Failure      409   {object}  response.ErrorResponse  "Nomor telepon sudah terdaftar"
// @Router       /api/v1/customers [post]
func (h *CustomerHandler) Create(c *fiber.Ctx) error {
	req := new(usecase.CreateCustomerRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	customer, err := h.customerUC.CreateCustomer(c.Context(), req)
	if err != nil {
		return response.Conflict(c, err.Error())
	}

	// Auto-sell membership jika is_member=true
	var membershipInfo fiber.Map
	if req.IsMember {
		var row struct {
			Value string `gorm:"column:value"`
		}
		h.db.WithContext(c.Context()).Raw(
			"SELECT * FROM "+spname.Ident("AppSettingValue")+"(?::text)", "membership_price",
		).Scan(&row)

		var sp struct {
			PaymentID    uuid.UUID `gorm:"column:payment_id"`
			ChangeAmount float64   `gorm:"column:change_amount"`
			Message      string    `gorm:"column:message"`
		}
		cashValue := row.Value
		if isCashless(req.PaymentMethod) {
			cashValue = fmt.Sprintf("%.0f", paymentMethodSentinel)
		}
		// Gunakan fmt.Sprintf untuk inject numeric value langsung (hindari GORM encode issue)
		err = h.db.WithContext(c.Context()).Transaction(func(tx *gorm.DB) error {
			if err := tx.Raw(
				fmt.Sprintf(`SELECT * FROM %s('%s'::UUID, %s::NUMERIC)`, spname.Ident("SellMembership"),
					customer.ID, cashValue),
			).Scan(&sp).Error; err != nil {
				return err
			}
			return finalizePaymentMethod(tx, sp.PaymentID, req.PaymentMethod)
		})
		if err != nil {
			return response.InternalServerError(c,
				"Pelanggan tersimpan tetapi penjualan membership gagal: "+err.Error())
		}
		membershipInfo = fiber.Map{
			"paymentId":    sp.PaymentID,
			"changeAmount": sp.ChangeAmount,
			"message":      sp.Message,
		}
		// Ambil ulang customer agar response sudah berisi isMember=true dan
		// harga membership yang benar-benar tersimpan.
		if updated, err := h.customerUC.GetCustomerByID(c.Context(), customer.ID); err == nil {
			customer = updated
		}
	}

	return response.Created(c, "Pelanggan berhasil didaftarkan", fiber.Map{
		"customer":   customer,
		"membership": membershipInfo,
	})
}

// Update godoc
// @Summary      Update data pelanggan
// @Description  Memperbarui data pelanggan yang sudah terdaftar
// @Tags         Pelanggan
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                         true  "Customer ID (UUID)"
// @Param        body  body      usecase.UpdateCustomerRequest  true  "Data pelanggan yang diperbarui"
// @Success      200   {object}  response.Response{data=entity.Customer}
// @Failure      400   {object}  response.ErrorResponse
// @Failure      401   {object}  response.ErrorResponse
// @Failure      404   {object}  response.ErrorResponse
// @Router       /api/v1/customers/{id} [put]
func (h *CustomerHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	req := new(usecase.UpdateCustomerRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	customer, err := h.customerUC.UpdateCustomer(c.Context(), id, req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Data pelanggan berhasil diperbarui", customer)
}

// Delete godoc
// @Summary      Hapus data pelanggan
// @Description  Menghapus data pelanggan dari sistem secara permanen
// @Tags         Pelanggan
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Customer ID (UUID)"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Router       /api/v1/customers/{id} [delete]
func (h *CustomerHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	if err := h.customerUC.DeleteCustomer(c.Context(), id); err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Data pelanggan berhasil dihapus", nil)
}

// SellMembership godoc
// @Summary      Jual membership ke customer
// @Description  Mengaktifkan membership LIFETIME. **Harga & uang otomatis dari pengaturan** — tidak perlu input apa pun. Body kosong atau isi `cashReceived` jika ada kembalian.
// @Tags         Pelanggan
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string  true  "Customer ID"
// @Param        body  body  object  false  "Opsional: {\"cashReceived\": 50000} jika uang pas/lebih"
// @Success      200   {object}  response.Response
// @Failure      400   {object}  response.ErrorResponse
// @Router       /api/v1/customers/{id}/membership [post]
func (h *CustomerHandler) SellMembership(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	// CashReceived opsional — default ke harga dari settings
	var req struct {
		CashReceived  *float64 `json:"cashReceived"`
		PaymentMethod string   `json:"paymentMethod" validate:"omitempty,oneof=cash qris transfer"`
	}
	c.BodyParser(&req)

	var cashStr string
	if req.CashReceived != nil {
		cashStr = fmt.Sprintf("%.0f", *req.CashReceived)
	} else {
		var row struct {
			Value string `gorm:"column:value"`
		}
		h.db.WithContext(c.Context()).Raw(
			"SELECT * FROM "+spname.Ident("AppSettingValue")+"(?::text)", "membership_price",
		).Scan(&row)
		cashStr = row.Value
		if cashStr == "" || cashStr == "0" {
			cashStr = "0"
		}
	}
	if isCashless(req.PaymentMethod) {
		cashStr = fmt.Sprintf("%.0f", paymentMethodSentinel)
	}

	type spResult struct {
		CustomerID      uuid.UUID `gorm:"column:customer_id"`
		MembershipPrice float64   `gorm:"column:membership_price"`
		PaymentID       uuid.UUID `gorm:"column:payment_id"`
		ChangeAmount    float64   `gorm:"column:change_amount"`
		Message         string    `gorm:"column:message"`
	}

	var result spResult
	err = h.db.WithContext(c.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(
			fmt.Sprintf(`SELECT * FROM %s('%s'::UUID, %s::NUMERIC)`, spname.Ident("SellMembership"),
				id, cashStr),
		).Scan(&result).Error; err != nil {
			return err
		}
		return finalizePaymentMethod(tx, result.PaymentID, req.PaymentMethod)
	})
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	// Sertakan data customer terbaru supaya frontend tidak perlu menebak dari
	// response pembayaran (yang tidak berisi id/nama/status member).
	payload := fiber.Map{
		"membershipPrice": result.MembershipPrice,
		"paymentId":       result.PaymentID,
		"changeAmount":    result.ChangeAmount,
	}
	if customer, err := h.customerUC.GetCustomerByID(c.Context(), id); err == nil {
		payload["customer"] = customer
	}

	return response.OK(c, result.Message, payload)
}

// GetLoyalty godoc
// @Summary      Ambil info bank waktu & poin loyalitas member
// @Description  Mengembalikan saldo bank waktu (menit) & poin loyalitas member saat ini, pengaturan penukaran poin yang aktif, dan riwayat mutasi (log) terbaru. Hanya relevan untuk customer yang isMember=true.
// @Tags         Pelanggan
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Customer ID (UUID)"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/customers/{id}/loyalty [get]
func (h *CustomerHandler) GetLoyalty(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	customer, err := h.customerUC.GetCustomerByID(c.Context(), id)
	if err != nil {
		return response.NotFound(c, err.Error())
	}

	pointThreshold := 15
	pointRedeemMinutes := 60
	var thrRow, redeemRow struct {
		Value string `gorm:"column:value"`
	}
	h.db.WithContext(c.Context()).Raw(
		"SELECT * FROM "+spname.Ident("AppSettingValue")+"(?::text)", "loyalty_point_threshold",
	).Scan(&thrRow)
	if thrRow.Value != "" {
		fmt.Sscanf(thrRow.Value, "%d", &pointThreshold)
	}
	h.db.WithContext(c.Context()).Raw(
		"SELECT * FROM "+spname.Ident("AppSettingValue")+"(?::text)", "loyalty_point_redeem_minutes",
	).Scan(&redeemRow)
	if redeemRow.Value != "" {
		fmt.Sscanf(redeemRow.Value, "%d", &pointRedeemMinutes)
	}

	var timeBankLogs []entity.CustomerTimeBankLog
	h.db.WithContext(c.Context()).
		Where("customer_id = ?", id).
		Order("created_at DESC").
		Limit(20).
		Find(&timeBankLogs)

	var pointLogs []entity.CustomerPointLog
	h.db.WithContext(c.Context()).
		Where("customer_id = ?", id).
		Order("created_at DESC").
		Limit(20).
		Find(&pointLogs)

	maxRedeemUnits := 0
	if pointThreshold > 0 {
		maxRedeemUnits = customer.LoyaltyPoints / pointThreshold
	}

	return response.OK(c, "Data bank waktu & poin loyalitas berhasil diambil", fiber.Map{
		"timeBalanceMinutes": customer.TimeBalanceMinutes,
		"loyaltyPoints":      customer.LoyaltyPoints,
		"pointThreshold":     pointThreshold,
		"pointRedeemMinutes": pointRedeemMinutes,
		"maxRedeemUnits":     maxRedeemUnits,
		"timeBankLogs":       timeBankLogs,
		"pointLogs":          pointLogs,
	})
}
