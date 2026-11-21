package handler

import (
	"strings"
	"time"

	"byone-arena/internal/delivery/websocket"
	"byone-arena/internal/domain/entity"
	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// notificationIsLoopEligible menentukan notifikasi yang boleh ikut loop
// otomatis. Notifikasi manual (autoShow=false) tidak pernah ikut loop karena
// hanya boleh tampil saat admin menekan "Tampilkan".
func notificationIsLoopEligible(n *entity.TvNotification) bool {
	return n != nil && n.IsActive && n.AutoShow && n.LoopEnabled
}

// normalizeNotificationRequest menyelaraskan input form dengan aturan bisnis.
// Dipakai oleh create DAN update agar keduanya tidak pernah menyimpan nilai
// yang tidak valid (mis. priority kosong atau interval 0).
func normalizeNotificationRequest(req *usecase.CreateNotificationRequest) {
	req.Title = strings.TrimSpace(req.Title)
	req.Message = strings.TrimSpace(req.Message)
	if req.ImageURL != nil {
		trimmed := strings.TrimSpace(*req.ImageURL)
		if trimmed == "" {
			req.ImageURL = nil
		} else {
			req.ImageURL = &trimmed
		}
	}
	switch req.Priority {
	case entity.NotificationPriorityLow,
		entity.NotificationPriorityNormal,
		entity.NotificationPriorityHigh:
		// sudah valid
	default:
		req.Priority = entity.NotificationPriorityNormal
	}
	if req.LoopInterval < 5 || req.LoopInterval > 3600 {
		req.LoopInterval = 30
	}
	// Notifikasi manual tidak boleh looping: hanya tampil lewat tombol
	// "Tampilkan", sehingga loop-nya tidak akan pernah terlihat.
	if !req.AutoShow {
		req.LoopEnabled = false
	}
}

// validateNotificationTarget memastikan notifikasi bertarget selalu punya
// minimal satu konsol. Tanpa ini, notifikasi bisa tersimpan tetapi tidak
// pernah terkirim ke TV mana pun.
func validateNotificationTarget(req *usecase.CreateNotificationRequest) string {
	if req.TargetAll {
		return ""
	}
	if len(req.ConsoleIDs) == 0 {
		return "Pilih minimal satu konsol atau aktifkan 'Semua Konsol'"
	}
	return ""
}

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

// reconcileNotificationLoop menjaga status loop tetap konsisten dengan data:
// mulai saat ada notifikasi loop pertama yang aktif, dan berhenti otomatis
// ketika tidak ada lagi notifikasi loop yang eligible. Loop yang sengaja
// dihentikan admin TIDAK dinyalakan ulang oleh edit biasa.
func (h *NotificationHandler) reconcileNotificationLoop(wasEligible, nowEligible bool) {
	if nowEligible && !wasEligible {
		h.hub.StartNotificationLoop()
		return
	}
	if wasEligible && !nowEligible {
		h.stopLoopIfNoEligibleNotification()
	}
}

// stopLoopIfNoEligibleNotification menghentikan loop hanya jika benar-benar
// tidak ada notifikasi loop aktif yang tersisa.
func (h *NotificationHandler) stopLoopIfNoEligibleNotification() {
	var count int64
	h.db.Model(&entity.TvNotification{}).
		Where("loop_enabled = ? AND is_active = ? AND auto_show = ?", true, true, true).
		Count(&count)
	if count == 0 {
		h.hub.StopNotificationLoop()
	}
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
	if err := query.Find(&list).Error; err != nil {
		return response.InternalServerError(c, "Gagal mengambil data notifikasi")
	}
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
	normalizeNotificationRequest(req)
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}
	if msg := validateNotificationTarget(req); msg != "" {
		return response.BadRequest(c, msg)
	}

	targetConsoleIDs := req.ConsoleIDs
	if req.TargetAll {
		// Notifikasi global tidak perlu menyimpan target lama.
		targetConsoleIDs = []string{}
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
		TargetConsoleIDs:   targetConsoleIDs,
		ActiveSessionsOnly: req.ActiveSessionsOnly,
		AutoShow:           req.AutoShow,
		// Eksplisit true supaya response create tidak bergantung pada
		// default kolom DB (sebelumnya response bisa tampil "Nonaktif").
		IsActive: true,
	}

	if err := h.db.Create(n).Error; err != nil {
		return response.InternalServerError(c, "Gagal membuat notifikasi")
	}
	// Notifikasi otomatis non-loop langsung dikirim sekali. Notifikasi loop
	// akan dikirim oleh loop (maksimal 1 detik kemudian), sehingga tidak
	// dikirim dua kali berturut-turut saat dibuat.
	if n.AutoShow && !n.LoopEnabled {
		h.hub.BroadcastTVNotification(n)
	}
	// Loop hanya dimulai untuk notifikasi yang benar-benar eligible.
	if notificationIsLoopEligible(n) && !h.hub.IsNotificationRunning() {
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
	normalizeNotificationRequest(req)
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}
	if msg := validateNotificationTarget(req); msg != "" {
		return response.BadRequest(c, msg)
	}

	wasLoopEligible := notificationIsLoopEligible(&n)

	n.Title = req.Title
	n.Message = req.Message
	n.ImageURL = req.ImageURL
	n.Priority = req.Priority
	n.LoopEnabled = req.LoopEnabled
	n.LoopInterval = req.LoopInterval
	n.TargetAll = req.TargetAll
	n.ActiveSessionsOnly = req.ActiveSessionsOnly
	n.AutoShow = req.AutoShow
	n.UpdatedAt = time.Now()
	if req.TargetAll {
		n.TargetConsoleIDs = entity.JSONBArray{}
	} else {
		n.TargetConsoleIDs = req.ConsoleIDs
	}

	// Jangan pernah menulis is_active dari form edit. Status aktif hanya
	// diubah lewat endpoint toggle / tombol Tampilkan.
	updates := map[string]interface{}{
		"title":                n.Title,
		"message":              n.Message,
		"image_url":            n.ImageURL,
		"priority":             n.Priority,
		"loop_enabled":         n.LoopEnabled,
		"loop_interval":        n.LoopInterval,
		"target_all":           n.TargetAll,
		"target_console_ids":   n.TargetConsoleIDs,
		"active_sessions_only": n.ActiveSessionsOnly,
		"auto_show":            n.AutoShow,
		"updated_at":           n.UpdatedAt,
	}
	if err := h.db.Model(&entity.TvNotification{}).
		Where("id = ?", id).
		Updates(updates).Error; err != nil {
		return response.InternalServerError(c, "Gagal update notifikasi")
	}

	h.reconcileNotificationLoop(wasLoopEligible, notificationIsLoopEligible(&n))
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
	var n entity.TvNotification
	if result := h.db.First(&n, "id = ?", id); result.Error != nil {
		return response.NotFound(c, "Notifikasi tidak ditemukan")
	}
	wasLoopEligible := notificationIsLoopEligible(&n)

	result := h.db.Delete(&entity.TvNotification{}, "id = ?", id)
	if result.Error != nil {
		return response.InternalServerError(c, "Gagal menghapus notifikasi")
	}
	if result.RowsAffected == 0 {
		return response.NotFound(c, "Notifikasi tidak ditemukan")
	}
	// Jika yang dihapus adalah satu-satunya notifikasi loop, hentikan loop
	// supaya UI tidak menampilkan "Looping berjalan" tanpa isi.
	if wasLoopEligible {
		h.stopLoopIfNoEligibleNotification()
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
	wasLoopEligible := notificationIsLoopEligible(&n)

	now := time.Now()
	n.IsActive = !n.IsActive
	n.UpdatedAt = now
	if err := h.db.Model(&entity.TvNotification{}).
		Where("id = ?", n.ID).
		Updates(map[string]interface{}{
			"is_active":  n.IsActive,
			"updated_at": now,
		}).Error; err != nil {
		return response.InternalServerError(c, "Gagal mengubah status notifikasi")
	}
	h.reconcileNotificationLoop(wasLoopEligible, notificationIsLoopEligible(&n))
	return response.OK(c, "Status notifikasi diubah", n)
}

// ShowNotification godoc
// @Summary      Tampilkan notifikasi sekarang (manual)
// @Description  Memicu notifikasi manual agar muncul sekali di TV sesuai target.
// Hanya berlaku untuk notifikasi yang sedang aktif. Klien menyimpan waktu
// trigger ini sehingga pesan tidak muncul berulang.
// @Tags         Notifikasi TV
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Notification ID"
// @Success      200  {object}  response.Response{data=entity.TvNotification}
// @Router       /api/v1/notifications/{id}/show [post]
func (h *NotificationHandler) ShowNotification(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	var n entity.TvNotification
	if result := h.db.First(&n, "id = ?", id); result.Error != nil {
		return response.NotFound(c, "Notifikasi tidak ditemukan")
	}
	if !n.IsActive {
		return response.BadRequest(c,
			"Notifikasi sedang nonaktif. Aktifkan notifikasi terlebih dahulu untuk menampilkannya")
	}
	if !n.TargetAll && len(n.TargetConsoleIDs) == 0 {
		return response.BadRequest(c,
			"Notifikasi tidak memiliki target konsol. Edit dan pilih konsol tujuan terlebih dahulu")
	}

	now := time.Now()
	n.LastTriggeredAt = &now
	n.UpdatedAt = now
	if err := h.db.Model(&entity.TvNotification{}).
		Where("id = ?", n.ID).
		Updates(map[string]interface{}{
			"last_triggered_at": now,
			"updated_at":        now,
		}).Error; err != nil {
		return response.InternalServerError(c, "Gagal menampilkan notifikasi")
	}

	// Trigger realtime: client langsung poll dan menampilkan notifikasi ini
	// sekali, tanpa menunggu interval polling berikutnya.
	h.hub.BroadcastTVNotification(n)
	return response.OK(c, "Notifikasi ditampilkan", n)
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
	h.db.Model(&entity.TvNotification{}).
		Where("loop_enabled = ? AND is_active = ? AND auto_show = ?", true, true, true).
		Count(&activeCount)
	return response.OK(c, "Status loop notifikasi", fiber.Map{
		"running":                 running,
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

	h.hub.BroadcastTVWake(id, console.IPAddress)

	// Update status layar + tandai live manual tanpa sesi. Saat sesi
	// dimulai, StartSession akan mengembalikan manual_live=false.
	h.db.Model(&entity.Console{}).Where("id = ?", id).Updates(map[string]interface{}{
		"screen_status": entity.ScreenStatusOn,
		"manual_live":   true,
	})

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

	h.hub.BroadcastTVSleep(id, console.IPAddress)

	h.db.Model(&entity.Console{}).Where("id = ?", id).Updates(map[string]interface{}{
		"screen_status": entity.ScreenStatusOff,
		"manual_live":   false,
	})

	return response.OK(c, "Perintah sleep dikirim ke TV", nil)
}

// ScreensaverConsole godoc
// @Summary      Tampilkan Screensaver Byone Arena di TV
// @Description  Memerintahkan TV client untuk kembali menampilkan layar Byone Arena (screensaver/launcher), tanpa mengubah status layar fisik. Dipakai admin untuk memaksa TV kembali ke aplikasi, atau otomatis saat sesi berakhir/waktu habis.
// @Tags         Kontrol TV
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Console ID"
// @Success      200  {object}  response.Response
// @Router       /api/v1/consoles/{id}/screensaver [post]
func (h *NotificationHandler) ScreensaverConsole(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	if _, err := h.consoleUC.GetConsoleByID(c.Context(), id); err != nil {
		return response.NotFound(c, "Konsol tidak ditemukan")
	}

	// Kembali ke screensaver = keluar dari live manual.
	h.db.Model(&entity.Console{}).Where("id = ?", id).Updates(map[string]interface{}{
		"screen_status": entity.ScreenStatusOff,
		"manual_live":   false,
	})
	h.hub.BroadcastTVScreensaver(id)

	return response.OK(c, "Perintah screensaver dikirim ke TV", nil)
}
