package entity

import (
	"time"

	"github.com/google/uuid"
)

// RentalPackage — paket rental fleksibel (mis. paket begadang 21:00-06:00).
type RentalPackage struct {
	ID              uuid.UUID           `json:"id"              gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Name            string              `json:"name"            gorm:"not null;size:150"`
	StartTime       string              `json:"startTime"       gorm:"-"` // "HH:MM"
	EndTime         string              `json:"endTime"         gorm:"-"` // "HH:MM"
	Price           float64             `json:"price"           gorm:"not null;type:numeric(10,2)"`
	DurationMinutes int                 `json:"durationMinutes" gorm:"-"`
	Description     string              `json:"description,omitempty" gorm:"type:text"`
	IsActive        bool                `json:"isActive"        gorm:"not null;default:true"`
	CreatedAt       time.Time           `json:"createdAt"       gorm:"autoCreateTime"`
	UpdatedAt       time.Time           `json:"updatedAt"       gorm:"autoUpdateTime"`
	Items           []RentalPackageItem `json:"items"           gorm:"-"`
}

func (RentalPackage) TableName() string { return "rental_packages" }

// RentalPackageItem — item bundling (menu + qty) dalam paket.
type RentalPackageItem struct {
	MenuItemID   uuid.UUID `json:"menuItemId"`
	MenuItemName string    `json:"menuItemName,omitempty"`
	Quantity     int       `json:"quantity"`
}
