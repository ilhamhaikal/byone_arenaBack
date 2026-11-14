package entity

import (
	"time"

	"github.com/google/uuid"
)

// AuditLog mencatat aksi sensitif (uang, harga, konfigurasi) beserta pelakunya.
// Tanpa tabel ini, selisih kas tidak dapat ditelusuri ke operator tertentu.
type AuditLog struct {
	ID        uuid.UUID      `json:"id"        gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID    *uuid.UUID     `json:"userId"    gorm:"type:uuid;index"`
	Username  string         `json:"username"  gorm:"size:50;not null;default:''"`
	Role      string         `json:"role"      gorm:"size:20;not null;default:''"`
	Action    string         `json:"action"    gorm:"size:50;not null;index"`
	Entity    string         `json:"entity"    gorm:"size:50;not null;default:''"`
	EntityID  *uuid.UUID     `json:"entityId"  gorm:"type:uuid"`
	Detail    map[string]any `json:"detail"    gorm:"type:jsonb;serializer:json;not null;default:'{}'"`
	IP        string         `json:"ip"        gorm:"size:50;not null;default:''"`
	CreatedAt time.Time      `json:"createdAt" gorm:"autoCreateTime;index"`
}

func (AuditLog) TableName() string { return "audit_logs" }
