package handler

import (
	"fmt"
	"sort"

	"byone-arena/pkg/response"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ActivityHandler struct {
	db *gorm.DB
}

func NewActivityHandler(db *gorm.DB) *ActivityHandler {
	return &ActivityHandler{db: db}
}

type ActivityItem struct {
	Type      string `json:"type"`
	Action    string `json:"action"`
	Title     string `json:"title"`
	Detail    string `json:"detail"`
	Timestamp string `json:"timestamp"`
}

// GetRecentActivities godoc
// @Summary      Aktivitas terbaru (realtime)
// @Description  Mengembalikan aktivitas terbaru: perubahan konsol, harga, sesi, rental, membership, pembayaran. `?limit=10` (default 10, max 50).
// @Tags         Aktivitas
// @Produce      json
// @Security     BearerAuth
// @Param        limit  query     int  false  "Jumlah data (default 10, max 50)"
// @Success      200    {object}  response.Response{data=[]handler.ActivityItem}
// @Router       /api/v1/activities/recent [get]
func (h *ActivityHandler) GetRecentActivities(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 10)
	if limit < 1 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	var activities []ActivityItem

	// 1. Konsol
	type cr struct {
		Name         string  `gorm:"column:name"`
		PricePerHour float64 `gorm:"column:price_per_hour"`
		DailyPrice   float64 `gorm:"column:daily_price"`
		Status       string  `gorm:"column:status"`
		UpdatedAt    string  `gorm:"column:updated_at"`
	}
	var consoles []cr
	h.db.Raw(`SELECT name, price_per_hour, daily_price, status,
		to_char(updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as updated_at
		FROM consoles ORDER BY updated_at DESC LIMIT ?`, limit).Scan(&consoles)
	for _, c := range consoles {
		activities = append(activities, ActivityItem{
			Type: "console", Action: "updated", Title: c.Name,
			Detail:    fmt.Sprintf("Status: %s | Rp %.0f/jam | Rp %.0f/hari", c.Status, c.PricePerHour, c.DailyPrice),
			Timestamp: c.UpdatedAt,
		})
	}

	// 2. Settings
	type sr struct {
		Key       string `gorm:"column:key"`
		Value     string `gorm:"column:value"`
		UpdatedAt string `gorm:"column:updated_at"`
	}
	var settings []sr
	h.db.Raw(`SELECT key, value,
		to_char(updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as updated_at
		FROM app_settings ORDER BY updated_at DESC LIMIT ?`, limit).Scan(&settings)
	for _, s := range settings {
		label := s.Key
		if s.Key == "membership_price" {
			label = "Harga Membership"
		} else if s.Key == "daily_price" {
			label = "Harga Sewa Harian"
		}
		activities = append(activities, ActivityItem{
			Type: "setting", Action: "updated", Title: label,
			Detail:    "Rp " + s.Value,
			Timestamp: s.UpdatedAt,
		})
	}

	// 3. Sesi
	type sessR struct {
		ConsoleName string `gorm:"column:console_name"`
		Status      string `gorm:"column:status"`
		CreatedAt   string `gorm:"column:created_at"`
	}
	var sessions []sessR
	h.db.Raw(`SELECT c.name as console_name, s.status,
		to_char(s.created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as created_at
		FROM sessions s JOIN consoles c ON c.id = s.console_id
		ORDER BY s.created_at DESC LIMIT ?`, limit).Scan(&sessions)
	for _, s := range sessions {
		action := "started"
		if s.Status == "completed" {
			action = "ended"
		} else if s.Status == "cancelled" {
			action = "cancelled"
		}
		activities = append(activities, ActivityItem{
			Type: "session", Action: action, Title: s.ConsoleName,
			Detail:    "Sesi " + action,
			Timestamp: s.CreatedAt,
		})
	}

	// 4. Rental Harian
	type renR struct {
		ConsoleName string `gorm:"column:console_name"`
		Status      string `gorm:"column:status"`
		TotalDays   int    `gorm:"column:total_days"`
		CreatedAt   string `gorm:"column:created_at"`
	}
	var rentals []renR
	h.db.Raw(`SELECT c.name as console_name, d.status, d.total_days,
		to_char(d.created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as created_at
		FROM daily_rentals d JOIN consoles c ON c.id = d.console_id
		ORDER BY d.created_at DESC LIMIT ?`, limit).Scan(&rentals)
	for _, r := range rentals {
		action := "created"
		if r.Status == "returned" {
			action = "returned"
		}
		activities = append(activities, ActivityItem{
			Type: "daily_rental", Action: action, Title: r.ConsoleName,
			Detail:    fmt.Sprintf("Rental %s (%d hari)", action, r.TotalDays),
			Timestamp: r.CreatedAt,
		})
	}

	// 5. Membership
	type memR struct {
		CustomerName string `gorm:"column:customer_name"`
		CreatedAt    string `gorm:"column:created_at"`
	}
	var members []memR
	h.db.Raw(`SELECT cu.name as customer_name,
		to_char(p.paid_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as created_at
		FROM payments p JOIN customers cu ON cu.is_member = true
		WHERE p.session_id IS NULL AND p.payment_status = 'paid'
		ORDER BY p.paid_at DESC LIMIT ?`, limit).Scan(&members)
	for _, m := range members {
		activities = append(activities, ActivityItem{
			Type: "membership", Action: "purchased", Title: m.CustomerName,
			Detail:    "Menjadi member",
			Timestamp: m.CreatedAt,
		})
	}

	// 6. Pembayaran
	type payR struct {
		ConsoleName string  `gorm:"column:console_name"`
		Amount      float64 `gorm:"column:amount"`
		PaidAt      string  `gorm:"column:paid_at"`
	}
	var payments []payR
	h.db.Raw(`SELECT COALESCE(c.name, 'Membership') as console_name, p.amount,
		to_char(p.paid_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') as paid_at
		FROM payments p LEFT JOIN sessions s ON s.id = p.session_id
		LEFT JOIN consoles c ON c.id = s.console_id
		WHERE p.payment_status = 'paid' AND p.paid_at IS NOT NULL
		ORDER BY p.paid_at DESC LIMIT ?`, limit).Scan(&payments)
	for _, p := range payments {
		activities = append(activities, ActivityItem{
			Type: "payment", Action: "confirmed", Title: p.ConsoleName,
			Detail:    fmt.Sprintf("Pembayaran Rp %.0f", p.Amount),
			Timestamp: p.PaidAt,
		})
	}

	sort.Slice(activities, func(i, j int) bool {
		return activities[i].Timestamp > activities[j].Timestamp
	})
	if len(activities) > limit {
		activities = activities[:limit]
	}

	return response.OK(c, "Aktivitas terbaru", activities)
}
