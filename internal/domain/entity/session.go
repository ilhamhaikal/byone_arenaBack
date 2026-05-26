package entity

import (
	"time"

	"github.com/google/uuid"
)

// SessionStatus mendefinisikan status sesi rental
type SessionStatus string

const (
	SessionStatusActive    SessionStatus = "active"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusCancelled SessionStatus = "cancelled"
)

// Session merepresentasikan satu sesi penyewaan konsol / TV Android
type Session struct {
	ID                     uuid.UUID     `json:"id"                     gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	ConsoleID              uuid.UUID     `json:"consoleId"              gorm:"type:uuid;not null;index"`
	CustomerID             *uuid.UUID    `json:"customerId,omitempty"   gorm:"type:uuid;index"` // nullable, walk-in
	StartTime              time.Time     `json:"startTime"              gorm:"not null"`
	EndTime                *time.Time    `json:"endTime,omitempty"`
	// BookedDurationMinutes adalah durasi yang dipesan di awal (contoh: 120 menit = 2 jam)
	BookedDurationMinutes  int           `json:"bookedDurationMinutes"  gorm:"not null;default:0"`
	// EndScheduledAt adalah waktu selesai yang direncanakan (StartTime + BookedDurationMinutes)
	EndScheduledAt         *time.Time    `json:"endScheduledAt,omitempty"`
	DurationMinutes        int           `json:"durationMinutes"        gorm:"not null;default:0"`
	TotalPrice             float64       `json:"totalPrice"             gorm:"not null;default:0;type:numeric(10,2)"`
	Status                 SessionStatus `json:"status"                 gorm:"not null;default:'active';size:20;index"`
	Notes                  string        `json:"notes,omitempty"        gorm:"type:text"`
	CreatedAt              time.Time     `json:"createdAt"              gorm:"autoCreateTime"`
	UpdatedAt              time.Time     `json:"updatedAt"              gorm:"autoUpdateTime"`

	// Relasi (join data)
	Console  *Console  `json:"console,omitempty"  gorm:"foreignKey:ConsoleID"`
	Customer *Customer `json:"customer,omitempty" gorm:"foreignKey:CustomerID"`
}

func (Session) TableName() string { return "sessions" }

// CalculateDuration menghitung durasi sesi aktual dalam menit
func (s *Session) CalculateDuration() int {
	if s.EndTime == nil {
		return int(time.Since(s.StartTime).Minutes())
	}
	return int(s.EndTime.Sub(s.StartTime).Minutes())
}

// CalculateTotalPrice menghitung total harga berdasarkan durasi dan harga per jam
func (s *Session) CalculateTotalPrice(pricePerHour float64) float64 {
	durationHours := float64(s.CalculateDuration()) / 60.0
	return durationHours * pricePerHour
}

// RemainingMinutes menghitung sisa menit sesi berdasarkan durasi yang dipesan.
// Mengembalikan -1 jika sesi tidak memiliki durasi pre-book (open-ended).
// Mengembalikan 0 jika sesi sudah melewati jadwal selesai.
func (s *Session) RemainingMinutes() int {
	if s.EndScheduledAt == nil {
		return -1
	}
	rem := int(time.Until(*s.EndScheduledAt).Minutes())
	if rem < 0 {
		return 0
	}
	return rem
}
