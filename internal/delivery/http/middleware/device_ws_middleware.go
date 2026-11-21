package middleware

import (
	"strings"

	"byone-arena/internal/domain/entity"
	"byone-arena/pkg/deviceauth"
	"byone-arena/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DeviceWSMiddleware memverifikasi secret perangkat TV sebelum WebSocket
// di-upgrade. Hasil verifikasi disimpan di Locals("consoleID") agar hub bisa
// memfilter event per konsol.
func DeviceWSMiddleware(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		secret := strings.TrimSpace(c.Get("X-Device-Secret"))
		deviceIDRaw := strings.TrimSpace(c.Get("X-Device-Id"))
		// Fallback untuk platform web yang tidak bisa mengirim header kustom
		// saat upgrade. Android TV tetap memakai header.
		if secret == "" {
			secret = strings.TrimSpace(c.Query("secret"))
		}
		if deviceIDRaw == "" {
			deviceIDRaw = strings.TrimSpace(c.Query("deviceId"))
		}
		if secret == "" || deviceIDRaw == "" {
			return response.Unauthorized(c, "Kredensial perangkat tidak ditemukan")
		}
		deviceID, err := uuid.Parse(deviceIDRaw)
		if err != nil {
			return response.Unauthorized(c, "Device ID tidak valid")
		}

		var console entity.Console
		if err := db.WithContext(c.Context()).
			Where("device_id = ?", deviceID).
			First(&console).Error; err != nil {
			return response.Unauthorized(c, "Perangkat belum terhubung ke konsol")
		}
		if console.DeviceSecretHash == "" ||
			!deviceauth.EqualHash(console.DeviceSecretHash, deviceauth.HashSecret(secret)) {
			return response.Unauthorized(c, "Kredensial perangkat tidak valid")
		}

		c.Locals("consoleID", console.ID)
		c.Locals("deviceID", deviceID)
		return c.Next()
	}
}
