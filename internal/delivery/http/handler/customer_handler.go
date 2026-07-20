package handler

import (
	"fmt"

	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"
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
		var row struct{ Value string `gorm:"column:value"` }
		h.db.WithContext(c.Context()).Raw(
			"SELECT value FROM app_settings WHERE key = 'membership_price'",
		).Scan(&row)

		var sp struct {
			PaymentID    uuid.UUID `gorm:"column:payment_id"`
			ChangeAmount float64   `gorm:"column:change_amount"`
			Message      string    `gorm:"column:message"`
		}
		// Gunakan fmt.Sprintf untuk inject numeric value langsung (hindari GORM encode issue)
		h.db.WithContext(c.Context()).Raw(
			fmt.Sprintf(`SELECT * FROM "byoneSellMembership"('%s'::UUID, %s::NUMERIC)`,
				customer.ID, row.Value),
		).Scan(&sp)
		membershipInfo = fiber.Map{
			"paymentId":    sp.PaymentID,
			"changeAmount": sp.ChangeAmount,
			"message":      sp.Message,
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
		CashReceived *float64 `json:"cashReceived"`
	}
	c.BodyParser(&req)

	var cashStr string
	if req.CashReceived != nil {
		cashStr = fmt.Sprintf("%.0f", *req.CashReceived)
	} else {
		var row struct{ Value string `gorm:"column:value"` }
		h.db.WithContext(c.Context()).Raw(
			"SELECT value FROM app_settings WHERE key = 'membership_price'",
		).Scan(&row)
		cashStr = row.Value
		if cashStr == "" || cashStr == "0" {
			cashStr = "0"
		}
	}

	type spResult struct {
		CustomerID      uuid.UUID `gorm:"column:customer_id"`
		MembershipPrice float64   `gorm:"column:membership_price"`
		PaymentID       uuid.UUID `gorm:"column:payment_id"`
		ChangeAmount    float64   `gorm:"column:change_amount"`
		Message         string    `gorm:"column:message"`
	}

	var result spResult
	tx := h.db.WithContext(c.Context()).Raw(
		fmt.Sprintf(`SELECT * FROM "byoneSellMembership"('%s'::UUID, %s::NUMERIC)`,
			id, cashStr),
	).Scan(&result)
	if tx.Error != nil {
		return response.BadRequest(c, tx.Error.Error())
	}

	return response.OK(c, result.Message, fiber.Map{
		"membershipPrice": result.MembershipPrice,
		"paymentId":       result.PaymentID,
		"changeAmount":    result.ChangeAmount,
	})
}

