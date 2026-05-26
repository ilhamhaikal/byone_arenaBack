package handler

import (
	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// MenuItemHandler menangani HTTP request untuk manajemen menu makanan & minuman
type MenuItemHandler struct {
	menuUC    usecase.MenuItemUseCase
	validator *validator.Validator
}

// NewMenuItemHandler membuat instance baru MenuItemHandler
func NewMenuItemHandler(menuUC usecase.MenuItemUseCase, v *validator.Validator) *MenuItemHandler {
	return &MenuItemHandler{menuUC: menuUC, validator: v}
}

// GetAll godoc
// @Summary      Ambil semua menu
// @Description  Mengembalikan seluruh daftar menu makanan & minuman (termasuk yang tidak tersedia)
// @Tags         Menu
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]entity.MenuItem}
// @Failure      401  {object}  response.ErrorResponse
// @Router       /api/v1/menus [get]
func (h *MenuItemHandler) GetAll(c *fiber.Ctx) error {
	items, err := h.menuUC.GetAllMenuItems(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil data menu")
	}
	return response.OK(c, "Data menu berhasil diambil", items)
}

// GetAvailable godoc
// @Summary      Ambil menu yang tersedia
// @Description  Mengembalikan hanya menu yang sedang tersedia, diurutkan per kategori
// @Tags         Menu
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]entity.MenuItem}
// @Failure      401  {object}  response.ErrorResponse
// @Router       /api/v1/menus/available [get]
func (h *MenuItemHandler) GetAvailable(c *fiber.Ctx) error {
	items, err := h.menuUC.GetAvailableMenuItems(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil data menu")
	}
	return response.OK(c, "Menu tersedia berhasil diambil", items)
}

// GetByCategory godoc
// @Summary      Ambil menu berdasarkan kategori
// @Description  Mengembalikan menu tersedia berdasarkan kategori: food, drink, snack, other
// @Tags         Menu
// @Produce      json
// @Security     BearerAuth
// @Param        category  path  string  true  "Kategori menu (food|drink|snack|other)"
// @Success      200  {object}  response.Response{data=[]entity.MenuItem}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Router       /api/v1/menus/category/{category} [get]
func (h *MenuItemHandler) GetByCategory(c *fiber.Ctx) error {
	category := c.Params("category")
	items, err := h.menuUC.GetMenuItemsByCategory(c.Context(), category)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Menu berhasil diambil", items)
}

// GetByID godoc
// @Summary      Ambil menu berdasarkan ID
// @Tags         Menu
// @Produce      json
// @Security     BearerAuth
// @Param        id   path  string  true  "Menu ID (UUID)"
// @Success      200  {object}  response.Response{data=entity.MenuItem}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/menus/{id} [get]
func (h *MenuItemHandler) GetByID(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	item, err := h.menuUC.GetMenuItemByID(c.Context(), id)
	if err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.OK(c, "Data menu berhasil diambil", item)
}

// Create godoc
// @Summary      Buat menu baru
// @Description  Admin membuat item menu baru. Kategori: food, drink, snack, other
// @Tags         Menu
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  usecase.CreateMenuItemRequest  true  "Data menu baru"
// @Success      201  {object}  response.Response{data=entity.MenuItem}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Router       /api/v1/menus [post]
func (h *MenuItemHandler) Create(c *fiber.Ctx) error {
	req := new(usecase.CreateMenuItemRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	item, err := h.menuUC.CreateMenuItem(c.Context(), req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "Menu berhasil dibuat", item)
}

// Update godoc
// @Summary      Update menu
// @Description  Mengubah data menu (partial update). Semua field opsional.
// @Tags         Menu
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                        true  "Menu ID (UUID)"
// @Param        body  body  usecase.UpdateMenuItemRequest true  "Data yang diubah"
// @Success      200  {object}  response.Response{data=entity.MenuItem}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/menus/{id} [put]
func (h *MenuItemHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	req := new(usecase.UpdateMenuItemRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	item, err := h.menuUC.UpdateMenuItem(c.Context(), id, req)
	if err != nil {
		if err.Error() == "menu tidak ditemukan" {
			return response.NotFound(c, err.Error())
		}
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Menu berhasil diperbarui", item)
}

// Toggle godoc
// @Summary      Toggle ketersediaan menu
// @Description  Mengubah status tersedia/tidak tersedia menu
// @Tags         Menu
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Menu ID (UUID)"
// @Success      200  {object}  response.Response{data=entity.MenuItem}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/menus/{id}/toggle [patch]
func (h *MenuItemHandler) Toggle(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	item, err := h.menuUC.ToggleMenuItem(c.Context(), id)
	if err != nil {
		if err.Error() == "menu tidak ditemukan" {
			return response.NotFound(c, err.Error())
		}
		return response.InternalServerError(c, "Gagal mengubah status menu")
	}
	status := "tidak tersedia"
	if item.IsAvailable {
		status = "tersedia"
	}
	return response.OK(c, "Menu sekarang "+status, item)
}

// Delete godoc
// @Summary      Hapus menu
// @Description  Menghapus item menu secara permanen. Tidak bisa dihapus jika masih ada pesanan aktif.
// @Tags         Menu
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Menu ID (UUID)"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/menus/{id} [delete]
func (h *MenuItemHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	if err := h.menuUC.DeleteMenuItem(c.Context(), id); err != nil {
		if err.Error() == "menu tidak ditemukan" {
			return response.NotFound(c, err.Error())
		}
		return response.InternalServerError(c, "Gagal menghapus menu. Pastikan menu tidak digunakan dalam pesanan aktif.")
	}
	return response.OK(c, "Menu berhasil dihapus", nil)
}
