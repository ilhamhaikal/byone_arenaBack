package handler

import (
	"byone-arena/internal/domain/repository"
	"byone-arena/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// DashboardHandler menangani HTTP request untuk dashboard & laporan
type DashboardHandler struct {
	paymentRepo repository.PaymentRepository
}

// NewDashboardHandler membuat instance baru DashboardHandler
func NewDashboardHandler(paymentRepo repository.PaymentRepository) *DashboardHandler {
	return &DashboardHandler{paymentRepo: paymentRepo}
}

// GetSummary godoc
// @Summary      Ringkasan dashboard
// @Description  Mengembalikan ringkasan pendapatan harian, penggunaan voucher, sesi aktif, dan status konsol.\n\n**Memerlukan autentikasi.**\n\nQuery param `date` opsional (default hari ini): `?date=2026-07-03`
// @Tags         Dashboard
// @Produce      json
// @Security     BearerAuth
// @Param        date  query     string  false  "Tanggal (YYYY-MM-DD), default hari ini"
// @Success      200   {object}  response.Response{data=entity.DashboardSummary}
// @Failure      401   {object}  response.ErrorResponse
// @Failure      500   {object}  response.ErrorResponse
// @Router       /api/v1/dashboard/summary [get]
func (h *DashboardHandler) GetSummary(c *fiber.Ctx) error {
	date := c.Query("date", "") // kosong = hari ini

	summary, err := h.paymentRepo.GetDashboardSummary(c.Context(), date)
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil ringkasan dashboard")
	}

	return response.OK(c, "Ringkasan dashboard berhasil diambil", summary)
}
