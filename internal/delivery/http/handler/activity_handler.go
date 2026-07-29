package handler

import (
	"byone-arena/pkg/response"
	"byone-arena/pkg/spname"

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
	Amount    string `json:"amount,omitempty"`
}

// activityRow hasil dari SP byoneGetRecentActivities
type activityRow struct {
	Typ      string `gorm:"column:typ"`
	Title    string `gorm:"column:title"`
	Subtitle string `gorm:"column:subtitle"`
	Amount   string `gorm:"column:amount"`
	Ts       string `gorm:"column:ts"`
}

// GetRecentActivities godoc
// @Summary      Aktivitas terbaru (realtime)
// @Description  Mengembalikan aktivitas terbaru dari 8 sumber: konsol, settings, sesi, rental, membership, pembayaran, voucher, food order.
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

	spName := spname.Ident("GetRecentActivities")
	var rows []activityRow
	if err := h.db.Raw("SELECT * FROM "+spName+"(?)", limit).Scan(&rows).Error; err != nil {
		return response.InternalServerError(c, "Gagal mengambil aktivitas")
	}

	activities := make([]ActivityItem, 0, len(rows))
	for _, r := range rows {
		action := "updated"
		switch r.Typ {
		case "session":
			if r.Subtitle == "Sesi cancelled" {
				action = "cancelled"
			} else if len(r.Subtitle) > 12 && r.Subtitle[:12] == "Sesi ended ·" {
				action = "ended"
			} else {
				action = "started"
			}
		case "daily_rental":
			if len(r.Subtitle) > 14 && r.Subtitle[7:15] == "returned" {
				action = "returned"
			} else {
				action = "created"
			}
		case "membership":
			action = "purchased"
		case "payment":
			action = "confirmed"
		case "voucher":
			action = "used"
		case "food_order":
			action = "updated"
		}

		if r.Amount == "" {
			r.Amount = ""
		}
		detail := r.Subtitle
		if r.Amount != "" && r.Typ == "payment" {
			detail = r.Subtitle
		}

		activities = append(activities, ActivityItem{
			Type:      r.Typ,
			Action:    action,
			Title:     r.Title,
			Detail:    detail,
			Amount:    r.Amount,
			Timestamp: r.Ts,
		})
	}

	return response.OK(c, "Aktivitas terbaru", activities)
}
