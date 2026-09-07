package postgres

import (
	"context"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

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
	err := r.db.WithContext(ctx).Raw(`
		SELECT i.id, i.name, i.unit,
		       COALESCE(SUM(b.remaining) FILTER (WHERE b.expires_at < CURRENT_DATE),0)::numeric AS expired_qty,
		       COALESCE(SUM(b.remaining) FILTER (WHERE b.expires_at >= CURRENT_DATE),0)::numeric AS usable_stock,
		       MIN(b.expires_at)::text AS nearest_expiry
		FROM ingredients i
		JOIN ingredient_stock_batches b ON b.ingredient_id = i.id
		WHERE b.remaining > 0
		GROUP BY i.id, i.name, i.unit
		HAVING MIN(b.expires_at) < CURRENT_DATE + (? * INTERVAL '1 day')
		ORDER BY nearest_expiry ASC`, days).Scan(&rows).Error
	return rows, err
}

func (r *ingredientRepository) GetStockLog(ctx context.Context, startDate, endDate string) ([]repository.StockLogEntry, error) {
	var rows []repository.StockLogEntry
	err := r.db.WithContext(ctx).Raw(`
		SELECT 'ingredient'::text AS type, i.name AS item_name, i.unit,
		       b.quantity, b.remaining, b.expires_at::text AS expires_at,
		       b.added_at::date::text AS added_at, COALESCE(b.notes,'') AS notes
		FROM ingredient_stock_batches b
		JOIN ingredients i ON i.id = b.ingredient_id
		WHERE b.added_at::date BETWEEN ?::date AND ?::date
		ORDER BY added_at DESC, item_name ASC`, startDate, endDate).Scan(&rows).Error
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
		IngredientID uuid.UUID  `gorm:"column:ingredient_id"`
		Usable       float64    `gorm:"column:usable"`
		NearestExpiry *time.Time `gorm:"column:nearest_expiry"`
	}
	var rows []stockRow
	r.db.WithContext(ctx).Raw(`
		SELECT ingredient_id,
		       COALESCE(SUM(remaining),0)::numeric AS usable,
		       MIN(expires_at) AS nearest_expiry
		FROM ingredient_stock_batches
		WHERE remaining > 0 AND expires_at >= CURRENT_DATE
		  AND ingredient_id IN ?
		GROUP BY ingredient_id`, ids).Scan(&rows)

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
