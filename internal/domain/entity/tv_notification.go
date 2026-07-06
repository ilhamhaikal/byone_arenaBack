package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// NotificationPriority mendefinisikan prioritas notifikasi
type NotificationPriority string

const (
	NotificationPriorityLow    NotificationPriority = "low"
	NotificationPriorityNormal NotificationPriority = "normal"
	NotificationPriorityHigh   NotificationPriority = "high"
)

// TvNotification merepresentasikan pesan promosi ke TV Android
type TvNotification struct {
	ID                  uuid.UUID            `json:"id"                  gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Title               string               `json:"title"               gorm:"not null;size:100"`
	Message             string               `json:"message"             gorm:"not null;type:text"`
	ImageURL            *string              `json:"imageUrl,omitempty"  gorm:"size:500"`
	Priority            NotificationPriority `json:"priority"            gorm:"not null;default:'normal';size:20"`
	LoopEnabled         bool                 `json:"loopEnabled"         gorm:"not null;default:false"`
	LoopInterval        int                  `json:"loopInterval"        gorm:"not null;default:30"`
	TargetAll           bool                 `json:"targetAll"           gorm:"not null;default:true"`
	TargetConsoleIDs    JSONBArray           `json:"targetConsoleIds"    gorm:"type:jsonb;default:'[]'"`  // array of UUID
	ActiveSessionsOnly  bool                 `json:"activeSessionsOnly"  gorm:"not null;default:false"`   // hanya TV dengan sesi aktif
	TargetConsoleType   *string              `json:"targetConsoleType,omitempty" gorm:"size:15"`
	IsActive            bool                 `json:"isActive"            gorm:"not null;default:true;index"`
	CreatedAt           time.Time            `json:"createdAt"           gorm:"autoCreateTime"`
	UpdatedAt           time.Time            `json:"updatedAt"           gorm:"autoUpdateTime"`

	// Field transient — untuk input/output API
	ConsoleIDs []string `json:"consoleIds,omitempty" gorm:"-"`
}

// JSONBArray untuk scan/save JSONB array di PostgreSQL
type JSONBArray []string

func (j *JSONBArray) Scan(value interface{}) error {
	if value == nil {
		*j = []string{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

func (j JSONBArray) Value() (interface{}, error) {
	if len(j) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(j)
}

func (TvNotification) TableName() string { return "tv_notifications" }
