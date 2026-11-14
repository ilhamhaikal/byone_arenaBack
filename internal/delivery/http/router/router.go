package router

import (
	"os"
	"path/filepath"
	"time"

	"byone-arena/internal/delivery/http/handler"
	"byone-arena/internal/delivery/http/middleware"
	wsHandler "byone-arena/internal/delivery/websocket"
	"byone-arena/pkg/config"
	"byone-arena/pkg/response"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	fiberCors "github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	fiberSwagger "github.com/swaggo/fiber-swagger"
	"gorm.io/gorm"
)

// Handlers mengumpulkan semua handler yang diperlukan router
type Handlers struct {
	Auth          *handler.AuthHandler
	Console       *handler.ConsoleHandler
	Session       *handler.SessionHandler
	Customer      *handler.CustomerHandler
	Payment       *handler.PaymentHandler
	Shift         *handler.ShiftHandler
	Voucher       *handler.VoucherHandler
	Discount      *handler.DiscountHandler
	Menu          *handler.MenuItemHandler
	Ingredient    *handler.IngredientHandler
	FoodOrder     *handler.FoodOrderHandler
	Capital       *handler.CapitalHandler
	RentalPackage *handler.RentalPackageHandler
	Dashboard     *handler.DashboardHandler
	Report        *handler.ReportHandler
	Notify        *handler.NotificationHandler
	Rental        *handler.RentalHandler
	Settings      *handler.SettingsHandler
	Activity      *handler.ActivityHandler
	User          *handler.UserHandler
	Device        *handler.DeviceHandler
	Audit         *handler.AuditHandler
	Promo         *handler.PromoHandler
	Hub           *wsHandler.Hub
}

// Setup mendaftarkan semua route ke Fiber app
func Setup(app *fiber.App, h *Handlers, cfg *config.Config, db *gorm.DB) {
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

	// Swagger UI - hanya di development (jangan terekspos di production)
	if cfg.AppEnv != "production" {
		app.Get("/swagger/*", fiberSwagger.WrapHandler)
	}

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

	// Update client TV/HP: manifest + file APK (statis).
	if cfg.UpdatesDir != "" {
		app.Static("/updates", cfg.UpdatesDir)
		api.Get("/updates/latest", func(c *fiber.Ctx) error {
			b, err := os.ReadFile(filepath.Join(cfg.UpdatesDir, "manifest.json"))
			if err != nil {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"success": false,
					"message": "Belum ada update yang dipublikasikan",
				})
			}
			c.Set("Content-Type", "application/json")
			c.Set("Cache-Control", "no-store")
			return c.Send(b)
		})
	}

	// Auth routes (publik)
	auth := api.Group("/auth")
	// Batasi percobaan login untuk mencegah brute force password kasir.
	auth.Post("/login", limiter.New(limiter.Config{
		Max:        10,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"success": false,
				"message": "Terlalu banyak percobaan login, coba lagi dalam 1 menit",
			})
		},
	}), h.Auth.Login)
	if cfg.AppEnv != "production" {
		auth.Post("/register", h.Auth.Register)
	}

	// Console overview — publik, digunakan oleh client Android TV tanpa login
	api.Get("/consoles/overview", h.Console.GetOverview)

	// Heartbeat — publik, dari TV Android
	api.Post("/consoles/:id/heartbeat", h.Console.Heartbeat)

	// Pairing TV: publik, dipanggil aplikasi TV setelah menemukan server.
	api.Post("/devices/announce", h.Device.Announce)

	// Promo bonus yang berlaku hari ini — publik (dipakai TV & dialog sesi).
	api.Get("/promo-bonuses/active", h.Promo.Active)

	// Routes publik lain — HARUS sebelum protected group
	api.Get("/notifications", h.Notify.GetAllNotifications)
	api.Get("/notifications/loop/status", h.Notify.LoopStatus)

	// Protected routes (memerlukan JWT)
	protected := api.Group("", middleware.AuthMiddleware(cfg, db))

	// Profil pemilik token — dipakai frontend untuk memvalidasi sesi.
	protected.Get("/auth/me", h.Auth.Me)

	// Audit trail aksi sensitif — admin & superadmin.
	protected.Get("/audit-logs", middleware.AdminOnly(), h.Audit.List)

	// Dashboard summary — memerlukan autentikasi
	protected.Get("/dashboard/summary", h.Dashboard.GetSummary)
	protected.Get("/dashboard/revenue-chart", h.Dashboard.GetRevenueChart)

	// Report routes — admin & superadmin only
	reports := protected.Group("/reports", middleware.AdminOnly())
	reports.Get("/summary", h.Report.GetSummary)
	reports.Get("/promo-bonus", h.Promo.PromoReport)

	// Capital routes (pembukuan modal awal) — admin & superadmin only
	capital := protected.Group("/capital", middleware.AdminOnly())
	capital.Get("/", h.Capital.ListEntries)
	capital.Post("/", h.Capital.AddEntry)
	capital.Delete("/:id", h.Capital.DeleteEntry)
	capital.Get("/monthly", h.Capital.MonthlyReport)

	// Rental Package routes (paket begadang dll) — admin & superadmin only
	packages := protected.Group("/rental-packages", middleware.AdminOnly())
	packages.Get("/", h.RentalPackage.List)
	packages.Post("/", h.RentalPackage.Save)
	packages.Delete("/:id", h.RentalPackage.Delete)
	// Mulai sesi dengan paket (semua role terautentikasi)
	protected.Post("/sessions/start-package", h.RentalPackage.Start)

	// Activity routes — realtime activity feed
	activities := protected.Group("/activities")
	activities.Get("/recent", h.Activity.GetRecentActivities)

	// Notification routes — semua role terautentikasi bisa kelola
	notifications := protected.Group("/notifications")
	notifications.Post("/", h.Notify.CreateNotification)
	notifications.Put("/:id", h.Notify.UpdateNotification)
	notifications.Delete("/:id", h.Notify.DeleteNotification)
	notifications.Patch("/:id/toggle", h.Notify.ToggleNotification)
	notifications.Post("/loop/start", h.Notify.StartLoop)
	notifications.Post("/loop/stop", h.Notify.StopLoop)

	// GET notifications + loop/status HARUS publik — daftar langsung di app, bypass group
	app.Get("/api/v1/notifications", h.Notify.GetAllNotifications)
	app.Get("/api/v1/notifications/loop/status", h.Notify.LoopStatus)

	// Daily Rental routes
	dailyRentals := protected.Group("/daily-rentals")
	dailyRentals.Get("/", h.Rental.GetAllDailyRentals)
	dailyRentals.Post("/", h.Rental.CreateDailyRental)
	dailyRentals.Post("/:id/return", h.Rental.ReturnDailyRental)

	// Booking routes
	bookings := protected.Group("/bookings")
	bookings.Get("/", h.Rental.GetAllBookings)
	bookings.Post("/", h.Rental.CreateBooking)
	bookings.Patch("/:id/status", h.Rental.UpdateBookingStatus)

	// Settings routes — GET publik, PUT admin
	api.Get("/settings/membership", h.Settings.GetMembershipPrice)
	protected.Put("/settings/membership", middleware.AdminOnly(), h.Settings.UpdateMembershipPrice)

	// Console routes — create & edit semua role, delete admin only
	consoles := protected.Group("/consoles")
	consoles.Get("/", h.Console.GetAll)
	consoles.Get("/available", h.Console.GetAvailable)
	consoles.Get("/overview", h.Console.GetOverview)
	consoles.Get("/:id", h.Console.GetByID)
	consoles.Post("/", h.Console.Create)
	// Harga & tarif konsol hanya boleh diubah admin: mencegah kasir menurunkan
	// harga sebelum memulai sesi.
	consoles.Put("/:id", middleware.AdminOnly(), h.Console.Update)
	consoles.Delete("/:id", middleware.AdminOnly(), h.Console.Delete)
	consoles.Get("/:id/price", h.Console.PreviewPrice)
	// TV Control — admin only
	consoles.Post("/:id/wake", middleware.AdminOnly(), h.Notify.WakeConsole)
	consoles.Post("/:id/sleep", middleware.AdminOnly(), h.Notify.SleepConsole)
	consoles.Post("/:id/screensaver", middleware.AdminOnly(), h.Notify.ScreensaverConsole)
	consoles.Get("/:id/tv-logs", h.Console.GetTvLogs) // log aktivitas TV

	// Pairing perangkat TV — admin/superadmin
	devices := protected.Group("/devices", middleware.AdminOnly())
	devices.Get("/pending", h.Device.Pending)
	devices.Post("/:id/bind", h.Device.Bind)
	devices.Delete("/:id", h.Device.Dismiss)

	// Promo bonus waktu: baca aturan aktif untuk semua role, kelola hanya admin.
	promoRules := protected.Group("/promo-bonuses")
	promoRules.Get("/", middleware.AdminOnly(), h.Promo.List)
	promoRules.Post("/", middleware.AdminOnly(), h.Promo.Create)
	promoRules.Put("/:id", middleware.AdminOnly(), h.Promo.Update)
	promoRules.Delete("/:id", middleware.AdminOnly(), h.Promo.Delete)

	// Session routes
	sessions := protected.Group("/sessions")
	sessions.Get("/", h.Session.GetAll)
	sessions.Get("/active", h.Session.GetActive)
	sessions.Get("/:id", h.Session.GetByID)
	sessions.Post("/start", h.Session.Start)
	sessions.Patch("/:id/end", h.Session.End)
	sessions.Patch("/:id/cancel", h.Session.Cancel)
	sessions.Post("/:id/extend", h.Session.Extend)            // tambah waktu sewa
	sessions.Post("/:id/promo-bonus", h.Promo.ApplyToSession) // pakai bonus waktu promo
	sessions.Get("/:session_id/payment", h.Payment.GetBySession)
	sessions.Get("/:session_id/payments", h.Payment.GetAllBySession)                        // SEMUA payment sesi (base + tiap extend) + ringkasan total
	sessions.Post("/:session_id/payments/confirm-pending", h.Payment.ConfirmSessionPending) // lunasi semua payment pending sesi ini sekaligus + hitung kembalian gabungan

	// Customer routes — create & edit semua role, delete admin only
	customers := protected.Group("/customers")
	customers.Get("/", h.Customer.GetAll)
	customers.Get("/:id", h.Customer.GetByID)
	customers.Post("/", h.Customer.Create)
	customers.Put("/:id", h.Customer.Update)
	customers.Delete("/:id", middleware.AdminOnly(), h.Customer.Delete)
	customers.Post("/:id/membership", h.Customer.SellMembership)
	customers.Get("/:id/loyalty", h.Customer.GetLoyalty)

	// Payment routes
	payments := protected.Group("/payments")
	payments.Get("/pending", h.Payment.GetPendingExtensions) // list pending extend payments — HARUS sebelum /:id
	payments.Get("/:id", h.Payment.GetByID)
	payments.Post("/", h.Payment.Create)
	payments.Patch("/:id/refund", middleware.AdminOnly(), h.Payment.Refund)
	payments.Post("/:id/confirm", h.Payment.Confirm) // admin konfirmasi pembayaran extend

	// Shift routes (admin & superadmin only)
	shifts := protected.Group("/shifts", middleware.AdminOnly())
	shifts.Get("/", h.Shift.GetAll)
	shifts.Get("/:id", h.Shift.GetByID)
	// Jadwal shift menentukan validasi jam kerja kasir: hanya admin yang boleh
	// membuat/mengubah/menghapus, agar kasir tidak bisa memberi shift 24 jam
	// untuk dirinya sendiri atau menghapus riwayat jam kerjanya.
	shifts.Post("/", middleware.AdminOnly(), h.Shift.Create)
	shifts.Put("/:id", middleware.AdminOnly(), h.Shift.Update)
	shifts.Delete("/:id", middleware.AdminOnly(), h.Shift.Delete)

	// Shifts by user
	protected.Get("/users/:user_id/shifts", middleware.AdminOnly(), h.Shift.GetByUser)

	// User management routes
	// Change password — semua user terautentikasi bisa (HARUS sebelum /:id)
	protected.Patch("/users/change-password", h.User.ChangePassword)

	users := protected.Group("/users", middleware.AdminOnly())
	users.Get("/", h.User.GetAll)
	users.Get("/:id", h.User.GetByID)
	users.Post("/", h.User.Create)
	users.Put("/:id", h.User.Update)
	users.Delete("/:id", h.User.Delete)
	// Change password — semua user terautentikasi bisa
	protected.Patch("/users/change-password", h.User.ChangePassword)

	// Voucher routes — kode boleh dicek semua role (pelanggan menyerahkan kode),
	// tetapi daftar seluruh voucher hanya admin agar kasir tidak bisa memanen kode
	// diskon yang tidak seharusnya dipakai.
	vouchers := protected.Group("/vouchers")
	vouchers.Get("/", middleware.AdminOnly(), h.Voucher.GetAll)
	vouchers.Get("/code/:code", h.Voucher.GetByCode)
	vouchers.Get("/:id", middleware.AdminOnly(), h.Voucher.GetByID)
	// Write operations: admin only
	protected.Post("/vouchers", middleware.AdminOnly(), h.Voucher.Create)
	protected.Put("/vouchers/:id", middleware.AdminOnly(), h.Voucher.Update)
	protected.Patch("/vouchers/:id/toggle", middleware.AdminOnly(), h.Voucher.Toggle)
	protected.Delete("/vouchers/:id", middleware.AdminOnly(), h.Voucher.Delete)

	// Discount rule routes — lihat boleh semua role, edit hanya admin
	discounts := protected.Group("/discounts")
	discounts.Get("/", h.Discount.GetAll)
	discounts.Get("/active", h.Discount.GetActive)
	discounts.Get("/:id", h.Discount.GetByID)
	// Write operations: admin only
	protected.Post("/discounts", middleware.AdminOnly(), h.Discount.Create)
	protected.Put("/discounts/:id", middleware.AdminOnly(), h.Discount.Update)
	protected.Patch("/discounts/:id/toggle", middleware.AdminOnly(), h.Discount.Toggle)
	protected.Delete("/discounts/:id", middleware.AdminOnly(), h.Discount.Delete)

	// Menu routes — semua role bisa kelola (create, edit, delete)
	menus := protected.Group("/menus")
	menus.Get("/", h.Menu.GetAll)
	menus.Get("/available", h.Menu.GetAvailable)
	menus.Get("/category/:category", h.Menu.GetByCategory)
	menus.Get("/:id", h.Menu.GetByID)
	// Harga menu memengaruhi pendapatan makanan: hanya admin yang boleh
	// membuat/mengubah/menghapus menu dan resepnya.
	menus.Post("/", middleware.AdminOnly(), h.Menu.Create)
	menus.Put("/:id", middleware.AdminOnly(), h.Menu.Update)
	menus.Patch("/:id/toggle", middleware.AdminOnly(), h.Menu.Toggle)
	menus.Delete("/:id", middleware.AdminOnly(), h.Menu.Delete)
	menus.Put("/:id/ingredients", middleware.AdminOnly(), h.Ingredient.SetMenuIngredients)
	menus.Get("/:id/ingredients", h.Ingredient.ListMenuIngredients)

	// Ingredient routes — bahan, resep, batch stok
	ingredients := protected.Group("/ingredients")
	ingredients.Get("/", h.Ingredient.GetAll)
	ingredients.Get("/expired", h.Ingredient.GetExpired)    // HARUS sebelum /:id
	ingredients.Get("/stock-log", h.Ingredient.GetStockLog) // HARUS sebelum /:id
	ingredients.Post("/", middleware.AdminOnly(), h.Ingredient.Create)
	ingredients.Get("/:id", h.Ingredient.GetByID)
	ingredients.Put("/:id", middleware.AdminOnly(), h.Ingredient.Update)
	ingredients.Delete("/:id", middleware.AdminOnly(), h.Ingredient.Delete)
	ingredients.Post("/:id/stock", middleware.AdminOnly(), h.Ingredient.AddStock)
	ingredients.Get("/:id/stock", h.Ingredient.ListBatches)
	ingredients.Put("/:id/components", middleware.AdminOnly(), h.Ingredient.SetComponents)
	ingredients.Get("/:id/components", h.Ingredient.ListComponents)

	// Food order routes — semua role bisa buat, lihat, update status & cancel; delete admin only
	foodOrders := protected.Group("/food-orders")
	foodOrders.Get("/", h.FoodOrder.GetAll)
	foodOrders.Get("/status", h.FoodOrder.GetByStatus)
	foodOrders.Get("/:id", h.FoodOrder.GetByID)
	foodOrders.Post("/", h.FoodOrder.Create)
	foodOrders.Post("/price-preview", h.FoodOrder.GetPricePreview)
	foodOrders.Patch("/:id/status", h.FoodOrder.UpdateStatus)
	foodOrders.Patch("/:id/cancel", h.FoodOrder.Cancel)
	foodOrders.Delete("/:id", middleware.AdminOnly(), h.FoodOrder.Delete)

	// Food orders terhubung ke sesi PS
	sessions.Get("/:session_id/food-orders", h.FoodOrder.GetBySession)

	// 404 handler
	app.Use(func(c *fiber.Ctx) error {
		return response.NotFound(c, "Endpoint tidak ditemukan")
	})
}
