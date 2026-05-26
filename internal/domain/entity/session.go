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

// Session merepresentasikan satu sesi penyewaan konsol
type Session struct {
	ID              uuid.UUID     `json:"id"                     gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	ConsoleID       uuid.UUID     `json:"consoleId"              gorm:"type:uuid;not null;index"`
	CustomerID      *uuid.UUID    `json:"customerId,omitempty"   gorm:"type:uuid;index"` // nullable, walk-in
	StartTime       time.Time     `json:"startTime"              gorm:"not null"`
	EndTime         *time.Time    `json:"endTime,omitempty"`
	DurationMinutes int           `json:"durationMinutes"        gorm:"not null;default:0"`
	TotalPrice      float64       `json:"totalPrice"             gorm:"not null;default:0;type:numeric(10,2)"`
	Status          SessionStatus `json:"status"                 gorm:"not null;default:'active';size:20;index"`
	Notes           string        `json:"notes,omitempty"        gorm:"type:text"`
	CreatedAt       time.Time     `json:"createdAt"              gorm:"autoCreateTime"`
	UpdatedAt       time.Time     `json:"updatedAt"              gorm:"autoUpdateTime"`

	// Relasi (join data)
	Console  *Console  `json:"console,omitempty"  gorm:"foreignKey:ConsoleID"`
	Customer *Customer `json:"customer,omitempty" gorm:"foreignKey:CustomerID"`
}

func (Session) TableName() string { return "sessions" }

// CalculateDuration menghitung durasi sesi dalam menit
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
