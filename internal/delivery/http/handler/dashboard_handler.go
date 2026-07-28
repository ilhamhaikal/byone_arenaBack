package handler

import (
	"byone-arena/internal/domain/repository"
	"byone-arena/pkg/response"

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
	h.db.Raw(`
		SELECT
			dd::DATE::TEXT AS date,
			COALESCE(sr.rev,0) + COALESCE(dr.rev,0) + COALESCE(mr.rev,0) + COALESCE(fr.rev,0) AS revenue,
			COALESCE(sr.sessions,0) + COALESCE(dr.rentals,0) + COALESCE(mr.memberships,0) + COALESCE(fr.orders,0) AS sessions
		FROM generate_series(
			CURRENT_DATE - (? - 1),
			CURRENT_DATE,
			'1 day'::INTERVAL
		) dd
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(p.total_payment),0) AS rev, COUNT(*)::INT AS sessions
			FROM payments p
			WHERE p.payment_status='paid' AND p.session_id IS NOT NULL
			AND p.paid_at::DATE = dd::DATE
		) sr ON TRUE
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(d.final_amount),0) AS rev, COUNT(*)::INT AS rentals
			FROM daily_rentals d
			WHERE d.created_at::DATE = dd::DATE
		) dr ON TRUE
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(p2.total_payment),0) AS rev, COUNT(*)::INT AS memberships
			FROM payments p2
			WHERE p2.payment_status='paid' AND p2.session_id IS NULL
			AND p2.paid_at::DATE = dd::DATE
		) mr ON TRUE
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(fo.total_amount),0) AS rev, COUNT(*)::INT AS orders
			FROM food_orders fo
			WHERE fo.status='served' AND fo.updated_at::DATE = dd::DATE
		) fr ON TRUE
		ORDER BY dd::DATE
	`, days).Scan(&items)

	if len(items) == 0 {
		items = []RevenueChartItem{}
	}

	return response.OK(c, "Data grafik pendapatan", items)
}
