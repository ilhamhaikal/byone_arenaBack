package router

import (
	"byone-arena/internal/delivery/http/handler"
	"byone-arena/internal/delivery/http/middleware"
	wsHandler "byone-arena/internal/delivery/websocket"
	"byone-arena/pkg/config"
	"byone-arena/pkg/response"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	fiberCors "github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	fiberSwagger "github.com/swaggo/fiber-swagger"
)

// Handlers mengumpulkan semua handler yang diperlukan router
type Handlers struct {
	Auth     *handler.AuthHandler
	Console  *handler.ConsoleHandler
	Session  *handler.SessionHandler
	Customer *handler.CustomerHandler
	Payment  *handler.PaymentHandler
	Shift    *handler.ShiftHandler
	Voucher  *handler.VoucherHandler
	Hub      *wsHandler.Hub
}

// Setup mendaftarkan semua route ke Fiber app
func Setup(app *fiber.App, h *Handlers, cfg *config.Config) {
	// Middleware global
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))
	app.Use(fiberCors.New(fiberCors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, PATCH, DELETE, OPTIONS",
	}))

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return response.OK(c, "Byone Arena API aktif", fiber.Map{
			"status":  "ok",
			"version": "1.0.0",
		})
	})

	// Swagger UI - dokumentasi API
	app.Get("/swagger/*", fiberSwagger.WrapHandler)

	// WebSocket upgrade middleware
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// WebSocket endpoint - dapat diakses tanpa auth untuk mobile & TV
	app.Get("/ws", websocket.New(func(conn *websocket.Conn) {
		h.Hub.HandleConnection(conn)
	}))

	// API v1 routes
	api := app.Group("/api/v1")

	// Auth routes (publik)
	auth := api.Group("/auth")
	auth.Post("/login", h.Auth.Login)
	auth.Post("/register", h.Auth.Register) // Sebaiknya dinonaktifkan di production

	// Protected routes (memerlukan JWT)
	protected := api.Group("", middleware.AuthMiddleware(cfg))

	// Console routes
	consoles := protected.Group("/consoles")
	consoles.Get("/", h.Console.GetAll)
	consoles.Get("/available", h.Console.GetAvailable)
	consoles.Get("/:id", h.Console.GetByID)
	consoles.Post("/", h.Console.Create)
	consoles.Put("/:id", h.Console.Update)
	consoles.Delete("/:id", h.Console.Delete)

	// Session routes
	sessions := protected.Group("/sessions")
	sessions.Get("/", h.Session.GetAll)
	sessions.Get("/active", h.Session.GetActive)
	sessions.Get("/:id", h.Session.GetByID)
	sessions.Post("/start", h.Session.Start)
	sessions.Patch("/:id/end", h.Session.End)
	sessions.Patch("/:id/cancel", h.Session.Cancel)
	sessions.Get("/:session_id/payment", h.Payment.GetBySession)

	// Customer routes
	customers := protected.Group("/customers")
	customers.Get("/", h.Customer.GetAll)
	customers.Get("/:id", h.Customer.GetByID)
	customers.Post("/", h.Customer.Create)
	customers.Put("/:id", h.Customer.Update)
	customers.Delete("/:id", h.Customer.Delete)

	// Payment routes
	payments := protected.Group("/payments")
	payments.Get("/:id", h.Payment.GetByID)
	payments.Post("/", h.Payment.Create)
	payments.Patch("/:id/refund", h.Payment.Refund)

	// Shift routes (admin & superadmin only)
	shifts := protected.Group("/shifts", middleware.AdminOnly())
	shifts.Get("/", h.Shift.GetAll)
	shifts.Get("/:id", h.Shift.GetByID)
	shifts.Post("/", h.Shift.Create)
	shifts.Put("/:id", h.Shift.Update)
	shifts.Delete("/:id", h.Shift.Delete)

	// Shifts by user
	protected.Get("/users/:user_id/shifts", middleware.AdminOnly(), h.Shift.GetByUser)

	// Voucher routes (admin & superadmin only)
	vouchers := protected.Group("/vouchers", middleware.AdminOnly())
	vouchers.Get("/", h.Voucher.GetAll)
	vouchers.Get("/code/:code", h.Voucher.GetByCode)
	vouchers.Get("/:id", h.Voucher.GetByID)
	vouchers.Post("/", h.Voucher.Create)
	vouchers.Put("/:id", h.Voucher.Update)
	vouchers.Patch("/:id/toggle", h.Voucher.Toggle)
	vouchers.Delete("/:id", h.Voucher.Delete)

	// 404 handler
	app.Use(func(c *fiber.Ctx) error {
		return response.NotFound(c, "Endpoint tidak ditemukan")
	})
}
