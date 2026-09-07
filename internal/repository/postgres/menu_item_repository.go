package postgres

import (
	"context"
	"math"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type menuItemRepository struct {
	db *gorm.DB
}

// NewMenuItemRepository membuat instance baru MenuItemRepository
func NewMenuItemRepository(db *gorm.DB) repository.MenuItemRepository {
	return &menuItemRepository{db: db}
}

func (r *menuItemRepository) FindAll(ctx context.Context) ([]*entity.MenuItem, error) {
	var items []*entity.MenuItem
	if err := r.db.WithContext(ctx).Order("category ASC, name ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	r.fillStock(ctx, items)
	return items, nil
}

func (r *menuItemRepository) FindAvailable(ctx context.Context) ([]*entity.MenuItem, error) {
	var items []*entity.MenuItem
	if err := r.db.WithContext(ctx).
		Where("is_available = TRUE").
		Order("category ASC, name ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	r.fillStock(ctx, items)
	return items, nil
}

func (r *menuItemRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.MenuItem, error) {
	var item entity.MenuItem
	if err := r.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	r.fillStock(ctx, []*entity.MenuItem{&item})
	return &item, nil
}

func (r *menuItemRepository) FindByCategory(ctx context.Context, category entity.MenuItemCategory) ([]*entity.MenuItem, error) {
	var items []*entity.MenuItem
	if err := r.db.WithContext(ctx).
		Where("category = ? AND is_available = TRUE", category).
		Order("name ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	r.fillStock(ctx, items)
	return items, nil
}

// fillStock mengisi field Stock (jumlah porsi yang bisa dibuat) dari resep bahan.
// Stok menu = min(floor(stok_bahan_efektif / qty_resep)) untuk tiap bahan resep.
func (r *menuItemRepository) fillStock(ctx context.Context, items []*entity.MenuItem) {
	if len(items) == 0 {
		return
	}
	menuIDs := make([]uuid.UUID, len(items))
	for i, it := range items {
		menuIDs[i] = it.ID
	}

	// Resep menu -> bahan
	var recipes []entity.MenuItemIngredient
	r.db.WithContext(ctx).Where("menu_item_id IN ?", menuIDs).Find(&recipes)
	menuRecipes := make(map[uuid.UUID][]entity.MenuItemIngredient)
	for _, rcp := range recipes {
		menuRecipes[rcp.MenuItemID] = append(menuRecipes[rcp.MenuItemID], rcp)
	}

	// Stok usable semua bahan
	type ingStock struct {
		IngredientID uuid.UUID  `gorm:"column:ingredient_id"`
		Usable       float64    `gorm:"column:usable"`
		NearestExpiry *time.Time `gorm:"column:nearest_expiry"`
	}
	var stocks []ingStock
	r.db.WithContext(ctx).Raw(`
		SELECT ingredient_id, COALESCE(SUM(remaining),0)::numeric AS usable,
		       MIN(expires_at) AS nearest_expiry
		FROM ingredient_stock_batches
		WHERE remaining > 0 AND expires_at >= CURRENT_DATE
		GROUP BY ingredient_id`).Scan(&stocks)
	batchStock := make(map[uuid.UUID]float64, len(stocks))
	batchExpiry := make(map[uuid.UUID]*time.Time, len(stocks))
	for _, s := range stocks {
		batchStock[s.IngredientID] = s.Usable
		batchExpiry[s.IngredientID] = s.NearestExpiry
	}

	// Sub-resep bahan -> bahan
	var subRecipes []entity.IngredientRecipe
	r.db.WithContext(ctx).Find(&subRecipes)
	subMap := make(map[uuid.UUID][]entity.IngredientRecipe)
	for _, sr := range subRecipes {
		subMap[sr.IngredientID] = append(subMap[sr.IngredientID], sr)
	}

	// Nama bahan (untuk menampilkan bahan pembatas stok)
	type ingName struct {
		ID   uuid.UUID `gorm:"column:id"`
		Name string    `gorm:"column:name"`
	}
	var ingNames []ingName
	r.db.WithContext(ctx).Raw(`SELECT id, name FROM ingredients`).Scan(&ingNames)
	nameMap := make(map[uuid.UUID]string, len(ingNames))
	for _, n := range ingNames {
		nameMap[n.ID] = n.Name
	}

	// Stok efektif bahan (memperhitungkan sub-resep, dengan guard cycle).
	// Sekaligus mencatat bahan mana yang membatasi (nilai minimum).
	type effResult struct {
		stock float64
		limit uuid.UUID
	}
	memo := make(map[uuid.UUID]effResult)
	visiting := make(map[uuid.UUID]bool)
	var effective func(id uuid.UUID, depth int) effResult
	effective = func(id uuid.UUID, depth int) effResult {
		if depth > 20 {
			return effResult{limit: id}
		}
		if v, ok := memo[id]; ok {
			return v
		}
		if visiting[id] {
			return effResult{limit: id} // cycle
		}
		visiting[id] = true
		own := batchStock[id]
		subs := subMap[id]
		res := effResult{stock: own, limit: id}
		if len(subs) > 0 {
			minDerivable := math.Inf(1)
			var limitID uuid.UUID
			for _, sub := range subs {
				if sub.Quantity <= 0 {
					continue
				}
				er := effective(sub.ComponentIngredientID, depth+1)
				derivable := math.Floor(er.stock / sub.Quantity)
				if derivable < minDerivable {
					minDerivable = derivable
					limitID = er.limit
				}
			}
			if math.IsInf(minDerivable, 1) {
				minDerivable = 0
				limitID = id
			}
			if minDerivable < res.stock {
				res.stock = minDerivable
				res.limit = limitID
			}
		}
		delete(visiting, id)
		memo[id] = res
		return res
	}

	// Kadaluarsa efektif bahan: tanggal paling awal dari batch usable sendiri
	// atau dari komponen sub-resepnya (dengan guard cycle).
	expiryMemo := make(map[uuid.UUID]*time.Time)
	expiryVisiting := make(map[uuid.UUID]bool)
	var effectiveExpiry func(id uuid.UUID, depth int) *time.Time
	effectiveExpiry = func(id uuid.UUID, depth int) *time.Time {
		if depth > 20 {
			return nil
		}
		if v, ok := expiryMemo[id]; ok {
			return v
		}
		if expiryVisiting[id] {
			return nil // cycle
		}
		expiryVisiting[id] = true
		best := batchExpiry[id]
		for _, sub := range subMap[id] {
			if sub.Quantity <= 0 {
				continue
			}
			c := effectiveExpiry(sub.ComponentIngredientID, depth+1)
			if c != nil && (best == nil || c.Before(*best)) {
				best = c
			}
		}
		delete(expiryVisiting, id)
		expiryMemo[id] = best
		return best
	}

	// Hitung stok, bahan pembatas, & kadaluarsa tiap menu
	for _, it := range items {
		rcps := menuRecipes[it.ID]
		if len(rcps) == 0 {
			it.Stock = 0
			it.ExpiresAt = nil
			it.LimitingIngredient = ""
			it.LimitingIngredientStock = 0
			continue
		}
		min := math.Inf(1)
		var expiry *time.Time
		var limitingID uuid.UUID
		for _, rcp := range rcps {
			if rcp.Quantity <= 0 {
				continue
			}
			er := effective(rcp.IngredientID, 0)
			derivable := math.Floor(er.stock / rcp.Quantity)
			if derivable < min {
				min = derivable
				limitingID = er.limit
			}
			e := effectiveExpiry(rcp.IngredientID, 0)
			if e != nil && (expiry == nil || e.Before(*expiry)) {
				expiry = e
			}
		}
		if math.IsInf(min, 1) {
			min = 0
		}
		it.Stock = min
		it.ExpiresAt = expiry
		if min > 0 && limitingID != uuid.Nil {
			it.LimitingIngredient = nameMap[limitingID]
			it.LimitingIngredientStock = batchStock[limitingID]
		} else {
			it.LimitingIngredient = ""
			it.LimitingIngredientStock = 0
		}
	}
}

func (r *menuItemRepository) Create(ctx context.Context, item *entity.MenuItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *menuItemRepository) Update(ctx context.Context, item *entity.MenuItem) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *menuItemRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&entity.MenuItem{}, "id = ?", id).Error
}
