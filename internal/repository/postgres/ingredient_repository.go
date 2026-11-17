package postgres

import (
	"context"
	"strings"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"
	"byone-arena/pkg/spname"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ingredientRepository struct {
	db *gorm.DB
}

// NewIngredientRepository membuat instance baru IngredientRepository
func NewIngredientRepository(db *gorm.DB) repository.IngredientRepository {
	return &ingredientRepository{db: db}
}

func (r *ingredientRepository) FindAll(ctx context.Context) ([]*entity.Ingredient, error) {
	var items []*entity.Ingredient
	if err := r.db.WithContext(ctx).Order("name ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	r.fillStock(ctx, items)
	return items, nil
}

func (r *ingredientRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.Ingredient, error) {
	var item entity.Ingredient
	if err := r.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	r.fillStock(ctx, []*entity.Ingredient{&item})
	return &item, nil
}

func (r *ingredientRepository) Create(ctx context.Context, ingredient *entity.Ingredient) error {
	return r.db.WithContext(ctx).Create(ingredient).Error
}

func (r *ingredientRepository) Update(ctx context.Context, ingredient *entity.Ingredient) error {
	return r.db.WithContext(ctx).Save(ingredient).Error
}

func (r *ingredientRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&entity.Ingredient{}, "id = ?", id).Error
}

func (r *ingredientRepository) AddStockBatch(ctx context.Context, ingredientID uuid.UUID, batch *entity.IngredientStockBatch) error {
	batch.IngredientID = ingredientID
	return r.db.WithContext(ctx).Create(batch).Error
}

func (r *ingredientRepository) ListStockBatches(ctx context.Context, ingredientID uuid.UUID) ([]*entity.IngredientStockBatch, error) {
	var batches []*entity.IngredientStockBatch
	err := r.db.WithContext(ctx).
		Where("ingredient_id = ?", ingredientID).
		Order("expires_at ASC, added_at ASC").
		Find(&batches).Error
	return batches, err
}

func (r *ingredientRepository) SetMenuItemIngredients(ctx context.Context, menuItemID uuid.UUID, items []repository.IngredientInput) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("menu_item_id = ?", menuItemID).Delete(&entity.MenuItemIngredient{}).Error; err != nil {
			return err
		}
		for _, it := range items {
			row := &entity.MenuItemIngredient{
				MenuItemID:   menuItemID,
				IngredientID: it.IngredientID,
				Quantity:     it.Quantity,
			}
			if err := tx.Create(row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *ingredientRepository) ListMenuItemIngredients(ctx context.Context, menuItemID uuid.UUID) ([]*entity.MenuItemIngredient, error) {
	var rows []*entity.MenuItemIngredient
	err := r.db.WithContext(ctx).
		Preload("Ingredient").
		Where("menu_item_id = ?", menuItemID).
		Order("created_at ASC").
		Find(&rows).Error
	return rows, err
}

func (r *ingredientRepository) SetIngredientComponents(ctx context.Context, ingredientID uuid.UUID, items []repository.IngredientInput) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("ingredient_id = ?", ingredientID).Delete(&entity.IngredientRecipe{}).Error; err != nil {
			return err
		}
		for _, it := range items {
			row := &entity.IngredientRecipe{
				IngredientID:          ingredientID,
				ComponentIngredientID: it.IngredientID,
				Quantity:              it.Quantity,
			}
			if err := tx.Create(row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *ingredientRepository) ListIngredientComponents(ctx context.Context, ingredientID uuid.UUID) ([]*entity.IngredientRecipe, error) {
	var rows []*entity.IngredientRecipe
	err := r.db.WithContext(ctx).
		Preload("ComponentIngredient").
		Where("ingredient_id = ?", ingredientID).
		Order("created_at ASC").
		Find(&rows).Error
	return rows, err
}

func (r *ingredientRepository) FindExpiredIngredients(ctx context.Context, days int) ([]repository.ExpiredStockItem, error) {
	var rows []repository.ExpiredStockItem
	err := r.db.WithContext(ctx).Raw(
		"SELECT * FROM "+spname.Ident("ExpiredIngredients")+"(?)", days,
	).Scan(&rows).Error
	return rows, err
}

func (r *ingredientRepository) GetStockLog(ctx context.Context, startDate, endDate string) ([]repository.StockLogEntry, error) {
	var rows []repository.StockLogEntry
	err := r.db.WithContext(ctx).Raw(
		"SELECT * FROM "+spname.Ident("StockLog")+"(?::DATE, ?::DATE)",
		startDate, endDate,
	).Scan(&rows).Error
	return rows, err
}

// fillStock mengisi field Stock (usable) dari batch yang belum kadaluarsa.
func (r *ingredientRepository) fillStock(ctx context.Context, items []*entity.Ingredient) {
	if len(items) == 0 {
		return
	}
	ids := make([]uuid.UUID, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	type stockRow struct {
		IngredientID  uuid.UUID  `gorm:"column:ingredient_id"`
		Usable        float64    `gorm:"column:usable"`
		NearestExpiry *time.Time `gorm:"column:nearest_expiry"`
	}
	var rows []stockRow
	idStrs := make([]string, len(ids))
	for i, id := range ids {
		idStrs[i] = id.String()
	}
	r.db.WithContext(ctx).Raw(
		"SELECT * FROM "+spname.Ident("IngredientUsableStock")+"(?)",
		strings.Join(idStrs, ","),
	).Scan(&rows)

	stockMap := make(map[uuid.UUID]float64, len(rows))
	expiryMap := make(map[uuid.UUID]*time.Time, len(rows))
	for _, row := range rows {
		stockMap[row.IngredientID] = row.Usable
		expiryMap[row.IngredientID] = row.NearestExpiry
	}
	for _, it := range items {
		it.Stock = stockMap[it.ID]
		it.NearestExpiry = expiryMap[it.ID]
	}
}
