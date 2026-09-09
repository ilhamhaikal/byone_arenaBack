package handler

import (
	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// CapitalHandler menangani HTTP request untuk pembukuan modal awal.
type CapitalHandler struct {
	capitalUC usecase.CapitalUseCase
	validator *validator.Validator
}

// NewCapitalHandler membuat instance baru CapitalHandler
func NewCapitalHandler(uc usecase.CapitalUseCase, v *validator.Validator) *CapitalHandler {
	return &CapitalHandler{capitalUC: uc, validator: v}
}

// AddEntry godoc
// @Summary      Tambah catatan modal awal
// @Tags         Laporan
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  usecase.AddCapitalEntryRequest  true  "Jumlah + tanggal + catatan"
// @Success      201  {object}  response.Response{data=entity.CapitalEntry}
// @Router       /api/v1/capital [post]
func (h *CapitalHandler) AddEntry(c *fiber.Ctx) error {
	req := new(usecase.AddCapitalEntryRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}
	entry, err := h.capitalUC.AddEntry(c.Context(), req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "Modal berhasil dicatat", entry)
}

// DeleteEntry godoc
// @Summary      Hapus catatan modal
// @Tags         Laporan
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Capital Entry ID (UUID)"
// @Success      200  {object}  response.Response
// @Router       /api/v1/capital/{id} [delete]
func (h *CapitalHandler) DeleteEntry(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	if err := h.capitalUC.DeleteEntry(c.Context(), id); err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Catatan modal dihapus", nil)
}

// ListEntries godoc
// @Summary      Riwayat modal awal (rentang tanggal)
// @Tags         Laporan
// @Produce      json
// @Security     BearerAuth
// @Param        startDate  query  string  false  "YYYY-MM-DD"
// @Param        endDate    query  string  false  "YYYY-MM-DD"
// @Success      200  {object}  response.Response{data=[]entity.CapitalEntry}
// @Router       /api/v1/capital [get]
func (h *CapitalHandler) ListEntries(c *fiber.Ctx) error {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	rows, err := h.capitalUC.ListEntries(c.Context(), startDate, endDate)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Riwayat modal berhasil diambil", rows)
}

// MonthlyReport godoc
// @Summary      Perbandingan modal vs pendapatan per bulan
// @Tags         Laporan
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]entity.MonthlyCapital}
// @Router       /api/v1/capital/monthly [get]
func (h *CapitalHandler) MonthlyReport(c *fiber.Ctx) error {
	rows, err := h.capitalUC.MonthlyReport(c.Context(), 6)
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil laporan bulanan")
	}
	return response.OK(c, "Laporan bulanan berhasil diambil", rows)
}
