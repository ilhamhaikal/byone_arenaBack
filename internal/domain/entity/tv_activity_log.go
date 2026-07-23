package entity

import (
	"time"

	"github.com/google/uuid"
)

// TvActivityEvent — tipe event TV
type TvActivityEvent string

const (
	TvEventOn  TvActivityEvent = "on"
	TvEventOff TvActivityEvent = "off"
)

// TvActivityLog — log aktivitas power TV (nyala/mati)
type TvActivityLog struct {
	ID              uuid.UUID       `json:"id"              gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	ConsoleID       uuid.UUID       `json:"consoleId"       gorm:"type:uuid;not null;index"`
	Event           TvActivityEvent `json:"event"           gorm:"not null;size:10"`
	SessionID       *uuid.UUID      `json:"sessionId"       gorm:"type:uuid"`                    // sesi aktif saat event, null = unauthorized
	IsAuthorized    bool            `json:"isAuthorized"    gorm:"not null;default:false"`        // true = TV nyala sesuai sesi
	DurationMinutes *int            `json:"durationMinutes" gorm:"column:duration_minutes"`       // dihitung saat TV mati (null saat on)
	CreatedAt       time.Time       `json:"createdAt"       gorm:"autoCreateTime"`
}

func (TvActivityLog) TableName() string { return "tv_activity_logs" }
