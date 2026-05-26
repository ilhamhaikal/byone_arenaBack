package entity

import (
	"time"

	"github.com/google/uuid"
)

// ConsoleType mendefinisikan jenis konsol PS
type ConsoleType string

const (
	ConsoleTypePS3 ConsoleType = "PS3"
	ConsoleTypePS4 ConsoleType = "PS4"
	ConsoleTypePS5 ConsoleType = "PS5"
)

// ConsoleStatus mendefinisikan status ketersediaan konsol
type ConsoleStatus string

const (
	ConsoleStatusAvailable   ConsoleStatus = "available"
	ConsoleStatusInUse       ConsoleStatus = "in_use"
	ConsoleStatusMaintenance ConsoleStatus = "maintenance"
)

// Console merepresentasikan unit konsol PlayStation yang tersedia untuk disewa
type Console struct {
	ID           uuid.UUID     `json:"id"           gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Name         string        `json:"name"         gorm:"not null;size:100"`
	ConsoleType  ConsoleType   `json:"consoleType"  gorm:"not null;size:10"`
	Status       ConsoleStatus `json:"status"       gorm:"not null;default:'available';size:20"`
	PricePerHour float64       `json:"pricePerHour" gorm:"not null;type:numeric(10,2)"`
	Description  string        `json:"description,omitempty" gorm:"type:text"`
	CreatedAt    time.Time     `json:"createdAt"    gorm:"autoCreateTime"`
	UpdatedAt    time.Time     `json:"updatedAt"    gorm:"autoUpdateTime"`
}

func (Console) TableName() string { return "consoles" }

// IsAvailable mengecek apakah konsol dapat disewa
func (c *Console) IsAvailable() bool {
	return c.Status == ConsoleStatusAvailable
}
