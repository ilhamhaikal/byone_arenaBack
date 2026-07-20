package entity

import (
	"time"

	"github.com/google/uuid"
)

// BookingStatus — status booking
type BookingStatus string

const (
	BookingPending   BookingStatus = "pending"
	BookingConfirmed BookingStatus = "confirmed"
	BookingCancelled BookingStatus = "cancelled"
	BookingCompleted BookingStatus = "completed"
)

// Booking — reservasi konsol untuk waktu tertentu
type Booking struct {
	ID              uuid.UUID     `json:"id"              gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	ConsoleID       uuid.UUID     `json:"consoleId"       gorm:"type:uuid;not null;index"`
	CustomerID      uuid.UUID     `json:"customerId"      gorm:"type:uuid;not null;index"`
	BookingDate     string        `json:"bookingDate"     gorm:"type:date;not null;index"`
	StartHour       int           `json:"startHour"       gorm:"not null"`
	StartMinute     int           `json:"startMinute"     gorm:"not null;default:0"`
	DurationMinutes int           `json:"durationMinutes" gorm:"not null"`
	Status          BookingStatus `json:"status"          gorm:"not null;default:'pending';size:20"`
	Notes           string        `json:"notes,omitempty" gorm:"type:text"`
	CreatedAt       time.Time     `json:"createdAt"       gorm:"autoCreateTime"`
	UpdatedAt       time.Time     `json:"updatedAt"       gorm:"autoUpdateTime"`

	Console  *Console  `json:"console,omitempty"  gorm:"foreignKey:ConsoleID"`
	Customer *Customer `json:"customer,omitempty" gorm:"foreignKey:CustomerID"`
}

func (Booking) TableName() string { return "bookings" }
