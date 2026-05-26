package handler

import (
	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// ConsoleHandler menangani HTTP request untuk manajemen konsol
type ConsoleHandler struct {
	consoleUC usecase.ConsoleUseCase
	validator *validator.Validator
}

// NewConsoleHandler membuat instance baru ConsoleHandler
func NewConsoleHandler(consoleUC usecase.ConsoleUseCase, v *validator.Validator) *ConsoleHandler {
	return &ConsoleHandler{consoleUC: consoleUC, validator: v}
}

// GetAll godoc
// @Summary      Ambil semua konsol
// @Description  Mengembalikan daftar seluruh unit konsol / TV Android
// @Tags         Konsol
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]entity.Console}
// @Failure      401  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /api/v1/consoles [get]
func (h *ConsoleHandler) GetAll(c *fiber.Ctx) error {
	consoles, err := h.consoleUC.GetAllConsoles(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil data konsol")
	}
	return response.OK(c, "Data konsol berhasil diambil", consoles)
}

// GetAvailable godoc
// @Summary      Ambil konsol yang tersedia
// @Description  Mengembalikan daftar konsol dengan status available
// @Tags         Konsol
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]entity.Console}
// @Failure      401  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /api/v1/consoles/available [get]
func (h *ConsoleHandler) GetAvailable(c *fiber.Ctx) error {
	consoles, err := h.consoleUC.GetAvailableConsoles(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil data konsol tersedia")
	}
	return response.OK(c, "Konsol tersedia berhasil diambil", consoles)
}

// GetByID godoc
// @Summary      Ambil konsol berdasarkan ID
// @Description  Mengembalikan detail satu unit konsol
// @Tags         Konsol
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Console ID (UUID)"
// @Success      200  {object}  response.Response{data=entity.Console}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/consoles/{id} [get]
func (h *ConsoleHandler) GetByID(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	console, err := h.consoleUC.GetConsoleByID(c.Context(), id)
	if err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.OK(c, "Data konsol berhasil diambil", console)
}

// Create godoc
// @Summary      Tambah konsol baru
// @Description  Menambahkan unit konsol PlayStation baru ke sistem
// @Tags         Konsol
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      usecase.CreateConsoleRequest  true  "Data konsol baru"
// @Success      201   {object}  response.Response{data=entity.Console}
// @Failure      400   {object}  response.ErrorResponse
// @Failure      401   {object}  response.ErrorResponse
// @Failure      500   {object}  response.ErrorResponse
// @Router       /api/v1/consoles [post]
func (h *ConsoleHandler) Create(c *fiber.Ctx) error {
	req := new(usecase.CreateConsoleRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	console, err := h.consoleUC.CreateConsole(c.Context(), req)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Created(c, "Konsol berhasil ditambahkan", console)
}

// Update godoc
// @Summary      Update data konsol
// @Description  Memperbarui informasi unit konsol (nama, tipe, harga, status, deskripsi)
// @Tags         Konsol
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                        true  "Console ID (UUID)"
// @Param        body  body      usecase.UpdateConsoleRequest  true  "Data konsol yang diperbarui"
// @Success      200   {object}  response.Response{data=entity.Console}
// @Failure      400   {object}  response.ErrorResponse
// @Failure      401   {object}  response.ErrorResponse
// @Failure      404   {object}  response.ErrorResponse
// @Router       /api/v1/consoles/{id} [put]
func (h *ConsoleHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	req := new(usecase.UpdateConsoleRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	console, err := h.consoleUC.UpdateConsole(c.Context(), id, req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Data konsol berhasil diperbarui", console)
}

// Delete godoc
// @Summary      Hapus konsol
// @Description  Menghapus unit konsol dari sistem secara permanen
// @Tags         Konsol
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Console ID (UUID)"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/consoles/{id} [delete]
func (h *ConsoleHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	if err := h.consoleUC.DeleteConsole(c.Context(), id); err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Konsol berhasil dihapus", nil)
}

// GetOverview godoc
// @Summary      Dashboard overview semua konsol
// @Description  Mengembalikan semua konsol beserta sesi aktif masing-masing (jika ada).\n\nSetiap item mencakup:\n- Data konsol (nama, tipe, IP, status, harga/jam)\n- `activeSession`: null jika konsol kosong, atau berisi info sesi aktif termasuk `remainingMinutes` (sisa menit dari durasi yang dipesan; -1 = open-ended).\n\nGunakan endpoint ini untuk tampilan dashboard / monitor konsol secara realtime.
// @Tags         Konsol
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]usecase.ConsoleOverviewItem}
// @Failure      401  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /api/v1/consoles/overview [get]
func (h *ConsoleHandler) GetOverview(c *fiber.Ctx) error {
	items, err := h.consoleUC.GetConsoleOverview(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil overview konsol")
	}
	return response.OK(c, "Overview konsol berhasil diambil", items)
}
