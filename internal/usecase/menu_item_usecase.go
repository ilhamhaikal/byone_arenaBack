package usecase

import (
	"context"
	"errors"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
)

// MenuItemUseCase mendefinisikan logika bisnis untuk manajemen menu makanan
type MenuItemUseCase interface {
	GetAllMenuItems(ctx context.Context) ([]*entity.MenuItem, error)
	GetAvailableMenuItems(ctx context.Context) ([]*entity.MenuItem, error)
	GetMenuItemsByCategory(ctx context.Context, category string) ([]*entity.MenuItem, error)
	GetMenuItemByID(ctx context.Context, id uuid.UUID) (*entity.MenuItem, error)
	CreateMenuItem(ctx context.Context, req *CreateMenuItemRequest) (*entity.MenuItem, error)
	UpdateMenuItem(ctx context.Context, id uuid.UUID, req *UpdateMenuItemRequest) (*entity.MenuItem, error)
	DeleteMenuItem(ctx context.Context, id uuid.UUID) error
	ToggleMenuItem(ctx context.Context, id uuid.UUID) (*entity.MenuItem, error)
}

type menuItemUseCase struct {
	menuRepo repository.MenuItemRepository
}

// NewMenuItemUseCase membuat instance baru MenuItemUseCase
func NewMenuItemUseCase(menuRepo repository.MenuItemRepository) MenuItemUseCase {
	return &menuItemUseCase{menuRepo: menuRepo}
}

// CreateMenuItemRequest payload untuk membuat menu baru
type CreateMenuItemRequest struct {
	Name        string                  `json:"name"        validate:"required,min=2,max=150"`
	Category    entity.MenuItemCategory `json:"category"    validate:"required,oneof=food drink snack other"`
	Price       float64                 `json:"price"       validate:"required,gte=0"`
	Description string                  `json:"description"`
}

// UpdateMenuItemRequest payload untuk update menu (partial)
type UpdateMenuItemRequest struct {
	Name        string                  `json:"name"        validate:"omitempty,min=2,max=150"`
	Category    entity.MenuItemCategory `json:"category"    validate:"omitempty,oneof=food drink snack other"`
	Price       *float64                `json:"price"       validate:"omitempty,gte=0"`
	Description *string                 `json:"description"`
	IsAvailable *bool                   `json:"isAvailable"`
}

func (uc *menuItemUseCase) GetAllMenuItems(ctx context.Context) ([]*entity.MenuItem, error) {
	return uc.menuRepo.FindAll(ctx)
}

func (uc *menuItemUseCase) GetAvailableMenuItems(ctx context.Context) ([]*entity.MenuItem, error) {
	return uc.menuRepo.FindAvailable(ctx)
}

func (uc *menuItemUseCase) GetMenuItemsByCategory(ctx context.Context, category string) ([]*entity.MenuItem, error) {
	cat := entity.MenuItemCategory(category)
	switch cat {
	case entity.MenuCategoryFood, entity.MenuCategoryDrink, entity.MenuCategorySnack, entity.MenuCategoryOther:
	default:
		return nil, errors.New("kategori tidak valid, pilih: food, drink, snack, other")
	}
	return uc.menuRepo.FindByCategory(ctx, cat)
}

func (uc *menuItemUseCase) GetMenuItemByID(ctx context.Context, id uuid.UUID) (*entity.MenuItem, error) {
	item, err := uc.menuRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.New("menu tidak ditemukan")
	}
	return item, nil
}

func (uc *menuItemUseCase) CreateMenuItem(ctx context.Context, req *CreateMenuItemRequest) (*entity.MenuItem, error) {
	item := &entity.MenuItem{
		Name:        req.Name,
		Category:    req.Category,
		Price:       req.Price,
		Description: req.Description,
		IsAvailable: true,
	}
	if err := uc.menuRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (uc *menuItemUseCase) UpdateMenuItem(ctx context.Context, id uuid.UUID, req *UpdateMenuItemRequest) (*entity.MenuItem, error) {
	item, err := uc.menuRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.New("menu tidak ditemukan")
	}

	if req.Name != "" {
		item.Name = req.Name
	}
	if req.Category != "" {
		item.Category = req.Category
	}
	if req.Price != nil {
		item.Price = *req.Price
	}
	if req.Description != nil {
		item.Description = *req.Description
	}
	if req.IsAvailable != nil {
		item.IsAvailable = *req.IsAvailable
	}

	if err := uc.menuRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (uc *menuItemUseCase) DeleteMenuItem(ctx context.Context, id uuid.UUID) error {
	item, err := uc.menuRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if item == nil {
		return errors.New("menu tidak ditemukan")
	}
	return uc.menuRepo.Delete(ctx, id)
}

func (uc *menuItemUseCase) ToggleMenuItem(ctx context.Context, id uuid.UUID) (*entity.MenuItem, error) {
	item, err := uc.menuRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.New("menu tidak ditemukan")
	}
	item.IsAvailable = !item.IsAvailable
	if err := uc.menuRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}
