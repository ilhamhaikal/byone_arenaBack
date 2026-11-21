package handler

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/license"
	"byone-arena/pkg/audit"
	"byone-arena/pkg/deviceauth"
	"byone-arena/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// pairingCodeTTL adalah masa berlaku kode pairing. Setelah lewat, TV akan
// menampilkan kode baru pada announce berikutnya.
const pairingCodeTTL = 15 * time.Minute

// DeviceHandler menangani pendaftaran TV baru (pairing) sebelum dihubungkan
// ke sebuah konsol oleh admin.
type DeviceHandler struct {
	db      *gorm.DB
	license *license.Service
}

func NewDeviceHandler(db *gorm.DB, licenseSvc *license.Service) *DeviceHandler {
	return &DeviceHandler{db: db, license: licenseSvc}
}

type announceDeviceRequest struct {
	// DeviceID dibuat & disimpan aplikasi TV. Opsional untuk kompatibilitas:
	// bila kosong/tidak valid, server memakai UUID turunan dari IP.
	DeviceID         string `json:"deviceId"`
	DeviceSecretHash string `json:"deviceSecretHash"`
	Name             string `json:"name"`
	Model            string `json:"model"`
	AppVersion       string `json:"appVersion"`
}

// Announce godoc
// @Summary      Daftarkan TV yang baru menemukan server
// @Description  Dipanggil aplikasi TV (tanpa login). Server membuat/memperbarui
// perangkat dan mengembalikan `pairingCode` 6 digit untuk ditampilkan di layar
// TV. Bila perangkat sudah terhubung ke konsol, respons berisi `bound: true`
// beserta `consoleId`.
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

	deviceID, err := uuid.Parse(strings.TrimSpace(req.DeviceID))
	if err != nil {
		// Fallback deterministik: perangkat lama tanpa deviceId tetap dikenali
		// per IP, bukan ditolak.
		deviceID = uuid.NewSHA1(uuid.NameSpaceOID, []byte("byone-device:"+ip))
	}

	secretHash := strings.ToLower(strings.TrimSpace(req.DeviceSecretHash))
	if !deviceauth.IsValidHash(secretHash) {
		return response.BadRequest(c, "deviceSecretHash wajib berupa hash SHA-256 yang valid")
	}

	// 1. Sudah terhubung ke konsol? → verifikasi secret sebelum membocorkan
	// informasi konsol atau menerima heartbeat.
	var bound entity.Console
	if err := h.db.WithContext(c.Context()).
		Where("device_id = ?", deviceID).First(&bound).Error; err == nil {
		if bound.DeviceSecretHash == "" ||
			!deviceauth.EqualHash(bound.DeviceSecretHash, secretHash) {
			return response.OK(c, "Perangkat perlu dihubungkan ulang", fiber.Map{
				"deviceId":       deviceID,
				"bound":          false,
				"reauthRequired": true,
			})
		}

		updates := map[string]interface{}{
			"ip_address":   ip,
			"last_seen_at": now,
			"updated_at":   now,
		}
		if bound.IPAddress == nil || *bound.IPAddress != ip {
			audit.Log(h.db, c, "device.ip_changed", "console", &bound.ID, map[string]any{
				"deviceId": deviceID,
				"oldIp":    bound.IPAddress,
				"newIp":    ip,
			})
		}
		if err := h.db.WithContext(c.Context()).Model(&entity.Console{}).
			Where("id = ?", bound.ID).Updates(updates).Error; err != nil {
			return response.InternalServerError(c, err.Error())
		}
		h.db.WithContext(c.Context()).
			Where("device_id = ?", deviceID).Delete(&entity.PendingDevice{})
		return response.OK(c, "Perangkat sudah terhubung", fiber.Map{
			"deviceId":    deviceID,
			"bound":       true,
			"consoleId":   bound.ID,
			"consoleName": bound.Name,
		})
	}

	// 2. Cari baris pairing yang sudah ada untuk perangkat ini.
	var dev entity.PendingDevice
	findErr := h.db.WithContext(c.Context()).
		Where("device_id = ?", deviceID).First(&dev).Error
	if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return response.InternalServerError(c, findErr.Error())
	}

	codeNeedsRefresh := findErr != nil ||
		dev.PairingCode == nil || *dev.PairingCode == "" ||
		dev.CodeExpiresAt == nil || dev.CodeExpiresAt.Before(now)

	if findErr != nil {
		// 3a. Perangkat baru → buat baris + kode.
		code, codeErr := h.generatePairingCode(c)
		if codeErr != nil {
			return response.InternalServerError(c, "Gagal membuat kode pairing")
		}
		expires := now.Add(pairingCodeTTL)
		dev = entity.PendingDevice{
			DeviceID:         &deviceID,
			DeviceSecretHash: secretHash,
			IP:               ip,
			Name:             req.Name,
			Model:            req.Model,
			AppVersion:       req.AppVersion,
			PairingCode:      &code,
			CodeExpiresAt:    &expires,
			LastSeen:         now,
		}
		if err := h.db.WithContext(c.Context()).Create(&dev).Error; err != nil {
			return response.InternalServerError(c, err.Error())
		}
	} else {
		// Pending device dengan kredensial berbeda tidak boleh diambil alih.
		if dev.DeviceSecretHash != "" &&
			!deviceauth.EqualHash(dev.DeviceSecretHash, secretHash) {
			return response.Conflict(c,
				"Perangkat sudah terdaftar dengan kredensial berbeda. Minta admin menghapus perangkat lama.")
		}

		// 3b. Perangkat sudah dikenal → perbarui info & mungkin kode.
		updates := map[string]interface{}{
			"ip":                 ip,
			"name":               req.Name,
			"model":              req.Model,
			"app_version":        req.AppVersion,
			"device_secret_hash": secretHash,
			"last_seen":          now,
			"updated_at":         now,
		}
		if codeNeedsRefresh {
			code, codeErr := h.generatePairingCode(c)
			if codeErr != nil {
				return response.InternalServerError(c, "Gagal membuat kode pairing")
			}
			expires := now.Add(pairingCodeTTL)
			updates["pairing_code"] = code
			updates["code_expires_at"] = expires
		}
		if err := h.db.WithContext(c.Context()).Model(&entity.PendingDevice{}).
			Where("id = ?", dev.ID).Updates(updates).Error; err != nil {
			return response.InternalServerError(c, err.Error())
		}
		if err := h.db.WithContext(c.Context()).First(&dev, "id = ?", dev.ID).Error; err != nil {
			return response.InternalServerError(c, err.Error())
		}
	}

	return response.OK(c, "Perangkat terdaftar, menunggu dihubungkan admin", fiber.Map{
		"deviceId":      dev.DeviceID,
		"bound":         false,
		"pairingCode":   dev.PairingCode,
		"codeExpiresAt": dev.CodeExpiresAt,
	})
}

// generatePairingCode membuat kode 6 digit (100000–999999) yang belum dipakai
// perangkat lain. Memakai crypto/rand agar tidak mudah ditebak.
func (h *DeviceHandler) generatePairingCode(c *fiber.Ctx) (string, error) {
	for i := 0; i < 25; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(900000))
		if err != nil {
			return "", err
		}
		code := fmt.Sprintf("%06d", n.Int64()+100000)

		var count int64
		if err := h.db.WithContext(c.Context()).
			Model(&entity.PendingDevice{}).
			Where("pairing_code = ?", code).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return code, nil
		}
	}
	return "", errors.New("kode pairing penuh, coba lagi")
}

// Pending godoc
// @Summary      Daftar TV yang menunggu dihubungkan
// @Description  Perangkat yang masih mengirim announce dalam 15 menit terakhir.
// @Tags         Perangkat
// @Security     BearerAuth
// @Router       /api/v1/devices/pending [get]
func (h *DeviceHandler) Pending(c *fiber.Ctx) error {
	var devices []entity.PendingDevice
	cutoff := time.Now().Add(-15 * time.Minute)
	if err := h.db.WithContext(c.Context()).
		Where("last_seen > ?", cutoff).
		Order("last_seen DESC").Find(&devices).Error; err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.OK(c, "Daftar perangkat menunggu", devices)
}

type resolveCodeRequest struct {
	Code string `json:"code"`
}

// ResolveByCode godoc
// @Summary      Validasi kode pairing tanpa mengikat konsol
// @Description  Dipakai form Tambah Konsol: admin mengetik kode 6 digit dari
// layar TV, server mengembalikan IP & identitas perangkat. Kode TIDAK
// dihapus, sehingga bisa langsung dipakai saat menyimpan konsol
// (POST /devices/bind-by-code).
// @Tags         Perangkat
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} response.Response
// @Router       /api/v1/devices/resolve-code [post]
func (h *DeviceHandler) ResolveByCode(c *fiber.Ctx) error {
	req := new(resolveCodeRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}

	code := normalizePairingCode(req.Code)
	if len(code) != 6 {
		return response.BadRequest(c, "Kode pairing harus 6 digit")
	}

	var dev entity.PendingDevice
	err := h.db.WithContext(c.Context()).
		Where("pairing_code = ?", code).First(&dev).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return response.NotFound(c, "Kode tidak ditemukan atau sudah dipakai")
	}
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	if dev.CodeExpiresAt == nil || dev.CodeExpiresAt.Before(time.Now()) {
		return response.BadRequest(c, "Kode sudah kedaluwarsa. Lihat kode terbaru di layar TV.")
	}

	return response.OK(c, "Kode pairing valid", fiber.Map{
		"deviceId":      dev.DeviceID,
		"ipAddress":     dev.IP,
		"name":          dev.Name,
		"model":         dev.Model,
		"codeExpiresAt": dev.CodeExpiresAt,
	})
}

type bindByCodeRequest struct {
	Code      string `json:"code"`
	ConsoleID string `json:"consoleId"`
}

// BindByCode godoc
// @Summary      Hubungkan TV ke konsol memakai kode pairing
// @Description  Admin mengetik kode 6 digit yang tampil di layar TV. Server
// mengikat konsol ke perangkat tersebut (ip_address + device_id) lalu kode
// dianggap terpakai.
// @Tags         Perangkat
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} response.Response
// @Router       /api/v1/devices/bind-by-code [post]
func (h *DeviceHandler) BindByCode(c *fiber.Ctx) error {
	req := new(bindByCodeRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}

	code := normalizePairingCode(req.Code)
	if len(code) != 6 {
		return response.BadRequest(c, "Kode pairing harus 6 digit")
	}
	consoleID, err := uuid.Parse(strings.TrimSpace(req.ConsoleID))
	if err != nil {
		return response.BadRequest(c, "Pilih konsol terlebih dahulu")
	}

	var dev entity.PendingDevice
	err = h.db.WithContext(c.Context()).
		Where("pairing_code = ?", code).First(&dev).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return response.NotFound(c, "Kode tidak ditemukan atau sudah dipakai")
	}
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	if dev.CodeExpiresAt == nil || dev.CodeExpiresAt.Before(time.Now()) {
		return response.BadRequest(c, "Kode sudah kedaluwarsa. Lihat kode terbaru di layar TV.")
	}

	var console entity.Console
	if err := h.db.WithContext(c.Context()).
		First(&console, "id = ?", consoleID).Error; err != nil {
		return response.NotFound(c, "Konsol tidak ditemukan")
	}

	if strings.TrimSpace(dev.DeviceSecretHash) == "" {
		return response.BadRequest(c,
			"Perangkat belum memiliki kredensial. Minta TV melakukan pairing ulang.")
	}

	// Konsol sudah dipakai TV lain → tolak agar tidak salah pindah tanpa sadar.
	if console.DeviceID != nil && (dev.DeviceID == nil || *console.DeviceID != *dev.DeviceID) {
		return response.Conflict(c,
			"Konsol ini sudah terhubung ke TV lain. Lepaskan dulu TV lama.")
	}

	// TV ini sudah terhubung ke konsol lain → tolak juga.
	if dev.DeviceID != nil {
		var other entity.Console
		if err := h.db.WithContext(c.Context()).
			Where("device_id = ? AND id <> ?", *dev.DeviceID, consoleID).
			First(&other).Error; err == nil {
			return response.Conflict(c, fmt.Sprintf(
				"TV ini sudah terhubung ke konsol %s. Lepaskan dulu di menu Konsol.",
				other.Name))
		}
	}

	// Batas unit lisensi — hanya bila konsol ini belum punya TV.
	if console.DeviceID == nil && h.license != nil {
		if err := h.license.EnsureCanBindUnit(c.Context()); err != nil {
			return response.Forbidden(c, err.Error())
		}
	}

	now := time.Now()
	err = h.db.WithContext(c.Context()).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&entity.Console{}).Where("id = ?", consoleID).
			Updates(map[string]interface{}{
				"ip_address":         dev.IP,
				"device_id":          dev.DeviceID,
				"device_secret_hash": dev.DeviceSecretHash,
				"last_seen_at":       now,
				"updated_at":         now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Where("id = ?", dev.ID).Delete(&entity.PendingDevice{}).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "Konsol tidak ditemukan")
		}
		return response.InternalServerError(c, err.Error())
	}

	return response.OK(c, "TV berhasil dihubungkan ke konsol", fiber.Map{
		"deviceId":    dev.DeviceID,
		"consoleId":   console.ID,
		"consoleName": console.Name,
		"ipAddress":   dev.IP,
	})
}

// normalizePairingCode menyisakan digit saja, agar admin boleh mengetik
// "482 913" atau "482-913".
func normalizePairingCode(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

type bindDeviceRequest struct {
	ConsoleID string `json:"consoleId"`
}

// Bind godoc
// @Summary      Hubungkan TV ke konsol (berdasarkan ID perangkat)
// @Description  Alternatif dari bind-by-code; dipakai daftar "Perangkat menunggu".
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
	if err := h.db.WithContext(c.Context()).
		First(&dev, "id = ?", deviceID).Error; err != nil {
		return response.NotFound(c, "Perangkat tidak ditemukan")
	}
	if strings.TrimSpace(dev.DeviceSecretHash) == "" {
		return response.BadRequest(c,
			"Perangkat belum memiliki kredensial. Minta TV melakukan pairing ulang.")
	}

	var console entity.Console
	if err := h.db.WithContext(c.Context()).
		First(&console, "id = ?", consoleID).Error; err != nil {
		return response.NotFound(c, "Konsol tidak ditemukan")
	}

	// Konsol sudah dipakai TV lain → tolak.
	if console.DeviceID != nil &&
		(dev.DeviceID == nil || *console.DeviceID != *dev.DeviceID) {
		return response.Conflict(c,
			"Konsol ini sudah terhubung ke TV lain. Lepaskan dulu TV lama.")
	}
	// TV ini sudah terhubung ke konsol lain → tolak.
	if dev.DeviceID != nil {
		var other entity.Console
		if err := h.db.WithContext(c.Context()).
			Where("device_id = ? AND id <> ?", *dev.DeviceID, consoleID).
			First(&other).Error; err == nil {
			return response.Conflict(c, fmt.Sprintf(
				"TV ini sudah terhubung ke konsol %s. Lepaskan dulu di menu Konsol.",
				other.Name))
		}
	}

	if console.DeviceID == nil && h.license != nil {
		if err := h.license.EnsureCanBindUnit(c.Context()); err != nil {
			return response.Forbidden(c, err.Error())
		}
	}

	now := time.Now()
	err = h.db.WithContext(c.Context()).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&entity.Console{}).Where("id = ?", consoleID).
			Updates(map[string]interface{}{
				"ip_address":         dev.IP,
				"device_id":          dev.DeviceID,
				"device_secret_hash": dev.DeviceSecretHash,
				"last_seen_at":       now,
				"updated_at":         now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Delete(&entity.PendingDevice{}, "id = ?", dev.ID).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "Konsol tidak ditemukan")
		}
		return response.InternalServerError(c, err.Error())
	}

	return response.OK(c, "TV berhasil dihubungkan ke konsol", fiber.Map{
		"deviceId":  deviceID,
		"consoleId": consoleID,
		"ipAddress": dev.IP,
	})
}

// UnbindConsole godoc
// @Summary      Lepaskan TV dari sebuah konsol
// @Description  Menghapus ikatan device_id & ip_address pada konsol sehingga
// TV lain bisa dihubungkan. TV lama akan menampilkan kode pairing baru.
// @Tags         Perangkat
// @Security     BearerAuth
// @Router       /api/v1/devices/console/{console_id} [delete]
func (h *DeviceHandler) UnbindConsole(c *fiber.Ctx) error {
	consoleID, err := uuid.Parse(c.Params("console_id"))
	if err != nil {
		return response.BadRequest(c, "ID konsol tidak valid")
	}

	res := h.db.WithContext(c.Context()).Model(&entity.Console{}).
		Where("id = ?", consoleID).
		Updates(map[string]interface{}{
			"ip_address":         nil,
			"device_id":          nil,
			"device_secret_hash": nil,
			"updated_at":         time.Now(),
		})
	if res.Error != nil {
		return response.InternalServerError(c, res.Error.Error())
	}
	if res.RowsAffected == 0 {
		return response.NotFound(c, "Konsol tidak ditemukan")
	}
	return response.OK(c, "TV berhasil dilepas dari konsol", nil)
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
	if err := h.db.WithContext(c.Context()).
		Delete(&entity.PendingDevice{}, "id = ?", deviceID).Error; err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.OK(c, "Perangkat dihapus dari daftar", nil)
}
