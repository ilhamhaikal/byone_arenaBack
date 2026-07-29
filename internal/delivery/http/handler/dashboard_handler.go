package handler

import (
	"byone-arena/internal/domain/repository"
	"byone-arena/pkg/response"
	"byone-arena/pkg/spname"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// DashboardHandler menangani HTTP request untuk dashboard & laporan
type DashboardHandler struct {
	paymentRepo repository.PaymentRepository
	db          *gorm.DB
}

// NewDashboardHandler membuat instance baru DashboardHandler
func NewDashboardHandler(paymentRepo repository.PaymentRepository, db *gorm.DB) *DashboardHandler {
	return &DashboardHandler{paymentRepo: paymentRepo, db: db}
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

// RevenueChartItem satu titik data grafik pendapatan
type RevenueChartItem struct {
	Date     string  `json:"date"`
	Revenue  float64 `json:"revenue"`
	Sessions int     `json:"sessions"`
}

// GetRevenueChart godoc
// @Summary      Grafik pendapatan harian
// @Description  Mengembalikan data pendapatan per hari untuk N hari terakhir (default 7). Mencakup sesi rental + sewa harian + membership + makanan.
// @Tags         Dashboard
// @Produce      json
// @Security     BearerAuth
// @Param        days  query     int  false  "Jumlah hari (default 7, max 30)"
// @Success      200   {object}  response.Response{data=[]handler.RevenueChartItem}
// @Router       /api/v1/dashboard/revenue-chart [get]
func (h *DashboardHandler) GetRevenueChart(c *fiber.Ctx) error {
	days := c.QueryInt("days", 7)
	if days < 1 {
		days = 7
	}
	if days > 30 {
		days = 30
	}

	var items []RevenueChartItem
	spName := spname.Ident("RevenueChart")
	if err := h.db.Raw("SELECT * FROM "+spName+"(?)", days).Scan(&items).Error; err != nil {
		return response.InternalServerError(c, "Gagal mengambil data grafik")
	}

	if len(items) == 0 {
		items = []RevenueChartItem{}
	}

	return response.OK(c, "Data grafik pendapatan", items)
}
