package handler

import (
	"byone-arena/internal/domain/repository"
	"byone-arena/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// ReportHandler menangani HTTP request untuk laporan
type ReportHandler struct {
	paymentRepo repository.PaymentRepository
}

// NewReportHandler membuat instance baru ReportHandler
func NewReportHandler(paymentRepo repository.PaymentRepository) *ReportHandler {
	return &ReportHandler{paymentRepo: paymentRepo}
}

// GetSummary godoc
// @Summary      Laporan komprehensif (rentang tanggal)
// @Description  Mengembalikan laporan lengkap: pendapatan, penggunaan voucher, total jam main, rincian per konsol, rincian per hari.\n\n**Memerlukan autentikasi.**\n\nQuery param:\n- `startDate` (YYYY-MM-DD, default 7 hari lalu)\n- `endDate` (YYYY-MM-DD, default hari ini)
// @Tags         Laporan
// @Produce      json
// @Security     BearerAuth
// @Param        startDate  query     string  false  "Tanggal mulai (YYYY-MM-DD)"
// @Param        endDate    query     string  false  "Tanggal akhir (YYYY-MM-DD)"
// @Success      200  {object}  response.Response{data=entity.ReportSummary}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Router       /api/v1/reports/summary [get]
func (h *ReportHandler) GetSummary(c *fiber.Ctx) error {
	startDate := c.Query("startDate", "")
	endDate := c.Query("endDate", "")

	report, err := h.paymentRepo.GetReportSummary(c.Context(), startDate, endDate)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	return response.OK(c, "Laporan berhasil diambil", report)
}
