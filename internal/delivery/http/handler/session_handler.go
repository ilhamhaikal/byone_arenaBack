package handler

import (
	"byone-arena/internal/delivery/websocket"
	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// SessionHandler menangani HTTP request untuk manajemen sesi rental
type SessionHandler struct {
	sessionUC usecase.SessionUseCase
	validator *validator.Validator
	hub       *websocket.Hub
}

// NewSessionHandler membuat instance baru SessionHandler
func NewSessionHandler(sessionUC usecase.SessionUseCase, v *validator.Validator, hub *websocket.Hub) *SessionHandler {
	return &SessionHandler{sessionUC: sessionUC, validator: v, hub: hub}
}

// GetAll godoc
// @Summary      Ambil semua sesi rental
// @Description  Mengembalikan daftar seluruh sesi rental (aktif, selesai, dibatalkan)
// @Tags         Sesi Rental
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]entity.Session}
// @Failure      401  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /api/v1/sessions [get]
func (h *SessionHandler) GetAll(c *fiber.Ctx) error {
	sessions, err := h.sessionUC.GetAllSessions(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil data sesi")
	}
	return response.OK(c, "Data sesi berhasil diambil", sessions)
}

// GetActive godoc
// @Summary      Ambil semua sesi aktif
// @Description  Mengembalikan sesi yang sedang berjalan saat ini di semua konsol
// @Tags         Sesi Rental
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]entity.Session}
// @Failure      401  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /api/v1/sessions/active [get]
func (h *SessionHandler) GetActive(c *fiber.Ctx) error {
	sessions, err := h.sessionUC.GetActiveSessions(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil data sesi aktif")
	}
	return response.OK(c, "Sesi aktif berhasil diambil", sessions)
}

// GetByID godoc
// @Summary      Ambil sesi berdasarkan ID
// @Description  Mengembalikan detail satu sesi rental beserta data konsol dan pelanggan
// @Tags         Sesi Rental
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Session ID (UUID)"
// @Success      200  {object}  response.Response{data=entity.Session}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/sessions/{id} [get]
func (h *SessionHandler) GetByID(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	session, err := h.sessionUC.GetSessionByID(c.Context(), id)
	if err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.OK(c, "Data sesi berhasil diambil", session)
}

// Start godoc
// @Summary      Mulai sesi rental baru
// @Description  Memulai sesi rental untuk konsol tertentu. Konsol harus dalam status available. Event realtime dikirim ke semua client yang terhubung.
// @Tags         Sesi Rental
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      usecase.StartSessionRequest  true  "Data sesi baru"
// @Success      201   {object}  response.Response{data=entity.Session}
// @Failure      400   {object}  response.ErrorResponse  "Konsol tidak tersedia atau request tidak valid"
// @Failure      401   {object}  response.ErrorResponse
// @Failure      500   {object}  response.ErrorResponse
// @Router       /api/v1/sessions/start [post]
func (h *SessionHandler) Start(c *fiber.Ctx) error {
	req := new(usecase.StartSessionRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	session, err := h.sessionUC.StartSession(c.Context(), req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	h.hub.Broadcast(websocket.NewEvent(websocket.EventSessionStarted, session))

	return response.Created(c, "Sesi rental berhasil dimulai", session)
}

// End godoc
// @Summary      Akhiri sesi rental
// @Description  Mengakhiri sesi rental aktif, menghitung durasi dan total harga, membebaskan konsol. Event realtime dikirim.
// @Tags         Sesi Rental
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Session ID (UUID)"
// @Success      200  {object}  response.Response{data=entity.Session}
// @Failure      400  {object}  response.ErrorResponse  "Sesi tidak aktif atau tidak ditemukan"
// @Failure      401  {object}  response.ErrorResponse
// @Router       /api/v1/sessions/{id}/end [patch]
func (h *SessionHandler) End(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	session, err := h.sessionUC.EndSession(c.Context(), id)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	h.hub.Broadcast(websocket.NewEvent(websocket.EventSessionEnded, session))

	return response.OK(c, "Sesi rental berhasil diakhiri", session)
}

// Cancel godoc
// @Summary      Batalkan sesi rental
// @Description  Membatalkan sesi rental aktif tanpa tagihan. Konsol dikembalikan ke status available. Event realtime dikirim.
// @Tags         Sesi Rental
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Session ID (UUID)"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.ErrorResponse  "Sesi tidak aktif atau tidak ditemukan"
// @Failure      401  {object}  response.ErrorResponse
// @Router       /api/v1/sessions/{id}/cancel [patch]
func (h *SessionHandler) Cancel(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	if err := h.sessionUC.CancelSession(c.Context(), id); err != nil {
		return response.BadRequest(c, err.Error())
	}

	h.hub.Broadcast(websocket.NewEvent(websocket.EventSessionCancelled, fiber.Map{"session_id": id}))

	return response.OK(c, "Sesi rental berhasil dibatalkan", nil)
}
