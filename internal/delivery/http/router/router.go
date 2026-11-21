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
	"github.com/google/uuid"
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
	License       *handler.LicenseHandler
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
			// Terlihat jelas instance ini memakai environment & DB yang mana.
			"env": cfg.AppEnv,
			"db":  cfg.DBName,
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

	// WebSocket endpoint — wajib secret perangkat TV.
	app.Get("/ws",
		limiter.New(limiter.Config{
			Max:        10,
			Expiration: 1 * time.Minute,
			KeyGenerator: func(c *fiber.Ctx) string {
				return c.IP()
			},
			LimitReached: func(c *fiber.Ctx) error {
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
					"success": false,
					"message": "Terlalu banyak koneksi WebSocket",
				})
			},
		}),
		middleware.DeviceWSMiddleware(db),
		websocket.New(func(conn *websocket.Conn) {
			consoleID, _ := conn.Locals("consoleID").(uuid.UUID)
			h.Hub.HandleConnection(conn, consoleID)
		}),
	)

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

	// Heartbeat — publik tetapi wajib secret perangkat (lihat handler).
	api.Post("/consoles/:id/heartbeat", limiter.New(limiter.Config{
		Max:        30,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP() + ":" + c.Params("id")
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"success": false,
				"message": "Terlalu banyak heartbeat, coba lagi nanti",
			})
		},
	}), h.Console.Heartbeat)

	// Pairing TV: publik, dipanggil aplikasi TV setelah menemukan server.
	api.Post("/devices/announce", limiter.New(limiter.Config{
		Max:        30,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"success": false,
				"message": "Terlalu banyak permintaan pairing, coba lagi nanti",
			})
		},
	}), h.Device.Announce)

	// Promo bonus yang berlaku hari ini — publik (dipakai TV & dialog sesi).
	api.Get("/promo-bonuses/active", h.Promo.Active)

	// Routes publik lain — HARUS sebelum protected group
	api.Get("/notifications", h.Notify.GetAllNotifications)
	api.Get("/notifications/loop/status", h.Notify.LoopStatus)

	// Settings publik (dibaca tanpa login) — HARUS sebelum protected group,
	// kalau tidak akan ikut terkena middleware auth (401) dan harga terbaca 0.
	api.Get("/settings/membership", h.Settings.GetMembershipPrice)
	api.Get("/settings/daily-price", h.Settings.GetDailyPrice)

	// Protected routes (memerlukan JWT)
	protected := api.Group("",
		middleware.AuthMiddleware(cfg, db),
		middleware.RequirePasswordChanged(),
	)

	// Profil pemilik token — dipakai frontend untuk memvalidasi sesi.
	protected.Get("/auth/me", h.Auth.Me)
	protected.Post("/auth/logout", h.Auth.Logout)

	// Audit trail aksi sensitif — admin & superadmin.
	protected.Get("/audit-logs", middleware.AdminOnly(), h.Audit.List)

	// Dashboard summary — memerlukan autentikasi
	protected.Get("/dashboard/summary", h.Dashboard.GetSummary)
	protected.Get("/dashboard/revenue-chart", h.Dashboard.GetRevenueChart)

	// Report routes — butuh akses menu Laporan
	reports := protected.Group("/reports", middleware.RequirePermission("report"))
	reports.Get("/summary", h.Report.GetSummary)
	reports.Get("/staff", middleware.AdminOnly(), h.Report.GetStaffReport)
	reports.Get("/promo-bonus", h.Promo.PromoReport)

	// Capital routes (pembukuan modal awal) — admin & superadmin only
	capital := protected.Group("/capital", middleware.AdminOnly())
	capital.Get("/", h.Capital.ListEntries)
	capital.Post("/", h.Capital.AddEntry)
	capital.Delete("/:id", h.Capital.DeleteEntry)
	capital.Get("/monthly", h.Capital.MonthlyReport)

	// Rental Package routes (paket begadang dll) — lihat butuh menu Rental,
	// kelola butuh izin rental.write / rental.delete.
	packages := protected.Group("/rental-packages", middleware.RequirePermission("rental"))
	packages.Get("/", h.RentalPackage.List)
	packages.Post("/", middleware.RequireWrite("rental"), h.RentalPackage.Save)
	packages.Delete("/:id", middleware.RequireDelete("rental"), h.RentalPackage.Delete)
	// Mulai sesi dengan paket
	protected.Post("/sessions/start-package",
		middleware.RequireWrite("rental"), h.RentalPackage.Start)

	// Activity routes — realtime activity feed
	activities := protected.Group("/activities")
	activities.Get("/recent", h.Activity.GetRecentActivities)

	// Notification routes — lihat publik, kelola butuh izin notif.write/delete
	notifications := protected.Group("/notifications")
	notifications.Post("/", middleware.RequireWrite("notif"), h.Notify.CreateNotification)
	notifications.Put("/:id", middleware.RequireWrite("notif"), h.Notify.UpdateNotification)
	notifications.Delete("/:id", middleware.RequireDelete("notif"), h.Notify.DeleteNotification)
	notifications.Patch("/:id/toggle", middleware.RequireWrite("notif"), h.Notify.ToggleNotification)
	notifications.Post("/loop/start", middleware.RequireWrite("notif"), h.Notify.StartLoop)
	notifications.Post("/loop/stop", middleware.RequireWrite("notif"), h.Notify.StopLoop)
	notifications.Post("/:id/show", middleware.RequireWrite("notif"), h.Notify.ShowNotification)

	// GET notifications + loop/status HARUS publik — daftar langsung di app, bypass group
	app.Get("/api/v1/notifications", h.Notify.GetAllNotifications)
	app.Get("/api/v1/notifications/loop/status", h.Notify.LoopStatus)

	// Daily Rental routes
	dailyRentals := protected.Group("/daily-rentals", middleware.RequirePermission("rental_harian"))
	dailyRentals.Get("/", h.Rental.GetAllDailyRentals)
	dailyRentals.Post("/", middleware.RequireWrite("rental_harian"), h.Rental.CreateDailyRental)
	dailyRentals.Post("/:id/return", middleware.RequireWrite("rental_harian"), h.Rental.ReturnDailyRental)

	// Booking routes
	bookings := protected.Group("/bookings", middleware.RequirePermission("booking"))
	bookings.Get("/", h.Rental.GetAllBookings)
	bookings.Post("/", middleware.RequireWrite("booking"), h.Rental.CreateBooking)
	bookings.Patch("/:id/status", middleware.RequireWrite("booking"), h.Rental.UpdateBookingStatus)

	// Settings routes — GET publik didaftarkan di atas (sebelum protected).
	// PUT hanya untuk superadmin atau user dengan izin khusus.
	protected.Put("/settings/membership",
		middleware.RequirePermission("member_price"), h.Settings.UpdateMembershipPrice)
	protected.Put("/settings/daily-price",
		middleware.RequirePermission("daily_price"), h.Settings.UpdateDailyPrice)

	// Console routes — lihat butuh menu Konsol, ubah butuh console.write.
	consoles := protected.Group("/consoles", middleware.RequirePermission("console"))
	consoles.Get("/", h.Console.GetAll)
	consoles.Get("/available", h.Console.GetAvailable)
	consoles.Get("/overview", h.Console.GetOverview)
	consoles.Get("/:id", h.Console.GetByID)
	// Membuat konsol berarti menentukan harga awal. Selain console.write,
	// wajib izin sensitif console.price agar kasir tidak bisa membuat konsol
	// dengan harga sendiri.
	consoles.Post("/",
		middleware.RequireWrite("console"),
		middleware.RequirePermission("console.price"),
		h.Console.Create,
	)
	// Harga & tarif konsol hanya boleh diubah user dengan izin console.write:
	// mencegah kasir menurunkan harga sebelum memulai sesi.
	consoles.Put("/:id", middleware.RequireWrite("console"), h.Console.Update)
	consoles.Delete("/:id", middleware.RequireDelete("console"), h.Console.Delete)
	consoles.Get("/:id/price", h.Console.PreviewPrice)
	// TV Control
	consoles.Post("/:id/wake", middleware.RequireWrite("console"), h.Notify.WakeConsole)
	consoles.Post("/:id/sleep", middleware.RequireWrite("console"), h.Notify.SleepConsole)
	consoles.Post("/:id/screensaver", middleware.RequireWrite("console"), h.Notify.ScreensaverConsole)
	consoles.Get("/:id/tv-logs", h.Console.GetTvLogs) // log aktivitas TV

	// Pairing perangkat TV — butuh izin console.write
	devices := protected.Group("/devices", middleware.RequireWrite("console"))
	devices.Get("/pending", h.Device.Pending)
	devices.Post("/resolve-code", h.Device.ResolveByCode)
	devices.Post("/bind-by-code", h.Device.BindByCode)
	devices.Delete("/console/:console_id", h.Device.UnbindConsole)
	devices.Post("/:id/bind", h.Device.Bind)
	devices.Delete("/:id", h.Device.Dismiss)

	// Lisensi offline: status boleh dibaca semua user terautentikasi (untuk
	// banner), tetapi aktivasi & riwayat HANYA superadmin/owner.
	protected.Get("/license/status", h.License.Status)
	protected.Get("/license/history", middleware.SuperAdminOnly(), h.License.History)
	protected.Post("/license/activate", middleware.SuperAdminOnly(), h.License.Activate)

	// Promo bonus waktu: lihat butuh menu Bonus Waktu, kelola butuh write/delete.
	promoRules := protected.Group("/promo-bonuses", middleware.RequirePermission("promo_bonus"))
	promoRules.Get("/", h.Promo.List)
	promoRules.Post("/", middleware.RequireWrite("promo_bonus"), h.Promo.Create)
	promoRules.Put("/:id", middleware.RequireWrite("promo_bonus"), h.Promo.Update)
	promoRules.Delete("/:id", middleware.RequireDelete("promo_bonus"), h.Promo.Delete)

	// Session routes — lihat butuh menu Rental, aksi butuh rental.write.
	sessions := protected.Group("/sessions", middleware.RequirePermission("rental"))
	sessions.Get("/", h.Session.GetAll)
	sessions.Get("/active", h.Session.GetActive)
	sessions.Get("/:id", h.Session.GetByID)
	sessions.Post("/start", middleware.RequireWrite("rental"), h.Session.Start)
	sessions.Patch("/:id/end", middleware.RequireWrite("rental"), h.Session.End)
	sessions.Patch("/:id/cancel", middleware.RequireWrite("rental"), h.Session.Cancel)
	sessions.Post("/:id/extend", middleware.RequireWrite("rental"), h.Session.Extend)            // tambah waktu sewa
	sessions.Post("/:id/recover", middleware.AdminOnly(), h.Session.Recover)                     // pulihkan sesi terputus (listrik/offline)
	sessions.Post("/:id/promo-bonus", middleware.RequireWrite("rental"), h.Promo.ApplyToSession) // pakai bonus waktu promo
	sessions.Get("/:session_id/payment", h.Payment.GetBySession)
	sessions.Get("/:session_id/payments", h.Payment.GetAllBySession)                                                           // SEMUA payment sesi (base + tiap extend) + ringkasan total
	sessions.Post("/:session_id/payments/confirm-pending", middleware.RequireWrite("rental"), h.Payment.ConfirmSessionPending) // lunasi semua payment pending sesi ini sekaligus + hitung kembalian gabungan

	// Customer routes — lihat butuh menu Member, ubah/hapus sesuai izin.
	customers := protected.Group("/customers", middleware.RequirePermission("member"))
	customers.Get("/", h.Customer.GetAll)
	customers.Get("/:id", h.Customer.GetByID)
	customers.Post("/", middleware.RequireWrite("member"), h.Customer.Create)
	customers.Put("/:id", middleware.RequireWrite("member"), h.Customer.Update)
	customers.Delete("/:id", middleware.RequireDelete("member"), h.Customer.Delete)
	customers.Post("/:id/membership", middleware.RequireWrite("member"), h.Customer.SellMembership)
	customers.Get("/:id/loyalty", h.Customer.GetLoyalty)

	// Payment routes
	payments := protected.Group("/payments")
	payments.Get("/pending", h.Payment.GetPendingExtensions) // list pending extend payments — HARUS sebelum /:id
	payments.Get("/:id", h.Payment.GetByID)
	payments.Post("/", middleware.RequireWrite("rental"), h.Payment.Create)
	payments.Patch("/:id/refund", middleware.RequireWrite("rental"), h.Payment.Refund)
	payments.Post("/:id/confirm", middleware.RequireWrite("rental"), h.Payment.Confirm)

	// Shift routes — bagian dari Manajemen User.
	shifts := protected.Group("/shifts", middleware.RequirePermission("user_management"))
	shifts.Get("/", h.Shift.GetAll)
	shifts.Get("/:id", h.Shift.GetByID)
	// Jadwal shift menentukan validasi jam kerja kasir: hanya user dengan izin
	// user_management.write yang boleh mengubah, agar kasir tidak bisa memberi
	// shift 24 jam untuk dirinya sendiri.
	shifts.Post("/", middleware.RequireWrite("user_management"), h.Shift.Create)
	shifts.Put("/:id", middleware.RequireWrite("user_management"), h.Shift.Update)
	shifts.Delete("/:id", middleware.RequireDelete("user_management"), h.Shift.Delete)

	// Shifts by user
	protected.Get("/users/:user_id/shifts",
		middleware.RequirePermission("user_management"), h.Shift.GetByUser)

	// User management routes
	// Change password — semua user terautentikasi bisa (HARUS sebelum /:id)
	protected.Patch("/users/change-password", h.User.ChangePassword)

	users := protected.Group("/users", middleware.RequirePermission("user_management"))
	users.Get("/", h.User.GetAll)
	users.Get("/:id", h.User.GetByID)
	users.Post("/", middleware.RequireWrite("user_management"), h.User.Create)
	users.Put("/:id", middleware.RequireWrite("user_management"), h.User.Update)
	users.Delete("/:id", middleware.RequireDelete("user_management"), h.User.Delete)

	// Voucher routes — kode boleh dicek semua role (pelanggan menyerahkan kode),
	// tetapi daftar seluruh voucher hanya admin agar kasir tidak bisa memanen kode
	// diskon yang tidak seharusnya dipakai.
	// Cek kode tetap terbuka untuk semua user terautentikasi (dipakai saat
	// pelanggan menyerahkan kode), tanpa perlu akses menu Voucher. HARUS
	// didaftarkan SEBELUM group /vouchers karena middleware group berlaku
	// untuk seluruh path berprefix sama.
	protected.Get("/vouchers/code/:code", h.Voucher.GetByCode)

	vouchers := protected.Group("/vouchers", middleware.RequirePermission("voucher"))
	vouchers.Get("/", h.Voucher.GetAll)
	vouchers.Get("/:id", h.Voucher.GetByID)
	vouchers.Post("/", middleware.RequireWrite("voucher"), h.Voucher.Create)
	vouchers.Put("/:id", middleware.RequireWrite("voucher"), h.Voucher.Update)
	vouchers.Patch("/:id/toggle", middleware.RequireWrite("voucher"), h.Voucher.Toggle)
	vouchers.Delete("/:id", middleware.RequireDelete("voucher"), h.Voucher.Delete)

	// Discount rule routes — lihat butuh menu Diskon, ubah/hapus sesuai izin.
	discounts := protected.Group("/discounts", middleware.RequirePermission("discount"))
	discounts.Get("/", h.Discount.GetAll)
	discounts.Get("/active", h.Discount.GetActive)
	discounts.Get("/:id", h.Discount.GetByID)
	discounts.Post("/", middleware.RequireWrite("discount"), h.Discount.Create)
	discounts.Put("/:id", middleware.RequireWrite("discount"), h.Discount.Update)
	discounts.Patch("/:id/toggle", middleware.RequireWrite("discount"), h.Discount.Toggle)
	discounts.Delete("/:id", middleware.RequireDelete("discount"), h.Discount.Delete)

	// Menu routes — lihat butuh menu Menu, ubah/hapus sesuai izin.
	menus := protected.Group("/menus", middleware.RequirePermission("menu"))
	menus.Get("/", h.Menu.GetAll)
	menus.Get("/available", h.Menu.GetAvailable)
	menus.Get("/category/:category", h.Menu.GetByCategory)
	menus.Get("/:id", h.Menu.GetByID)
	// Harga menu memengaruhi pendapatan makanan: butuh izin menu.write/delete.
	menus.Post("/", middleware.RequireWrite("menu"), h.Menu.Create)
	menus.Put("/:id", middleware.RequireWrite("menu"), h.Menu.Update)
	menus.Patch("/:id/toggle", middleware.RequireWrite("menu"), h.Menu.Toggle)
	menus.Delete("/:id", middleware.RequireDelete("menu"), h.Menu.Delete)
	menus.Put("/:id/ingredients", middleware.RequireWrite("menu"), h.Ingredient.SetMenuIngredients)
	menus.Get("/:id/ingredients", h.Ingredient.ListMenuIngredients)

	// Ingredient routes — bahan, resep, batch stok (di bawah izin menu).
	ingredients := protected.Group("/ingredients", middleware.RequirePermission("menu"))
	ingredients.Get("/", h.Ingredient.GetAll)
	ingredients.Get("/expired", h.Ingredient.GetExpired)    // HARUS sebelum /:id
	ingredients.Get("/stock-log", h.Ingredient.GetStockLog) // HARUS sebelum /:id
	ingredients.Post("/", middleware.RequireWrite("menu"), h.Ingredient.Create)
	ingredients.Get("/:id", h.Ingredient.GetByID)
	ingredients.Put("/:id", middleware.RequireWrite("menu"), h.Ingredient.Update)
	ingredients.Delete("/:id", middleware.RequireDelete("menu"), h.Ingredient.Delete)
	ingredients.Post("/:id/stock", middleware.RequireWrite("menu"), h.Ingredient.AddStock)
	ingredients.Get("/:id/stock", h.Ingredient.ListBatches)
	ingredients.Put("/:id/components", middleware.RequireWrite("menu"), h.Ingredient.SetComponents)
	ingredients.Get("/:id/components", h.Ingredient.ListComponents)

	// Food order routes — lihat butuh menu Pesanan, aksi sesuai izin.
	foodOrders := protected.Group("/food-orders", middleware.RequirePermission("food_order"))
	foodOrders.Get("/", h.FoodOrder.GetAll)
	foodOrders.Get("/status", h.FoodOrder.GetByStatus)
	foodOrders.Get("/:id", h.FoodOrder.GetByID)
	foodOrders.Post("/", middleware.RequireWrite("food_order"), h.FoodOrder.Create)
	foodOrders.Post("/price-preview", middleware.RequireWrite("food_order"), h.FoodOrder.GetPricePreview)
	foodOrders.Patch("/:id/status", middleware.RequireWrite("food_order"), h.FoodOrder.UpdateStatus)
	foodOrders.Patch("/:id/cancel", middleware.RequireWrite("food_order"), h.FoodOrder.Cancel)
	foodOrders.Delete("/:id", middleware.RequireDelete("food_order"), h.FoodOrder.Delete)

	// Food orders terhubung ke sesi PS
	sessions.Get("/:session_id/food-orders", h.FoodOrder.GetBySession)

	// 404 handler
	app.Use(func(c *fiber.Ctx) error {
		return response.NotFound(c, "Endpoint tidak ditemukan")
	})
}
