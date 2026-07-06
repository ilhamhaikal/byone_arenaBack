package handler

import (
	"byone-arena/internal/delivery/websocket"
	"byone-arena/internal/domain/entity"
	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NotificationHandler menangani notifikasi TV dan kontrol TV
type NotificationHandler struct {
	db        *gorm.DB
	hub       *websocket.Hub
	consoleUC usecase.ConsoleUseCase
	validator *validator.Validator
}

// NewNotificationHandler membuat instance baru
func NewNotificationHandler(db *gorm.DB, hub *websocket.Hub, consoleUC usecase.ConsoleUseCase, v *validator.Validator) *NotificationHandler {
	return &NotificationHandler{db: db, hub: hub, consoleUC: consoleUC, validator: v}
}

// --- NOTIFICATION CRUD ---

// GetAllNotifications godoc
// @Summary      Ambil semua notifikasi (PUBLIK)
// @Description  Endpoint publik untuk TV client mengambil notifikasi aktif. Gunakan query `?active=true` untuk hanya notifikasi yang sedang aktif.
// @Tags         Notifikasi TV
// @Produce      json
// @Param        active  query     bool  false  "Hanya notifikasi aktif"
// @Success      200     {object}  response.Response{data=[]entity.TvNotification}
// @Router       /api/v1/notifications [get]
func (h *NotificationHandler) GetAllNotifications(c *fiber.Ctx) error {
	var list []entity.TvNotification
	query := h.db.Order("created_at DESC")
	if c.Query("active") == "true" {
		query = query.Where("is_active = ?", true)
	}
	query.Find(&list)
	return response.OK(c, "Data notifikasi berhasil diambil", list)
}

// CreateNotification godoc
// @Summary      Buat notifikasi baru
// @Tags         Notifikasi TV
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      usecase.CreateNotificationRequest  true  "Data notifikasi"
// @Success      201   {object}  response.Response{data=entity.TvNotification}
// @Router       /api/v1/notifications [post]
func (h *NotificationHandler) CreateNotification(c *fiber.Ctx) error {
	req := new(usecase.CreateNotificationRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	n := &entity.TvNotification{
		ID:                 uuid.New(),
		Title:              req.Title,
		Message:            req.Message,
		ImageURL:           req.ImageURL,
		Priority:           req.Priority,
		LoopEnabled:        req.LoopEnabled,
		LoopInterval:       req.LoopInterval,
		TargetAll:          req.TargetAll,
		TargetConsoleIDs:   req.ConsoleIDs,
		ActiveSessionsOnly: req.ActiveSessionsOnly,
	}

	if err := h.db.Create(n).Error; err != nil {
		return response.InternalServerError(c, "Gagal membuat notifikasi")
	}
	// Kirim langsung ke client jika tidak loop, atau jika loop aktif
	if !req.LoopEnabled {
		h.hub.Broadcast(websocket.NewEvent(websocket.EventTVNotification, n))
	}
	// Jika loopEnabled, notification loop akan otomatis mengirimkannya
	if req.LoopEnabled && !h.hub.IsNotificationRunning() {
		h.hub.StartNotificationLoop()
	}
	return response.Created(c, "Notifikasi berhasil dibuat", n)
}

// UpdateNotification godoc
// @Summary      Update notifikasi
// @Tags         Notifikasi TV
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string  true  "Notification ID"
// @Param        body  body      usecase.CreateNotificationRequest  true  "Data notifikasi"
// @Success      200   {object}  response.Response{data=entity.TvNotification}
// @Router       /api/v1/notifications/{id} [put]
func (h *NotificationHandler) UpdateNotification(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	var n entity.TvNotification
	if result := h.db.First(&n, "id = ?", id); result.Error != nil {
		return response.NotFound(c, "Notifikasi tidak ditemukan")
	}

	req := new(usecase.CreateNotificationRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}

	n.Title = req.Title
	n.Message = req.Message
	n.ImageURL = req.ImageURL
	n.Priority = req.Priority
	n.LoopEnabled = req.LoopEnabled
	n.LoopInterval = req.LoopInterval
	n.TargetAll = req.TargetAll
	n.TargetConsoleIDs = req.ConsoleIDs
	n.ActiveSessionsOnly = req.ActiveSessionsOnly

	if err := h.db.Save(&n).Error; err != nil {
		return response.InternalServerError(c, "Gagal update notifikasi")
	}

	// Auto-start loop jika di-enable
	if n.LoopEnabled && n.IsActive && !h.hub.IsNotificationRunning() {
		h.hub.StartNotificationLoop()
	}
	return response.OK(c, "Notifikasi berhasil diupdate", n)
}

// DeleteNotification godoc
// @Summary      Hapus notifikasi
// @Tags         Notifikasi TV
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Notification ID"
// @Success      200  {object}  response.Response
// @Router       /api/v1/notifications/{id} [delete]
func (h *NotificationHandler) DeleteNotification(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	if err := h.db.Delete(&entity.TvNotification{}, "id = ?", id).Error; err != nil {
		return response.InternalServerError(c, "Gagal menghapus notifikasi")
	}
	return response.OK(c, "Notifikasi berhasil dihapus", nil)
}

// ToggleNotification godoc
// @Summary      Aktif/nonaktifkan notifikasi
// @Tags         Notifikasi TV
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Notification ID"
// @Success      200  {object}  response.Response{data=entity.TvNotification}
// @Router       /api/v1/notifications/{id}/toggle [patch]
func (h *NotificationHandler) ToggleNotification(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	var n entity.TvNotification
	if result := h.db.First(&n, "id = ?", id); result.Error != nil {
		return response.NotFound(c, "Notifikasi tidak ditemukan")
	}
	n.IsActive = !n.IsActive
	h.db.Save(&n)
	return response.OK(c, "Status notifikasi diubah", n)
}

// --- LOOP CONTROL ---

// StartLoop godoc
// @Summary      Mulai loop notifikasi
// @Tags         Notifikasi TV
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response
// @Router       /api/v1/notifications/loop/start [post]
func (h *NotificationHandler) StartLoop(c *fiber.Ctx) error {
	h.hub.StartNotificationLoop()
	return response.OK(c, "Loop notifikasi dimulai", fiber.Map{"running": true})
}

// StopLoop godoc
// @Summary      Hentikan loop notifikasi
// @Tags         Notifikasi TV
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response
// @Router       /api/v1/notifications/loop/stop [post]
func (h *NotificationHandler) StopLoop(c *fiber.Ctx) error {
	h.hub.StopNotificationLoop()
	return response.OK(c, "Loop notifikasi dihentikan", fiber.Map{"running": false})
}

// LoopStatus godoc
// @Summary      Cek status loop notifikasi
// @Tags         Notifikasi TV
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /api/v1/notifications/loop/status [get]
func (h *NotificationHandler) LoopStatus(c *fiber.Ctx) error {
	running := h.hub.IsNotificationRunning()
	var activeCount int64
	h.db.Model(&entity.TvNotification{}).Where("loop_enabled = ? AND is_active = ?", true, true).Count(&activeCount)
	return response.OK(c, "Status loop notifikasi", fiber.Map{
		"running":           running,
		"activeLoopNotifications": activeCount,
	})
}

// --- TV CONTROL ---

// WakeConsole godoc
// @Summary      Nyalakan TV (kirim event wake)
// @Tags         Kontrol TV
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Console ID"
// @Success      200  {object}  response.Response
// @Router       /api/v1/consoles/{id}/wake [post]
func (h *NotificationHandler) WakeConsole(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	console, err := h.consoleUC.GetConsoleByID(c.Context(), id)
	if err != nil {
		return response.NotFound(c, "Konsol tidak ditemukan")
	}

	h.hub.Broadcast(websocket.NewEvent(websocket.EventTVWake, fiber.Map{
		"consoleId": id,
		"ipAddress": console.IPAddress,
	}))

	// Update status layar
	h.db.Model(&entity.Console{}).Where("id = ?", id).Update("screen_status", entity.ScreenStatusOn)

	return response.OK(c, "Perintah wake dikirim ke TV", nil)
}

// SleepConsole godoc
// @Summary      Matikan/sleep TV
// @Tags         Kontrol TV
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Console ID"
// @Success      200  {object}  response.Response
// @Router       /api/v1/consoles/{id}/sleep [post]
func (h *NotificationHandler) SleepConsole(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	console, err := h.consoleUC.GetConsoleByID(c.Context(), id)
	if err != nil {
		return response.NotFound(c, "Konsol tidak ditemukan")
	}

	h.hub.Broadcast(websocket.NewEvent(websocket.EventTVSleep, fiber.Map{
		"consoleId": id,
		"ipAddress": console.IPAddress,
	}))

	h.db.Model(&entity.Console{}).Where("id = ?", id).Update("screen_status", entity.ScreenStatusScreensaver)

	return response.OK(c, "Perintah sleep dikirim ke TV", nil)
}
