package usecase

import (
	"context"
	"errors"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
)

// IngredientUseCase mendefinisikan logika bisnis bahan + resep + batch stok.
type IngredientUseCase interface {
	GetAllIngredients(ctx context.Context) ([]*entity.Ingredient, error)
	GetIngredientByID(ctx context.Context, id uuid.UUID) (*entity.Ingredient, error)
	CreateIngredient(ctx context.Context, req *CreateIngredientRequest) (*entity.Ingredient, error)
	UpdateIngredient(ctx context.Context, id uuid.UUID, req *UpdateIngredientRequest) (*entity.Ingredient, error)
	DeleteIngredient(ctx context.Context, id uuid.UUID) error

	AddIngredientStock(ctx context.Context, id uuid.UUID, req *AddStockRequest) (*entity.IngredientStockBatch, error)
	ListIngredientBatches(ctx context.Context, id uuid.UUID) ([]*entity.IngredientStockBatch, error)

	SetMenuItemIngredients(ctx context.Context, menuItemID uuid.UUID, req *SetRecipeRequest) error
	ListMenuItemIngredients(ctx context.Context, menuItemID uuid.UUID) ([]*entity.MenuItemIngredient, error)

	SetIngredientComponents(ctx context.Context, ingredientID uuid.UUID, req *SetRecipeRequest) error
	ListIngredientComponents(ctx context.Context, ingredientID uuid.UUID) ([]*entity.IngredientRecipe, error)

	GetExpired(ctx context.Context, days int) (*ExpiredStockResponse, error)
	GetStockLog(ctx context.Context, startDate, endDate string) ([]repository.StockLogEntry, error)
}

type ingredientUseCase struct {
	ingredientRepo repository.IngredientRepository
}

// NewIngredientUseCase membuat instance baru IngredientUseCase
func NewIngredientUseCase(ingredientRepo repository.IngredientRepository) IngredientUseCase {
	return &ingredientUseCase{ingredientRepo: ingredientRepo}
}

type CreateIngredientRequest struct {
	Name        string  `json:"name"        validate:"required,min=2,max=150"`
	Unit        string  `json:"unit"        validate:"omitempty,max=20"`
	Category    string  `json:"category"    validate:"omitempty,oneof=food drink spice"`
	CostPerUnit float64 `json:"costPerUnit" validate:"gte=0"`
}

type UpdateIngredientRequest struct {
	Name        string   `json:"name"        validate:"omitempty,min=2,max=150"`
	Unit        string   `json:"unit"        validate:"omitempty,max=20"`
	Category    string   `json:"category"    validate:"omitempty,oneof=food drink spice"`
	CostPerUnit *float64 `json:"costPerUnit" validate:"omitempty,gte=0"`
}

// AddStockRequest payload penambahan batch stok (qty + tanggal kadaluarsa).
type AddStockRequest struct {
	Quantity  float64 `json:"quantity"  validate:"required,gt=0"`
	ExpiresAt string  `json:"expiresAt" validate:"required"` // format YYYY-MM-DD
	Notes     string  `json:"notes"`
}

// RecipeItemInput satu baris resep.
type RecipeItemInput struct {
	IngredientID uuid.UUID `json:"ingredientId" validate:"required"`
	Quantity     float64   `json:"quantity"     validate:"required,gt=0"`
}

// SetRecipeRequest payload untuk mengganti daftar resep.
type SetRecipeRequest struct {
	Items []RecipeItemInput `json:"items" validate:"dive"`
}

// ExpiredStockResponse daftar item kadaluarsa/hampir kadaluarsa.
type ExpiredStockResponse struct {
	Ingredients []repository.ExpiredStockItem `json:"ingredients"`
}

func parseExpiry(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, errors.New("expiresAt harus berformat YYYY-MM-DD")
	}
	return t, nil
}

func (uc *ingredientUseCase) GetAllIngredients(ctx context.Context) ([]*entity.Ingredient, error) {
	return uc.ingredientRepo.FindAll(ctx)
}

func (uc *ingredientUseCase) GetIngredientByID(ctx context.Context, id uuid.UUID) (*entity.Ingredient, error) {
	item, err := uc.ingredientRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.New("bahan tidak ditemukan")
	}
	return item, nil
}

func (uc *ingredientUseCase) CreateIngredient(ctx context.Context, req *CreateIngredientRequest) (*entity.Ingredient, error) {
	unit := req.Unit
	if unit == "" {
		unit = "pcs"
	}
	category := req.Category
	if category == "" {
		category = "food"
	}
	item := &entity.Ingredient{
		Name:        req.Name,
		Unit:        unit,
		Category:    category,
		CostPerUnit: req.CostPerUnit,
	}
	if err := uc.ingredientRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (uc *ingredientUseCase) UpdateIngredient(ctx context.Context, id uuid.UUID, req *UpdateIngredientRequest) (*entity.Ingredient, error) {
	item, err := uc.ingredientRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.New("bahan tidak ditemukan")
	}
	if req.Name != "" {
		item.Name = req.Name
	}
	if req.Unit != "" {
		item.Unit = req.Unit
	}
	if req.Category != "" {
		item.Category = req.Category
	}
	if req.CostPerUnit != nil {
		item.CostPerUnit = *req.CostPerUnit
	}
	if err := uc.ingredientRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (uc *ingredientUseCase) DeleteIngredient(ctx context.Context, id uuid.UUID) error {
	item, err := uc.ingredientRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if item == nil {
		return errors.New("bahan tidak ditemukan")
	}
	return uc.ingredientRepo.Delete(ctx, id)
}

func (uc *ingredientUseCase) AddIngredientStock(ctx context.Context, id uuid.UUID, req *AddStockRequest) (*entity.IngredientStockBatch, error) {
	if _, err := uc.GetIngredientByID(ctx, id); err != nil {
		return nil, err
	}
	expiry, err := parseExpiry(req.ExpiresAt)
	if err != nil {
		return nil, err
	}
	batch := &entity.IngredientStockBatch{
		Quantity:  req.Quantity,
		Remaining: req.Quantity,
		ExpiresAt: expiry,
		Notes:     req.Notes,
	}
	if err := uc.ingredientRepo.AddStockBatch(ctx, id, batch); err != nil {
		return nil, err
	}
	return batch, nil
}

func (uc *ingredientUseCase) ListIngredientBatches(ctx context.Context, id uuid.UUID) ([]*entity.IngredientStockBatch, error) {
	if _, err := uc.GetIngredientByID(ctx, id); err != nil {
		return nil, err
	}
	return uc.ingredientRepo.ListStockBatches(ctx, id)
}

func (uc *ingredientUseCase) SetMenuItemIngredients(ctx context.Context, menuItemID uuid.UUID, req *SetRecipeRequest) error {
	items := make([]repository.IngredientInput, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, repository.IngredientInput{
			IngredientID: it.IngredientID,
			Quantity:     it.Quantity,
		})
	}
	return uc.ingredientRepo.SetMenuItemIngredients(ctx, menuItemID, items)
}

func (uc *ingredientUseCase) ListMenuItemIngredients(ctx context.Context, menuItemID uuid.UUID) ([]*entity.MenuItemIngredient, error) {
	return uc.ingredientRepo.ListMenuItemIngredients(ctx, menuItemID)
}

func (uc *ingredientUseCase) SetIngredientComponents(ctx context.Context, ingredientID uuid.UUID, req *SetRecipeRequest) error {
	if _, err := uc.GetIngredientByID(ctx, ingredientID); err != nil {
		return err
	}
	items := make([]repository.IngredientInput, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, repository.IngredientInput{
			IngredientID: it.IngredientID,
			Quantity:     it.Quantity,
		})
	}
	return uc.ingredientRepo.SetIngredientComponents(ctx, ingredientID, items)
}

func (uc *ingredientUseCase) ListIngredientComponents(ctx context.Context, ingredientID uuid.UUID) ([]*entity.IngredientRecipe, error) {
	return uc.ingredientRepo.ListIngredientComponents(ctx, ingredientID)
}

func (uc *ingredientUseCase) GetExpired(ctx context.Context, days int) (*ExpiredStockResponse, error) {
	if days < 0 {
		days = 0
	}
	if days > 365 {
		days = 365
	}
	ingredients, err := uc.ingredientRepo.FindExpiredIngredients(ctx, days)
	if err != nil {
		return nil, err
	}
	if ingredients == nil {
		ingredients = []repository.ExpiredStockItem{}
	}
	return &ExpiredStockResponse{Ingredients: ingredients}, nil
}

func (uc *ingredientUseCase) GetStockLog(ctx context.Context, startDate, endDate string) ([]repository.StockLogEntry, error) {
	if startDate == "" {
		startDate = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = time.Now().Format("2006-01-02")
	}
	rows, err := uc.ingredientRepo.GetStockLog(ctx, startDate, endDate)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []repository.StockLogEntry{}
	}
	return rows, nil
}
