// Package audit menyediakan pencatatan aksi sensitif beserta pelakunya.
//
// Semua pemanggilan bersifat best-effort: kegagalan menulis audit tidak boleh
// menggagalkan transaksi bisnis, tetapi dicatat ke log aplikasi.
package audit

import (
	"byone-arena/internal/domain/entity"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Log menulis satu baris audit dari konteks request (user diambil dari JWT).
func Log(db *gorm.DB, c *fiber.Ctx, action, entityName string, entityID *uuid.UUID, detail map[string]any) {
	if db == nil {
		return
	}
	if detail == nil {
		detail = map[string]any{}
	}

	entry := entity.AuditLog{
		Action:   action,
		Entity:   entityName,
		EntityID: entityID,
		Detail:   detail,
	}

	if c != nil {
		entry.IP = c.IP()
		if uid, ok := c.Locals("userID").(uuid.UUID); ok {
			entry.UserID = &uid
		}
		if uname, ok := c.Locals("username").(string); ok {
			entry.Username = uname
		}
		if role, ok := c.Locals("role").(string); ok {
			entry.Role = role
		} else if r, ok := c.Locals("role").(entity.UserRole); ok {
			entry.Role = string(r)
		}
	}

	_ = db.Create(&entry).Error
}

// CurrentUserID mengambil user ID dari konteks (nil bila tidak ada).
func CurrentUserID(c *fiber.Ctx) *uuid.UUID {
	if c == nil {
		return nil
	}
	if uid, ok := c.Locals("userID").(uuid.UUID); ok {
		return &uid
	}
	return nil
}
