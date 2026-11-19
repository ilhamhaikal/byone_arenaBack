package entity

import (
	"time"

	"github.com/google/uuid"
)

// PendingDevice adalah TV/box yang sudah menemukan server tetapi belum
// dihubungkan ke konsol mana pun oleh admin.
//
// Alur: aplikasi TV memanggil POST /devices/announce -> server membuat kode
// pairing 6 digit -> TV menampilkan kode -> admin mengetik kode di aplikasi
// admin (POST /devices/bind-by-code) -> server menulis consoles.ip_address
// dan consoles.device_id.
type PendingDevice struct {
	ID uuid.UUID `json:"id"         gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	// DeviceID adalah identitas tetap perangkat TV (dibuat aplikasi TV dan
	// disimpan di SharedPreferences). Identitas memakai ID ini, bukan IP,
	// supaya TV tetap dikenali walau IP-nya berubah (DHCP).
	DeviceID   *uuid.UUID `json:"deviceId,omitempty" gorm:"type:uuid;index"`
	IP         string     `json:"ip"         gorm:"not null;size:50"`
	Name       string     `json:"name"       gorm:"size:100;not null;default:''"`
	Model      string     `json:"model"      gorm:"size:100;not null;default:''"`
	AppVersion string     `json:"appVersion" gorm:"size:30;not null;default:''"`
	// PairingCode adalah kode 6 digit yang tampil di layar TV dan diketik admin.
	PairingCode *string `json:"pairingCode,omitempty" gorm:"size:6;index"`
	// CodeExpiresAt menentukan masa berlaku kode. Setelah lewat, TV akan
	// menampilkan kode baru saat announce berikutnya.
	CodeExpiresAt *time.Time `json:"codeExpiresAt,omitempty"`
	LastSeen      time.Time  `json:"lastSeen"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

func (PendingDevice) TableName() string { return "pending_devices" }
