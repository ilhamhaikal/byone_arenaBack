package entity

import (
	"time"

	"github.com/google/uuid"
)

// ShiftStatus mendefinisikan status shift kasir
type ShiftStatus string

const (
	ShiftStatusActive   ShiftStatus = "active"
	ShiftStatusInactive ShiftStatus = "inactive"
)

// Shift merepresentasikan jadwal shift untuk kasir
// StartHour dan EndHour menggunakan format 24 jam (0-23)
// Jika Is24Hour = true, kasir dapat login kapan saja tanpa batasan jam
type Shift struct {
	ID        uuid.UUID   `json:"id"          gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID    uuid.UUID   `json:"userId"      gorm:"type:uuid;not null;index"`
	Name      string      `json:"name"        gorm:"not null;size:100"`      // contoh: "Shift Pagi", "Shift Malam"
	StartHour int         `json:"startHour"   gorm:"not null"`               // 0-23 (jam mulai)
	EndHour   int         `json:"endHour"     gorm:"not null"`               // 0-23 (jam selesai)
	Is24Hour  bool        `json:"is24Hour"    gorm:"not null;default:false"` // true = bisa login kapan saja
	Status    ShiftStatus `json:"status"      gorm:"not null;default:'active';size:20"`
	CreatedAt time.Time   `json:"createdAt"   gorm:"autoCreateTime"`
	UpdatedAt time.Time   `json:"updatedAt"   gorm:"autoUpdateTime"`

	// Relasi (join data)
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

func (Shift) TableName() string { return "shifts" }

// IsLoginAllowed mengecek apakah kasir boleh login berdasarkan jam saat ini
func (s *Shift) IsLoginAllowed(now time.Time) bool {
	if s.Status != ShiftStatusActive {
		return false
	}
	if s.Is24Hour {
		return true
	}

	currentHour := now.Hour()

	// Handle shift yang melewati tengah malam, misal StartHour=22 EndHour=6
	if s.StartHour > s.EndHour {
		return currentHour >= s.StartHour || currentHour < s.EndHour
	}

	// Shift normal dalam hari yang sama, misal StartHour=8 EndHour=16
	return currentHour >= s.StartHour && currentHour < s.EndHour
}
