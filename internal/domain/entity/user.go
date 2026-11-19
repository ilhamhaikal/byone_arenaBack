package entity

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

// UserRole mendefinisikan peran pengguna dalam sistem
type UserRole string

const (
	UserRoleSuperAdmin UserRole = "superadmin"
	UserRoleAdmin      UserRole = "admin"
	UserRoleKasir      UserRole = "kasir"
)

// User merepresentasikan akun pengguna yang mengelola sistem
type User struct {
	ID          uuid.UUID `json:"id"         gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Username    string    `json:"username"   gorm:"uniqueIndex;not null;size:50"`
	Password    string    `json:"-"          gorm:"not null;size:255"`
	FullName    string    `json:"fullName"   gorm:"not null;size:100"`
	Role        UserRole  `json:"role"       gorm:"not null;size:20"`
	IsActive    bool      `json:"isActive"    gorm:"not null;default:true"`
	Permissions string    `json:"permissions" gorm:"type:jsonb;not null;default:'[]'"`
	CreatedAt   time.Time `json:"createdAt"   gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updatedAt"   gorm:"autoUpdateTime"`
}

func (User) TableName() string { return "users" }

// MarshalJSON mengubah kolom permissions (JSONB berupa string di Go) menjadi
// array JSON di response API. Tanpa ini frontend menerima string berisi JSON,
// yang mudah salah diurai oleh client lain.
func (u User) MarshalJSON() ([]byte, error) {
	type Alias User

	permissions := []string{}
	if strings.TrimSpace(u.Permissions) != "" {
		_ = json.Unmarshal([]byte(u.Permissions), &permissions)
	}
	if permissions == nil {
		permissions = []string{}
	}

	return json.Marshal(struct {
		Alias
		Permissions []string `json:"permissions"`
	}{
		Alias:       Alias(u),
		Permissions: permissions,
	})
}
