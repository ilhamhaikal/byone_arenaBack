package entity

import (
	"time"

	"github.com/google/uuid"
)

// AuthSession mencatat sesi login user. jti berasal dari klaim JWT sehingga
// logout dapat menutup sesi yang tepat.
type AuthSession struct {
	ID        uuid.UUID  `json:"id"         gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID    uuid.UUID  `json:"userId"     gorm:"type:uuid;not null;index"`
	JTI       string     `json:"-"          gorm:"size:64;not null;uniqueIndex"`
	LoginAt   time.Time  `json:"loginAt"    gorm:"not null;default:now()"`
	LogoutAt  *time.Time `json:"logoutAt,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	IP        string     `json:"ip"         gorm:"size:50;not null;default:''"`
	UserAgent string     `json:"userAgent"  gorm:"size:255;not null;default:''"`
	CreatedAt time.Time  `json:"createdAt"  gorm:"autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt"  gorm:"autoUpdateTime"`
}

func (AuthSession) TableName() string { return "auth_sessions" }
