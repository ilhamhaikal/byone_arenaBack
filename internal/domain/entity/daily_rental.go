package entity

import (
	"time"

	"github.com/google/uuid"
)

// DailyRentalStatus — status rental harian
type DailyRentalStatus string

const (
	DailyRentalActive   DailyRentalStatus = "active"
	DailyRentalReturned DailyRentalStatus = "returned"
	DailyRentalOverdue  DailyRentalStatus = "overdue"
)

// DailyRental — sewa konsol harian (dibawa pulang)
type DailyRental struct {
	ID            uuid.UUID         `json:"id"            gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	ConsoleID     uuid.UUID         `json:"consoleId"     gorm:"type:uuid;not null;index"`
	CustomerID    uuid.UUID         `json:"customerId"    gorm:"type:uuid;not null;index"`
	StartDate     string            `json:"startDate"     gorm:"type:date;not null"`
	EndDate       string            `json:"endDate"       gorm:"type:date;not null"`
	DailyPrice    float64           `json:"dailyPrice"    gorm:"not null;type:numeric(10,2)"`
	TotalDays     int               `json:"totalDays"     gorm:"not null;default:1"`
	FreeDaysUsed  int               `json:"freeDaysUsed"  gorm:"not null;default:0"`
	TotalAmount   float64           `json:"totalAmount"   gorm:"not null;type:numeric(10,2)"`
	DiscountAmount float64          `json:"discountAmount" gorm:"not null;default:0;type:numeric(10,2)"`
	FinalAmount   float64           `json:"finalAmount"   gorm:"not null;default:0;type:numeric(10,2)"`
	VoucherID     *uuid.UUID        `json:"voucherId,omitempty" gorm:"type:uuid"`
	Status        DailyRentalStatus `json:"status"        gorm:"not null;default:'active';size:20"`
	Notes         string            `json:"notes,omitempty" gorm:"type:text"`
	ReturnedAt    *time.Time        `json:"returnedAt,omitempty"`
	CreatedAt     time.Time         `json:"createdAt"     gorm:"autoCreateTime"`
	UpdatedAt     time.Time         `json:"updatedAt"     gorm:"autoUpdateTime"`

	Console  *Console  `json:"console,omitempty"  gorm:"foreignKey:ConsoleID"`
	Customer *Customer `json:"customer,omitempty" gorm:"foreignKey:CustomerID"`
}

func (DailyRental) TableName() string { return "daily_rentals" }
