package handler

import (
	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// DiscountHandler menangani HTTP request untuk manajemen aturan diskon otomatis
type DiscountHandler struct {
	discountUC usecase.DiscountRuleUseCase
	validator  *validator.Validator
}

// NewDiscountHandler membuat instance baru DiscountHandler
func NewDiscountHandler(discountUC usecase.DiscountRuleUseCase, v *validator.Validator) *DiscountHandler {
	return &DiscountHandler{discountUC: discountUC, validator: v}
}

// GetAll godoc
// @Summary      Ambil semua aturan diskon
// @Description  Mengembalikan seluruh aturan diskon otomatis (happy hour, member, dll) beserta statusnya
// @Tags         Discount
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]entity.DiscountRule}
// @Failure      401  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /api/v1/discounts [get]
func (h *DiscountHandler) GetAll(c *fiber.Ctx) error {
	rules, err := h.discountUC.GetAllRules(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil data aturan diskon")
	}
	return response.OK(c, "Data aturan diskon berhasil diambil", rules)
}

// GetActive godoc
// @Summary      Ambil aturan diskon aktif
// @Description  Mengembalikan aturan diskon yang sedang aktif, diurutkan berdasarkan prioritas
// @Tags         Discount
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]entity.DiscountRule}
// @Failure      401  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /api/v1/discounts/active [get]
func (h *DiscountHandler) GetActive(c *fiber.Ctx) error {
	rules, err := h.discountUC.GetActiveRules(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil aturan diskon aktif")
	}
	return response.OK(c, "Aturan diskon aktif berhasil diambil", rules)
}

// GetByID godoc
// @Summary      Ambil aturan diskon berdasarkan ID
// @Description  Mengembalikan detail satu aturan diskon
// @Tags         Discount
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Discount Rule ID (UUID)"
// @Success      200  {object}  response.Response{data=entity.DiscountRule}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/discounts/{id} [get]
func (h *DiscountHandler) GetByID(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	rule, err := h.discountUC.GetRuleByID(c.Context(), id)
	if err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.OK(c, "Data aturan diskon berhasil diambil", rule)
}

// Create godoc
// @Summary      Buat aturan diskon baru
// @Description  Membuat aturan diskon otomatis baru. Tipe rule: 'always', 'happy_hour', 'member', 'day_of_week'.
// @Tags         Discount
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      usecase.CreateDiscountRuleRequest  true  "Data aturan diskon baru"
// @Success      201   {object}  response.Response{data=entity.DiscountRule}
// @Failure      400   {object}  response.ErrorResponse
// @Failure      401   {object}  response.ErrorResponse
// @Failure      403   {object}  response.ErrorResponse
// @Router       /api/v1/discounts [post]
func (h *DiscountHandler) Create(c *fiber.Ctx) error {
	req := new(usecase.CreateDiscountRuleRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	rule, err := h.discountUC.CreateRule(c.Context(), req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "Aturan diskon berhasil dibuat", rule)
}

// Update godoc
// @Summary      Update aturan diskon
// @Description  Mengubah data aturan diskon. Semua field opsional (partial update).
// @Tags         Discount
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                             true  "Discount Rule ID (UUID)"
// @Param        body  body      usecase.UpdateDiscountRuleRequest  true  "Data yang diubah"
// @Success      200   {object}  response.Response{data=entity.DiscountRule}
// @Failure      400   {object}  response.ErrorResponse
// @Failure      401   {object}  response.ErrorResponse
// @Failure      403   {object}  response.ErrorResponse
// @Failure      404   {object}  response.ErrorResponse
// @Router       /api/v1/discounts/{id} [put]
func (h *DiscountHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	req := new(usecase.UpdateDiscountRuleRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	rule, err := h.discountUC.UpdateRule(c.Context(), id, req)
	if err != nil {
		if err.Error() == "aturan diskon tidak ditemukan" {
			return response.NotFound(c, err.Error())
		}
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Aturan diskon berhasil diperbarui", rule)
}

// Delete godoc
// @Summary      Hapus aturan diskon
// @Description  Menghapus aturan diskon secara permanen
// @Tags         Discount
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Discount Rule ID (UUID)"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/discounts/{id} [delete]
func (h *DiscountHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	if err := h.discountUC.DeleteRule(c.Context(), id); err != nil {
		if err.Error() == "aturan diskon tidak ditemukan" {
			return response.NotFound(c, err.Error())
		}
		return response.InternalServerError(c, "Gagal menghapus aturan diskon")
	}
	return response.OK(c, "Aturan diskon berhasil dihapus", nil)
}

// Toggle godoc
// @Summary      Aktifkan / nonaktifkan aturan diskon
// @Description  Toggle status aktif aturan diskon (aktif ↔ nonaktif)
// @Tags         Discount
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Discount Rule ID (UUID)"
// @Success      200  {object}  response.Response{data=entity.DiscountRule}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/discounts/{id}/toggle [patch]
func (h *DiscountHandler) Toggle(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	rule, err := h.discountUC.ToggleRule(c.Context(), id)
	if err != nil {
		if err.Error() == "aturan diskon tidak ditemukan" {
			return response.NotFound(c, err.Error())
		}
		return response.InternalServerError(c, "Gagal mengubah status aturan diskon")
	}
	status := "dinonaktifkan"
	if rule.IsActive {
		status = "diaktifkan"
	}
	return response.OK(c, "Aturan diskon berhasil "+status, rule)
}
