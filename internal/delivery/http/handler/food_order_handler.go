package handler

import (
	wsHandler "byone-arena/internal/delivery/websocket"
	"byone-arena/internal/license"
	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"
	"byone-arena/pkg/spname"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// FoodOrderHandler menangani HTTP request untuk pesanan makanan
type FoodOrderHandler struct {
	orderUC   usecase.FoodOrderUseCase
	validator *validator.Validator
	db        *gorm.DB
	hub       *wsHandler.Hub
	license   *license.Service
}

// NewFoodOrderHandler membuat instance baru FoodOrderHandler
func NewFoodOrderHandler(orderUC usecase.FoodOrderUseCase, v *validator.Validator, db *gorm.DB, hub *wsHandler.Hub, licenseSvc *license.Service) *FoodOrderHandler {
	return &FoodOrderHandler{orderUC: orderUC, validator: v, db: db, hub: hub, license: licenseSvc}
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

	// Lisensi: blokir pesanan baru bila lisensi sudah berakhir.
	if h.license != nil {
		if err := h.license.EnsureCanStartBusiness(c.Context()); err != nil {
			return response.Forbidden(c, err.Error())
		}
	}

	order, err := h.orderUC.CreateOrder(c.Context(), req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	order.PaymentMethod = normalizePaymentMethod(req.PaymentMethod)

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

	// Broadcast + insert notifikasi ke client (TV display) jika pesanan sedang disiapkan
	if req.Status == "preparing" && h.hub != nil {
		msg := "Pesanan " + order.OrderNumber + " sedang disiapkan! 🍳"

		// Targetkan notifikasi HANYA ke konsol milik sesi pesanan ini (bukan
		// broadcast ke semua TV) supaya tidak muncul di konsol lain yang
		// sedang/sudah dipakai oleh sesi yang berbeda.
		var consoleIDStr string
		if order.Session != nil {
			consoleIDStr = order.Session.ConsoleID.String()
		}

		// WebSocket broadcast (payload konsisten dengan event lain: consoleId)
		evt := wsHandler.NewEvent("food_order_preparing", fiber.Map{
			"message":     msg,
			"orderId":     order.ID,
			"orderNumber": order.OrderNumber,
			"consoleId":   consoleIDStr,
		})
		if cid, err := uuid.Parse(consoleIDStr); err == nil {
			evt.ConsoleID = &cid
		}
		h.hub.Broadcast(evt)

		// Insert ke tv_notifications untuk client HTTP polling.
		// active_sessions_only=true + target console spesifik → hanya TV
		// konsol pesanan ini yang menampilkan, dan hanya jika masih ada sesi.
		if consoleIDStr != "" {
			// Nonaktifkan dulu notifikasi 'Pesanan Disiapkan' lama milik
			// konsol ini agar tidak muncul kembali saat sesi baru dimulai.
			h.db.WithContext(c.Context()).Exec(
				"SELECT "+spname.Ident("DeactivateTvNotification")+"(?::text, ?::text)",
				"Pesanan Disiapkan", consoleIDStr)
			h.db.WithContext(c.Context()).Exec(
				"SELECT "+spname.Ident("InsertTvNotification")+"(?::text, ?::text, ?::text, ?::boolean, ?::varchar)",
				"Pesanan Disiapkan", msg, consoleIDStr, true, "normal")
		}
	}

	return response.OK(c, "Status pesanan berhasil diperbarui", order)
}

// FoodPricePreviewItem satu item dalam preview harga makanan
type FoodPricePreviewItem struct {
	MenuID   string  `json:"menuId"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
	Subtotal float64 `json:"subtotal"`
}

// FoodPricePreviewRequest payload untuk kalkulasi harga + diskon
type FoodPricePreviewRequest struct {
	Items      []FoodPricePreviewItem `json:"items"      validate:"required,min=1"`
	CustomerID *string                `json:"customerId"`
	SessionID  *string                `json:"sessionId"`
}

// FoodPricePreviewResponse hasil kalkulasi harga makanan + auto diskon
type FoodPricePreviewResponse struct {
	TotalAmount  float64 `json:"totalAmount"`
	AutoDiscount float64 `json:"autoDiscount"`
	FinalAmount  float64 `json:"finalAmount"`
}

// GetPricePreview godoc
// @Summary      Kalkulasi harga makanan + auto diskon
// @Description  Hitung total harga menu items lalu aplikasikan aturan diskon otomatis yang aktif. Tidak membuat order.
// @Tags         FoodOrder
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  handler.FoodPricePreviewRequest  true  "List menu items"
// @Success      200   {object}  response.Response{data=handler.FoodPricePreviewResponse}
// @Router       /api/v1/food-orders/price-preview [post]
func (h *FoodOrderHandler) GetPricePreview(c *fiber.Ctx) error {
	req := new(FoodPricePreviewRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}

	// Hitung total dari menu items
	var totalAmount float64
	for _, item := range req.Items {
		var priceRow struct {
			Price float64 `gorm:"column:price"`
		}
		h.db.WithContext(c.Context()).Raw(
			"SELECT * FROM "+spname.Ident("MenuItemPrice")+"(?::uuid)", item.MenuID,
		).Scan(&priceRow)
		subtotal := priceRow.Price * float64(item.Quantity)
		totalAmount += subtotal
	}

	if totalAmount <= 0 {
		return response.BadRequest(c, "Total belanja tidak valid")
	}

	// Cek apakah customer adalah member. Jika customer tidak dikirim,
	// gunakan customer milik sesi agar preview sama dengan perhitungan SP.
	var customerID *uuid.UUID
	if req.CustomerID != nil && *req.CustomerID != "" {
		if parsed, err := uuid.Parse(*req.CustomerID); err == nil {
			customerID = &parsed
		}
	}
	if customerID == nil && req.SessionID != nil && *req.SessionID != "" {
		if sessionID, err := uuid.Parse(*req.SessionID); err == nil {
			var sessionRow struct {
				CustomerID *uuid.UUID `gorm:"column:customer_id"`
			}
			h.db.WithContext(c.Context()).Table("sessions").
				Select("customer_id").Where("id = ?", sessionID).Scan(&sessionRow)
			customerID = sessionRow.CustomerID
		}
	}

	var isMember bool
	if customerID != nil {
		var memberRow struct {
			IsMember bool `gorm:"column:is_member"`
		}
		h.db.WithContext(c.Context()).Raw(
			"SELECT * FROM "+spname.Ident("CustomerIsMember")+"(?::uuid)", *customerID,
		).Scan(&memberRow)
		isMember = memberRow.IsMember
	}

	// Aplikasikan auto-diskon via EvaluateDiscountRules
	var autoDiscount float64
	h.db.WithContext(c.Context()).Raw(
		"SELECT COALESCE("+spname.Ident("EvaluateDiscountRules")+"(?::NUMERIC, NOW(), ?::BOOLEAN), 0)",
		totalAmount, isMember).Scan(&autoDiscount)

	finalAmount := totalAmount - autoDiscount
	if finalAmount < 0 {
		finalAmount = 0
	}

	return response.OK(c, "Preview harga makanan", FoodPricePreviewResponse{
		TotalAmount:  totalAmount,
		AutoDiscount: autoDiscount,
		FinalAmount:  finalAmount,
	})
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
