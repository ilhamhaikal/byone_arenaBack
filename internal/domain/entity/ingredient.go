package entity

import (
	"time"

	"github.com/google/uuid"
)

// Ingredient merepresentasikan bahan baku (telur, beras, dll) — bukan dijual,
// hanya stok internal dengan batch + tanggal kadaluarsa.
type Ingredient struct {
	ID            uuid.UUID `json:"id"            gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Name          string    `json:"name"          gorm:"not null;size:150"`
	Unit          string    `json:"unit"          gorm:"not null;size:20;default:'pcs'"`
	Stock         float64   `json:"stock"         gorm:"-"` // usable stock dari batch (bukan kolom DB)
	NearestExpiry *time.Time `json:"nearestExpiry" gorm:"-"` // kadaluarsa terdekat batch usable
	CreatedAt     time.Time `json:"createdAt"     gorm:"autoCreateTime"`
	UpdatedAt     time.Time `json:"updatedAt"     gorm:"autoUpdateTime"`
}

func (Ingredient) TableName() string { return "ingredients" }

// IngredientStockBatch — satu batch penambahan stok bahan (log + kadaluarsa).
type IngredientStockBatch struct {
	ID           uuid.UUID `json:"id"           gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	IngredientID uuid.UUID `json:"ingredientId" gorm:"type:uuid;not null;index"`
	Quantity     float64   `json:"quantity"     gorm:"not null;type:numeric(10,3)"`
	Remaining    float64   `json:"remaining"    gorm:"not null;default:0;type:numeric(10,3)"`
	ExpiresAt    time.Time `json:"expiresAt"    gorm:"not null;type:date"`
	AddedAt      time.Time `json:"addedAt"      gorm:"autoCreateTime"`
	Notes        string    `json:"notes,omitempty" gorm:"type:text"`
}

func (IngredientStockBatch) TableName() string { return "ingredient_stock_batches" }

// MenuItemIngredient — resep menu -> bahan (BOM langsung).
type MenuItemIngredient struct {
	ID           uuid.UUID `json:"id"           gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	MenuItemID   uuid.UUID `json:"menuItemId"   gorm:"type:uuid;not null;index"`
	IngredientID uuid.UUID `json:"ingredientId" gorm:"type:uuid;not null;index"`
	Quantity     float64   `json:"quantity"     gorm:"not null;type:numeric(10,3)"`
	CreatedAt    time.Time `json:"createdAt"    gorm:"autoCreateTime"`

	Ingredient *Ingredient `json:"ingredient,omitempty" gorm:"foreignKey:IngredientID"`
}

func (MenuItemIngredient) TableName() string { return "menu_item_ingredients" }

// IngredientRecipe — sub-resep bahan -> bahan (BOM bertingkat).
type IngredientRecipe struct {
	ID                    uuid.UUID `json:"id"                    gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	IngredientID          uuid.UUID `json:"ingredientId"          gorm:"type:uuid;not null;index"`
	ComponentIngredientID uuid.UUID `json:"componentIngredientId" gorm:"type:uuid;not null;index"`
	Quantity              float64   `json:"quantity"              gorm:"not null;type:numeric(10,3)"`
	CreatedAt             time.Time `json:"createdAt"             gorm:"autoCreateTime"`

	ComponentIngredient *Ingredient `json:"componentIngredient,omitempty" gorm:"foreignKey:ComponentIngredientID"`
}

func (IngredientRecipe) TableName() string { return "ingredient_recipes" }
