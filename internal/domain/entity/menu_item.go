package entity

import (
	"time"

	"github.com/google/uuid"
)

// MenuItemCategory mendefinisikan kategori menu
type MenuItemCategory string

const (
	MenuCategoryFood  MenuItemCategory = "food"
	MenuCategoryDrink MenuItemCategory = "drink"
	MenuCategorySnack MenuItemCategory = "snack"
	MenuCategoryOther MenuItemCategory = "other"
)

// MenuItem merepresentasikan item makanan/minuman yang tersedia untuk dipesan
type MenuItem struct {
	ID          uuid.UUID        `json:"id"          gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Name        string           `json:"name"        gorm:"not null;size:150"`
	Category    MenuItemCategory `json:"category"    gorm:"not null;size:30;default:'food'"`
	Price       float64          `json:"price"       gorm:"not null;type:numeric(10,2)"`
	Description string           `json:"description,omitempty" gorm:"type:text"`
	IsAvailable bool             `json:"isAvailable" gorm:"not null;default:true"`
	CreatedAt   time.Time        `json:"createdAt"   gorm:"autoCreateTime"`
	UpdatedAt   time.Time        `json:"updatedAt"   gorm:"autoUpdateTime"`
}

func (MenuItem) TableName() string { return "menu_items" }
