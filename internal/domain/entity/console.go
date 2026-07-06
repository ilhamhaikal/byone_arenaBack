package entity

import (
	"time"

	"github.com/google/uuid"
)

// ConsoleType mendefinisikan jenis konsol (nilai referensi, tidak dibatasi di DB)
type ConsoleType string

const (
	ConsoleTypePS3       ConsoleType = "PS3"
	ConsoleTypePS4       ConsoleType = "PS4"
	ConsoleTypePS5       ConsoleType = "PS5"
	ConsoleTypeAndroidTV ConsoleType = "AndroidTV"
	ConsoleTypeSwitch    ConsoleType = "Switch"
)

// ConsoleStatus mendefinisikan status ketersediaan konsol
type ConsoleStatus string

const (
	ConsoleStatusAvailable   ConsoleStatus = "available"  // siap disewa / TV mati
	ConsoleStatusInUse       ConsoleStatus = "in_use"      // sedang aktif / TV menyala
	ConsoleStatusMaintenance ConsoleStatus = "maintenance" // sedang diperbaiki
)

// ScreenStatus mendefinisikan status layar TV
type ScreenStatus string

const (
	ScreenStatusOn         ScreenStatus = "on"
	ScreenStatusOff        ScreenStatus = "off"
	ScreenStatusScreensaver ScreenStatus = "screensaver"
)

// Console merepresentasikan unit konsol / TV Android yang tersedia untuk disewa
type Console struct {
	ID           uuid.UUID     `json:"id"                     gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Name         string        `json:"name"                   gorm:"not null;size:100"`
	ConsoleType  ConsoleType   `json:"consoleType"            gorm:"not null;size:15"`
	IPAddress    *string       `json:"ipAddress,omitempty"    gorm:"size:50"`
	ADBPort      int           `json:"adbPort,omitempty"      gorm:"default:5555"`
	MACAddress   *string       `json:"macAddress,omitempty"   gorm:"size:20"`
	Status       ConsoleStatus `json:"status"                 gorm:"not null;default:'available';size:20"`
	ScreenStatus ScreenStatus  `json:"screenStatus"           gorm:"not null;default:'off';size:20"`
	PricePerHour float64       `json:"pricePerHour"           gorm:"not null;type:numeric(10,2)"`
	Description  string        `json:"description,omitempty" gorm:"type:text"`
	CreatedAt    time.Time     `json:"createdAt"              gorm:"autoCreateTime"`
	UpdatedAt    time.Time     `json:"updatedAt"              gorm:"autoUpdateTime"`
}

func (Console) TableName() string { return "consoles" }

// IsAvailable mengecek apakah konsol dapat disewa
func (c *Console) IsAvailable() bool {
	return c.Status == ConsoleStatusAvailable
}
