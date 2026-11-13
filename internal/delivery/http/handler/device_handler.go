package handler

import (
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DeviceHandler menangani pendaftaran TV baru (pairing) sebelum dihubungkan
// ke sebuah konsol oleh admin.
type DeviceHandler struct {
	db *gorm.DB
}

func NewDeviceHandler(db *gorm.DB) *DeviceHandler {
	return &DeviceHandler{db: db}
}

type announceDeviceRequest struct {
	Name       string `json:"name"`
	Model      string `json:"model"`
	AppVersion string `json:"appVersion"`
}

// Announce godoc
// @Summary      Daftarkan TV yang baru menemukan server
// @Description  Dipanggil aplikasi TV (tanpa login). Server memakai IP sumber
//               request sehingga TV tidak perlu mengetik alamat apa pun.
// @Tags         Perangkat
// @Accept       json
// @Produce      json
// @Success      200 {object} response.Response
// @Router       /api/v1/devices/announce [post]
func (h *DeviceHandler) Announce(c *fiber.Ctx) error {
	req := new(announceDeviceRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}

	ip := c.IP()
	now := time.Now()

	dev := entity.PendingDevice{
		IP:         ip,
		Name:       req.Name,
		Model:      req.Model,
		AppVersion: req.AppVersion,
		LastSeen:   now,
	}

	err := h.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "ip"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"name":        req.Name,
			"model":       req.Model,
			"app_version": req.AppVersion,
			"last_seen":   now,
			"updated_at":  now,
		}),
	}).Create(&dev).Error
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}

	if err := h.db.Where("ip = ?", ip).First(&dev).Error; err != nil {
		return response.InternalServerError(c, err.Error())
	}

	return response.OK(c, "Perangkat terdaftar, menunggu persetujuan admin", dev)
}

// Pending godoc
// @Summary      Daftar TV yang menunggu dihubungkan
// @Tags         Perangkat
// @Security     BearerAuth
// @Router       /api/v1/devices/pending [get]
func (h *DeviceHandler) Pending(c *fiber.Ctx) error {
	var devices []entity.PendingDevice
	cutoff := time.Now().Add(-15 * time.Minute)
	if err := h.db.Where("last_seen > ?", cutoff).
		Order("last_seen DESC").Find(&devices).Error; err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.OK(c, "Daftar perangkat menunggu", devices)
}

type bindDeviceRequest struct {
	ConsoleID string `json:"consoleId"`
}

// Bind godoc
// @Summary      Hubungkan TV ke konsol
// @Description  Menulis IP perangkat ke consoles.ip_address sehingga aplikasi
//               TV langsung menemukan konsolnya.
// @Tags         Perangkat
// @Security     BearerAuth
// @Router       /api/v1/devices/{id}/bind [post]
func (h *DeviceHandler) Bind(c *fiber.Ctx) error {
	deviceID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID perangkat tidak valid")
	}

	req := new(bindDeviceRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	consoleID, err := uuid.Parse(req.ConsoleID)
	if err != nil {
		return response.BadRequest(c, "consoleId tidak valid")
	}

	var dev entity.PendingDevice
	if err := h.db.First(&dev, "id = ?", deviceID).Error; err != nil {
		return response.NotFound(c, "Perangkat tidak ditemukan")
	}

	now := time.Now()
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	res := tx.Model(&entity.Console{}).Where("id = ?", consoleID).Updates(map[string]interface{}{
		"ip_address":   dev.IP,
		"last_seen_at": now,
		"updated_at":   now,
	})
	if res.Error != nil {
		tx.Rollback()
		return response.InternalServerError(c, res.Error.Error())
	}
	if res.RowsAffected == 0 {
		tx.Rollback()
		return response.NotFound(c, "Konsol tidak ditemukan")
	}

	if err := tx.Delete(&entity.PendingDevice{}, "id = ?", dev.ID).Error; err != nil {
		tx.Rollback()
		return response.InternalServerError(c, err.Error())
	}
	if err := tx.Commit().Error; err != nil {
		return response.InternalServerError(c, err.Error())
	}

	return response.OK(c, "TV berhasil dihubungkan ke konsol", fiber.Map{
		"deviceId":  dev.ID,
		"consoleId": consoleID,
		"ipAddress": dev.IP,
	})
}

// Dismiss godoc
// @Summary      Hapus TV dari daftar menunggu
// @Tags         Perangkat
// @Security     BearerAuth
// @Router       /api/v1/devices/{id} [delete]
func (h *DeviceHandler) Dismiss(c *fiber.Ctx) error {
	deviceID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID perangkat tidak valid")
	}
	if err := h.db.Delete(&entity.PendingDevice{}, "id = ?", deviceID).Error; err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.OK(c, "Perangkat dihapus dari daftar", nil)
}