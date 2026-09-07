package handler

import (
	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// IngredientHandler menangani HTTP request untuk bahan, resep, dan batch stok.
type IngredientHandler struct {
	ingredientUC usecase.IngredientUseCase
	validator    *validator.Validator
}

// NewIngredientHandler membuat instance baru IngredientHandler
func NewIngredientHandler(ingredientUC usecase.IngredientUseCase, v *validator.Validator) *IngredientHandler {
	return &IngredientHandler{ingredientUC: ingredientUC, validator: v}
}

func (h *IngredientHandler) parseID(c *fiber.Ctx) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// GetAll godoc
// @Summary      Ambil semua bahan
// @Tags         Bahan
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]entity.Ingredient}
// @Router       /api/v1/ingredients [get]
func (h *IngredientHandler) GetAll(c *fiber.Ctx) error {
	items, err := h.ingredientUC.GetAllIngredients(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil data bahan")
	}
	return response.OK(c, "Data bahan berhasil diambil", items)
}

// GetByID godoc
// @Summary      Ambil bahan berdasarkan ID
// @Tags         Bahan
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Ingredient ID (UUID)"
// @Success      200  {object}  response.Response{data=entity.Ingredient}
// @Router       /api/v1/ingredients/{id} [get]
func (h *IngredientHandler) GetByID(c *fiber.Ctx) error {
	id, err := h.parseID(c)
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	item, err := h.ingredientUC.GetIngredientByID(c.Context(), id)
	if err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.OK(c, "Data bahan berhasil diambil", item)
}

// Create godoc
// @Summary      Buat bahan baru
// @Tags         Bahan
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  usecase.CreateIngredientRequest  true  "Data bahan baru"
// @Success      201  {object}  response.Response{data=entity.Ingredient}
// @Router       /api/v1/ingredients [post]
func (h *IngredientHandler) Create(c *fiber.Ctx) error {
	req := new(usecase.CreateIngredientRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}
	item, err := h.ingredientUC.CreateIngredient(c.Context(), req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "Bahan berhasil dibuat", item)
}

// Update godoc
// @Summary      Update bahan
// @Tags         Bahan
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                        true  "Ingredient ID (UUID)"
// @Param        body  body  usecase.UpdateIngredientRequest true  "Data yang diubah"
// @Success      200  {object}  response.Response{data=entity.Ingredient}
// @Router       /api/v1/ingredients/{id} [put]
func (h *IngredientHandler) Update(c *fiber.Ctx) error {
	id, err := h.parseID(c)
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	req := new(usecase.UpdateIngredientRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}
	item, err := h.ingredientUC.UpdateIngredient(c.Context(), id, req)
	if err != nil {
		if err.Error() == "bahan tidak ditemukan" {
			return response.NotFound(c, err.Error())
		}
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Bahan berhasil diperbarui", item)
}

// Delete godoc
// @Summary      Hapus bahan
// @Tags         Bahan
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Ingredient ID (UUID)"
// @Success      200  {object}  response.Response
// @Router       /api/v1/ingredients/{id} [delete]
func (h *IngredientHandler) Delete(c *fiber.Ctx) error {
	id, err := h.parseID(c)
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	if err := h.ingredientUC.DeleteIngredient(c.Context(), id); err != nil {
		if err.Error() == "bahan tidak ditemukan" {
			return response.NotFound(c, err.Error())
		}
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Bahan berhasil dihapus", nil)
}

// AddStock godoc
// @Summary      Tambah stok bahan (batch + tanggal kadaluarsa)
// @Tags         Bahan
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                 true  "Ingredient ID (UUID)"
// @Param        body  body  usecase.AddStockRequest true  "Quantity + expiresAt"
// @Success      201  {object}  response.Response{data=entity.IngredientStockBatch}
// @Router       /api/v1/ingredients/{id}/stock [post]
func (h *IngredientHandler) AddStock(c *fiber.Ctx) error {
	id, err := h.parseID(c)
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	req := new(usecase.AddStockRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}
	batch, err := h.ingredientUC.AddIngredientStock(c.Context(), id, req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "Stok bahan berhasil ditambahkan", batch)
}

// ListBatches godoc
// @Summary      Riwayat batch stok bahan
// @Tags         Bahan
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Ingredient ID (UUID)"
// @Success      200  {object}  response.Response{data=[]entity.IngredientStockBatch}
// @Router       /api/v1/ingredients/{id}/stock [get]
func (h *IngredientHandler) ListBatches(c *fiber.Ctx) error {
	id, err := h.parseID(c)
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	batches, err := h.ingredientUC.ListIngredientBatches(c.Context(), id)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Riwayat stok bahan berhasil diambil", batches)
}

// SetComponents godoc
// @Summary      Atur sub-resep bahan
// @Tags         Bahan
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                 true  "Ingredient ID (UUID)"
// @Param        body  body  usecase.SetRecipeRequest true  "Daftar komponen bahan"
// @Success      200  {object}  response.Response
// @Router       /api/v1/ingredients/{id}/components [put]
func (h *IngredientHandler) SetComponents(c *fiber.Ctx) error {
	id, err := h.parseID(c)
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	req := new(usecase.SetRecipeRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}
	if err := h.ingredientUC.SetIngredientComponents(c.Context(), id, req); err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Sub-resep bahan berhasil disimpan", nil)
}

// ListComponents godoc
// @Summary      Ambil sub-resep bahan
// @Tags         Bahan
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Ingredient ID (UUID)"
// @Success      200  {object}  response.Response{data=[]entity.IngredientRecipe}
// @Router       /api/v1/ingredients/{id}/components [get]
func (h *IngredientHandler) ListComponents(c *fiber.Ctx) error {
	id, err := h.parseID(c)
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	rows, err := h.ingredientUC.ListIngredientComponents(c.Context(), id)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Sub-resep bahan berhasil diambil", rows)
}

// SetMenuIngredients godoc
// @Summary      Atur resep menu (bahan penyusun)
// @Tags         Menu
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                 true  "Menu ID (UUID)"
// @Param        body  body  usecase.SetRecipeRequest true  "Daftar bahan"
// @Success      200  {object}  response.Response
// @Router       /api/v1/menus/{id}/ingredients [put]
func (h *IngredientHandler) SetMenuIngredients(c *fiber.Ctx) error {
	id, err := h.parseID(c)
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	req := new(usecase.SetRecipeRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}
	if err := h.ingredientUC.SetMenuItemIngredients(c.Context(), id, req); err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Resep menu berhasil disimpan", nil)
}

// ListMenuIngredients godoc
// @Summary      Ambil resep menu
// @Tags         Menu
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Menu ID (UUID)"
// @Success      200  {object}  response.Response{data=[]entity.MenuItemIngredient}
// @Router       /api/v1/menus/{id}/ingredients [get]
func (h *IngredientHandler) ListMenuIngredients(c *fiber.Ctx) error {
	id, err := h.parseID(c)
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	rows, err := h.ingredientUC.ListMenuItemIngredients(c.Context(), id)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Resep menu berhasil diambil", rows)
}

// GetExpired godoc
// @Summary      Daftar bahan/menu kadaluarsa atau hampir kadaluarsa
// @Tags         Bahan
// @Produce      json
// @Security     BearerAuth
// @Param        days  query  int  false  "Ambang hari ke depan (default 3)"
// @Success      200  {object}  response.Response{data=usecase.ExpiredStockResponse}
// @Router       /api/v1/ingredients/expired [get]
func (h *IngredientHandler) GetExpired(c *fiber.Ctx) error {
	days := c.QueryInt("days", 3)
	result, err := h.ingredientUC.GetExpired(c.Context(), days)
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil data kadaluarsa")
	}
	return response.OK(c, "Daftar kadaluarsa berhasil diambil", result)
}

// GetStockLog godoc
// @Summary      Log penambahan stok terpusat (bahan + menu)
// @Tags         Bahan
// @Produce      json
// @Security     BearerAuth
// @Param        startDate  query  string  false  "YYYY-MM-DD (default 30 hari lalu)"
// @Param        endDate    query  string  false  "YYYY-MM-DD (default hari ini)"
// @Success      200  {object}  response.Response{data=[]repository.StockLogEntry}
// @Router       /api/v1/ingredients/stock-log [get]
func (h *IngredientHandler) GetStockLog(c *fiber.Ctx) error {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	rows, err := h.ingredientUC.GetStockLog(c.Context(), startDate, endDate)
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil log stok")
	}
	return response.OK(c, "Log stok berhasil diambil", rows)
}
