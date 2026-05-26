package handler

import (
	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// FoodOrderHandler menangani HTTP request untuk pesanan makanan
type FoodOrderHandler struct {
	orderUC   usecase.FoodOrderUseCase
	validator *validator.Validator
}

// NewFoodOrderHandler membuat instance baru FoodOrderHandler
func NewFoodOrderHandler(orderUC usecase.FoodOrderUseCase, v *validator.Validator) *FoodOrderHandler {
	return &FoodOrderHandler{orderUC: orderUC, validator: v}
}

// GetAll godoc
// @Summary      Ambil semua pesanan makanan
// @Description  Mengembalikan seluruh pesanan makanan diurutkan terbaru
// @Tags         FoodOrder
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]entity.FoodOrder}
// @Failure      401  {object}  response.ErrorResponse
// @Router       /api/v1/food-orders [get]
func (h *FoodOrderHandler) GetAll(c *fiber.Ctx) error {
	orders, err := h.orderUC.GetAllOrders(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil data pesanan")
	}
	return response.OK(c, "Data pesanan berhasil diambil", orders)
}

// GetByStatus godoc
// @Summary      Ambil pesanan berdasarkan status
// @Description  Filter pesanan berdasarkan status: pending, preparing, served, cancelled
// @Tags         FoodOrder
// @Produce      json
// @Security     BearerAuth
// @Param        status  query  string  true  "Status pesanan"
// @Success      200  {object}  response.Response{data=[]entity.FoodOrder}
// @Failure      400  {object}  response.ErrorResponse
// @Router       /api/v1/food-orders/status [get]
func (h *FoodOrderHandler) GetByStatus(c *fiber.Ctx) error {
	status := c.Query("status")
	if status == "" {
		return response.BadRequest(c, "Parameter status diperlukan (pending|preparing|served|cancelled)")
	}
	orders, err := h.orderUC.GetOrdersByStatus(c.Context(), status)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Pesanan berhasil diambil", orders)
}

// GetByID godoc
// @Summary      Ambil pesanan berdasarkan ID
// @Description  Mengembalikan detail pesanan beserta semua item dan info menu
// @Tags         FoodOrder
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Food Order ID (UUID)"
// @Success      200  {object}  response.Response{data=entity.FoodOrder}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/food-orders/{id} [get]
func (h *FoodOrderHandler) GetByID(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	order, err := h.orderUC.GetOrderByID(c.Context(), id)
	if err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.OK(c, "Data pesanan berhasil diambil", order)
}

// GetBySession godoc
// @Summary      Ambil pesanan makanan dari satu sesi PS
// @Description  Mengembalikan semua pesanan makanan yang terhubung ke sesi PS tertentu
// @Tags         FoodOrder
// @Produce      json
// @Security     BearerAuth
// @Param        session_id  path  string  true  "Session ID (UUID)"
// @Success      200  {object}  response.Response{data=[]entity.FoodOrder}
// @Failure      400  {object}  response.ErrorResponse
// @Router       /api/v1/sessions/{session_id}/food-orders [get]
func (h *FoodOrderHandler) GetBySession(c *fiber.Ctx) error {
	sessionID, err := uuid.Parse(c.Params("session_id"))
	if err != nil {
		return response.BadRequest(c, "Format session ID tidak valid")
	}
	orders, err := h.orderUC.GetOrdersBySession(c.Context(), sessionID)
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil pesanan sesi")
	}
	return response.OK(c, "Pesanan sesi berhasil diambil", orders)
}

// Create godoc
// @Summary      Buat pesanan makanan baru
// @Description  Admin membuat pesanan makanan. Harga otomatis diambil dari data menu, total dihitung otomatis dari quantity × harga.
// @Tags         FoodOrder
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  usecase.CreateFoodOrderRequest  true  "Data pesanan baru"
// @Success      201  {object}  response.Response{data=entity.FoodOrder}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Router       /api/v1/food-orders [post]
func (h *FoodOrderHandler) Create(c *fiber.Ctx) error {
	req := new(usecase.CreateFoodOrderRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	order, err := h.orderUC.CreateOrder(c.Context(), req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "Pesanan berhasil dibuat", order)
}

// UpdateStatus godoc
// @Summary      Update status pesanan
// @Description  Mengubah status pesanan. Alur: pending → preparing → served. Dapat dibatalkan (cancelled) dari status apapun kecuali served.
// @Tags         FoodOrder
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                           true  "Food Order ID (UUID)"
// @Param        body  body  usecase.UpdateOrderStatusRequest true  "Status baru"
// @Success      200  {object}  response.Response{data=entity.FoodOrder}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/food-orders/{id}/status [patch]
func (h *FoodOrderHandler) UpdateStatus(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	req := new(usecase.UpdateOrderStatusRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	order, err := h.orderUC.UpdateOrderStatus(c.Context(), id, req)
	if err != nil {
		if err.Error() == "pesanan tidak ditemukan" {
			return response.NotFound(c, err.Error())
		}
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Status pesanan berhasil diperbarui", order)
}

// Cancel godoc
// @Summary      Batalkan pesanan
// @Description  Membatalkan pesanan makanan. Tidak bisa membatalkan pesanan yang sudah served.
// @Tags         FoodOrder
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Food Order ID (UUID)"
// @Success      200  {object}  response.Response{data=entity.FoodOrder}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/food-orders/{id}/cancel [patch]
func (h *FoodOrderHandler) Cancel(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	order, err := h.orderUC.CancelOrder(c.Context(), id)
	if err != nil {
		if err.Error() == "pesanan tidak ditemukan" {
			return response.NotFound(c, err.Error())
		}
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Pesanan berhasil dibatalkan", order)
}

// Delete godoc
// @Summary      Hapus pesanan
// @Description  Menghapus pesanan secara permanen. Hanya pesanan berstatus 'cancelled' yang bisa dihapus.
// @Tags         FoodOrder
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Food Order ID (UUID)"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/food-orders/{id} [delete]
func (h *FoodOrderHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	if err := h.orderUC.DeleteOrder(c.Context(), id); err != nil {
		if err.Error() == "pesanan tidak ditemukan" {
			return response.NotFound(c, err.Error())
		}
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Pesanan berhasil dihapus", nil)
}
