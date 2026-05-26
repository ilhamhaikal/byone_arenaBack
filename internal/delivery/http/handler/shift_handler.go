package handler

import (
	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// ShiftHandler menangani endpoint manajemen shift kasir
type ShiftHandler struct {
	shiftUC  usecase.ShiftUseCase
	validate *validator.Validate
}

// NewShiftHandler membuat instance baru ShiftHandler
func NewShiftHandler(shiftUC usecase.ShiftUseCase) *ShiftHandler {
	return &ShiftHandler{
		shiftUC:  shiftUC,
		validate: validator.New(),
	}
}

// GetAll godoc
// @Summary      Ambil semua jadwal shift
// @Description  Mengembalikan semua jadwal shift kasir beserta data pengguna terkait
// @Tags         Shift Kasir
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]entity.Shift}
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse  "Hanya admin dan superadmin"
// @Failure      500  {object}  response.ErrorResponse
// @Router       /api/v1/shifts [get]
func (h *ShiftHandler) GetAll(c *fiber.Ctx) error {
	shifts, err := h.shiftUC.GetAllShifts(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil data shift")
	}
	return response.OK(c, "Data shift berhasil diambil", shifts)
}

// GetByUser godoc
// @Summary      Ambil shift berdasarkan User ID
// @Description  Mengembalikan semua jadwal shift milik satu pengguna kasir
// @Tags         Shift Kasir
// @Produce      json
// @Security     BearerAuth
// @Param        user_id  path      string  true  "User ID (UUID)"
// @Success      200      {object}  response.Response{data=[]entity.Shift}
// @Failure      400      {object}  response.ErrorResponse
// @Failure      401      {object}  response.ErrorResponse
// @Failure      403      {object}  response.ErrorResponse
// @Failure      500      {object}  response.ErrorResponse
// @Router       /api/v1/users/{user_id}/shifts [get]
func (h *ShiftHandler) GetByUser(c *fiber.Ctx) error {
	userID, err := uuid.Parse(c.Params("user_id"))
	if err != nil {
		return response.BadRequest(c, "User ID tidak valid")
	}
	shifts, err := h.shiftUC.GetShiftsByUser(c.Context(), userID)
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil data shift")
	}
	return response.OK(c, "Data shift berhasil diambil", shifts)
}

// GetByID godoc
// @Summary      Ambil shift berdasarkan ID
// @Description  Mengembalikan detail satu jadwal shift kasir
// @Tags         Shift Kasir
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Shift ID (UUID)"
// @Success      200  {object}  response.Response{data=entity.Shift}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/shifts/{id} [get]
func (h *ShiftHandler) GetByID(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Shift ID tidak valid")
	}
	shift, err := h.shiftUC.GetShiftByID(c.Context(), id)
	if err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.OK(c, "Data shift berhasil diambil", shift)
}

// Create godoc
// @Summary      Buat jadwal shift baru
// @Description  Membuat jadwal shift untuk pengguna kasir. Gunakan is24Hour=true untuk shift tanpa batasan jam. Untuk shift partial, startHour dan endHour tidak boleh sama (mendukung overnight, misal: startHour=22, endHour=6).
// @Tags         Shift Kasir
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      usecase.CreateShiftRequest  true  "Data shift baru"
// @Success      201   {object}  response.Response{data=entity.Shift}
// @Failure      400   {object}  response.ErrorResponse  "User bukan kasir atau jam tidak valid"
// @Failure      401   {object}  response.ErrorResponse
// @Failure      403   {object}  response.ErrorResponse
// @Router       /api/v1/shifts [post]
func (h *ShiftHandler) Create(c *fiber.Ctx) error {
	req := new(usecase.CreateShiftRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validate.Struct(req); err != nil {
		return response.BadRequest(c, err.Error())
	}

	shift, err := h.shiftUC.CreateShift(c.Context(), req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "Shift berhasil dibuat", shift)
}

// Update godoc
// @Summary      Update jadwal shift
// @Description  Memperbarui data jadwal shift. Semua field bersifat opsional.
// @Tags         Shift Kasir
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                      true  "Shift ID (UUID)"
// @Param        body  body      usecase.UpdateShiftRequest  true  "Data shift yang diperbarui"
// @Success      200   {object}  response.Response{data=entity.Shift}
// @Failure      400   {object}  response.ErrorResponse
// @Failure      401   {object}  response.ErrorResponse
// @Failure      403   {object}  response.ErrorResponse
// @Failure      404   {object}  response.ErrorResponse
// @Router       /api/v1/shifts/{id} [put]
func (h *ShiftHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Shift ID tidak valid")
	}

	req := new(usecase.UpdateShiftRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validate.Struct(req); err != nil {
		return response.BadRequest(c, err.Error())
	}

	shift, err := h.shiftUC.UpdateShift(c.Context(), id, req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Shift berhasil diperbarui", shift)
}

// Delete godoc
// @Summary      Hapus jadwal shift
// @Description  Menghapus jadwal shift kasir secara permanen
// @Tags         Shift Kasir
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Shift ID (UUID)"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/shifts/{id} [delete]
func (h *ShiftHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Shift ID tidak valid")
	}
	if err := h.shiftUC.DeleteShift(c.Context(), id); err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Shift berhasil dihapus", nil)
}
