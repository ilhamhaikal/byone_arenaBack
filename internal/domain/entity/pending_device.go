package entity

import (
	"time"

	"github.com/google/uuid"
)

// PendingDevice adalah TV/box yang sudah menemukan server tetapi belum
// dihubungkan ke konsol mana pun oleh admin.
//
// Alur: aplikasi TV broadcast "BYONE_DISCOVER" -> server membalas alamat API
// -> TV memanggil POST /devices/announce -> baris muncul di sini -> admin
// memilih konsol -> server menulis consoles.ip_address = pending_devices.ip.
type PendingDevice struct {
	ID         uuid.UUID `json:"id"         gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	IP         string    `json:"ip"         gorm:"not null;uniqueIndex;size:50"`
	Name       string    `json:"name"       gorm:"size:100;not null;default:''"`
	Model      string    `json:"model"      gorm:"size:100;not null;default:''"`
	AppVersion string    `json:"appVersion" gorm:"size:30;not null;default:''"`
	LastSeen   time.Time `json:"lastSeen"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (PendingDevice) TableName() string { return "pending_devices" }
