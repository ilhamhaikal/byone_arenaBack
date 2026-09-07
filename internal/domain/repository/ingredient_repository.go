package repository

import (
	"context"

	"byone-arena/internal/domain/entity"

	"github.com/google/uuid"
)

// IngredientInput — satu baris resep (bahan + kuantitas per 1 porsi/unit).
type IngredientInput struct {
	IngredientID uuid.UUID
	Quantity     float64
}

// ExpiredStockItem — ringkasan item (bahan/menu) yang kadaluarsa/hampir kadaluarsa.
type ExpiredStockItem struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Unit          string    `json:"unit,omitempty"`
	ExpiredQty    float64   `json:"expiredQty"`
	UsableStock   float64   `json:"usableStock"`
	NearestExpiry string    `json:"nearestExpiry"` // format YYYY-MM-DD
}

// StockLogEntry — satu baris log penambahan stok (bahan atau menu).
type StockLogEntry struct {
	Type      string  `json:"type"` // "ingredient" | "menu"
	ItemName  string  `json:"itemName"`
	Unit      string  `json:"unit,omitempty"`
	Quantity  float64 `json:"quantity"`
	Remaining float64 `json:"remaining"`
	ExpiresAt string  `json:"expiresAt"` // format YYYY-MM-DD
	AddedAt   string  `json:"addedAt"`   // format YYYY-MM-DD
	Notes     string  `json:"notes,omitempty"`
}

// IngredientRepository mendefinisikan kontrak akses data bahan + resep + batch stok.
type IngredientRepository interface {
	// Bahan
	FindAll(ctx context.Context) ([]*entity.Ingredient, error)
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Ingredient, error)
	Create(ctx context.Context, ingredient *entity.Ingredient) error
	Update(ctx context.Context, ingredient *entity.Ingredient) error
	Delete(ctx context.Context, id uuid.UUID) error

	// Batch stok bahan (log penambahan)
	AddStockBatch(ctx context.Context, ingredientID uuid.UUID, batch *entity.IngredientStockBatch) error
	ListStockBatches(ctx context.Context, ingredientID uuid.UUID) ([]*entity.IngredientStockBatch, error)

	// Resep menu -> bahan
	SetMenuItemIngredients(ctx context.Context, menuItemID uuid.UUID, items []IngredientInput) error
	ListMenuItemIngredients(ctx context.Context, menuItemID uuid.UUID) ([]*entity.MenuItemIngredient, error)

	// Sub-resep bahan -> bahan
	SetIngredientComponents(ctx context.Context, ingredientID uuid.UUID, items []IngredientInput) error
	ListIngredientComponents(ctx context.Context, ingredientID uuid.UUID) ([]*entity.IngredientRecipe, error)

	// Kadaluarsa / hampir kadaluarsa (days = N hari ke depan)
	FindExpiredIngredients(ctx context.Context, days int) ([]ExpiredStockItem, error)

	// Log penambahan stok terpusat (bahan), filter rentang tanggal.
	GetStockLog(ctx context.Context, startDate, endDate string) ([]StockLogEntry, error)
}
