package handler

import (
	"log"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditHandler menyediakan pembacaan audit trail untuk admin.
type AuditHandler struct {
	db *gorm.DB
}

func NewAuditHandler(db *gorm.DB) *AuditHandler {
	return &AuditHandler{db: db}
}

// List godoc
// @Summary      Riwayat aksi sensitif
// @Description  Menampilkan audit trail (siapa melakukan apa). Dapat difilter
//
//	berdasarkan action, user, dan rentang tanggal.
//
// @Tags         Audit
// @Produce      json
// @Security     BearerAuth
// @Param        action  query  string  false  "Filter aksi, mis. payment.refund"
// @Param        userId  query  string  false  "Filter user ID (UUID)"
// @Param        from    query  string  false  "Tanggal awal (YYYY-MM-DD)"
// @Param        to      query  string  false  "Tanggal akhir (YYYY-MM-DD)"
// @Param        limit   query  int     false  "Jumlah maksimum (default 100, max 500)"
// @Success      200  {object}  response.Response{data=[]entity.AuditLog}
// @Router       /api/v1/audit-logs [get]
func (h *AuditHandler) List(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 100)
	if limit < 1 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	query := h.db.WithContext(c.Context()).Model(&entity.AuditLog{})

	if action := c.Query("action"); action != "" {
		query = query.Where("action = ?", action)
	}
	if uidStr := c.Query("userId"); uidStr != "" {
		uid, err := uuid.Parse(uidStr)
		if err != nil {
			return response.BadRequest(c, "userId tidak valid")
		}
		query = query.Where("user_id = ?", uid)
	}
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			query = query.Where("created_at < ?", t.AddDate(0, 0, 1))
		}
	}

	var logs []entity.AuditLog
	if err := query.Order("created_at DESC").Limit(limit).Find(&logs).Error; err != nil {
		log.Printf("audit: gagal membaca audit_logs: %v", err)
		return response.InternalServerError(c, "Gagal mengambil audit log")
	}

	return response.OK(c, "Audit log berhasil diambil", logs)
}
