package entity

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
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
	Stock       float64          `json:"stock"       gorm:"-"`                                     // usable stock dari batch (bukan kolom DB)
	ExpiresAt   *time.Time       `json:"expiresAt,omitempty" gorm:"-"`                             // kadaluarsa terdekat dari resep (bukan kolom DB)
	LimitingIngredient      string  `json:"limitingIngredient,omitempty" gorm:"-"`      // nama bahan pembatas stok (yang nilainya paling kecil)
	LimitingIngredientStock float64 `json:"limitingIngredientStock,omitempty" gorm:"-"` // stok usable bahan pembatas
	HPP         float64          `json:"hpp"         gorm:"not null;default:0;type:numeric(10,2)"` // harga pokok per pcs
	CreatedAt   time.Time        `json:"createdAt"   gorm:"autoCreateTime"`
	UpdatedAt   time.Time        `json:"updatedAt"   gorm:"autoUpdateTime"`
	DeletedAt   gorm.DeletedAt   `json:"-"          gorm:"index"` // soft delete: simpan riwayat pesanan
}

func (MenuItem) TableName() string { return "menu_items" }
