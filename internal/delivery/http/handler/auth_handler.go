package handler

import (
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuthHandler menangani HTTP request untuk autentikasi
type AuthHandler struct {
	authUC    usecase.AuthUseCase
	validator *validator.Validator
	db        *gorm.DB
}

// NewAuthHandler membuat instance baru AuthHandler
func NewAuthHandler(authUC usecase.AuthUseCase, v *validator.Validator, db *gorm.DB) *AuthHandler {
	return &AuthHandler{authUC: authUC, validator: v, db: db}
}

// Me godoc
// @Summary      Profil pemilik token
// @Description  Dipakai frontend untuk memastikan token masih valid dan mengambil role terbaru.
// @Tags         Autentikasi
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=entity.User}
// @Failure      401  {object}  response.ErrorResponse
// @Router       /api/v1/auth/me [get]
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	uid, ok := c.Locals("userID").(uuid.UUID)
	if !ok {
		return response.Unauthorized(c, "Tidak terautentikasi")
	}
	var user entity.User
	if err := h.db.WithContext(c.Context()).First(&user, "id = ?", uid).Error; err != nil {
		return response.Unauthorized(c, "Sesi tidak valid, silakan login ulang")
	}
	if !user.IsActive {
		return response.Unauthorized(c, "Akun tidak aktif, hubungi administrator")
	}
	return response.OK(c, "Profil pengguna", user)
}

// Login godoc
// @Summary      Login pengguna
// @Description  Autentikasi pengguna (admin, superadmin, kasir). Kasir hanya bisa login sesuai jadwal shift aktif.
// @Tags         Autentikasi
// @Accept       json
// @Produce      json
// @Param        body  body      usecase.LoginRequest  true  "Kredensial login"
// @Success      200   {object}  response.Response{data=usecase.LoginResponse}
// @Failure      400   {object}  response.ErrorResponse
// @Failure      401   {object}  response.ErrorResponse  "Kredensial salah atau di luar jam shift"
// @Router       /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	req := new(usecase.LoginRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	result, err := h.authUC.Login(c.Context(), req)
	if err != nil {
		return response.Unauthorized(c, err.Error())
	}

	ua := c.Get("User-Agent")
	if len(ua) > 255 {
		ua = ua[:255]
	}
	session := &entity.AuthSession{
		UserID:    result.User.ID,
		JTI:       result.JTI,
		LoginAt:   time.Now(),
		ExpiresAt: &result.ExpiresAt,
		IP:        c.IP(),
		UserAgent: ua,
	}
	if err := h.db.WithContext(c.Context()).Create(session).Error; err != nil {
		return response.InternalServerError(c, "Gagal mencatat sesi login")
	}

	return response.OK(c, "Login berhasil", result)
}

// Logout godoc
// @Summary      Logout pengguna
// @Description  Menutup auth session milik token yang sedang dipakai.
// @Tags         Autentikasi
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response
// @Router       /api/v1/auth/logout [post]
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	jti, _ := c.Locals("jti").(string)
	if jti != "" {
		now := time.Now()
		h.db.WithContext(c.Context()).Model(&entity.AuthSession{}).
			Where("jti = ? AND logout_at IS NULL", jti).
			Updates(map[string]interface{}{
				"logout_at":  now,
				"updated_at": now,
			})
	}
	return response.OK(c, "Logout berhasil", nil)
}

// Register godoc
// @Summary      Daftarkan pengguna baru
// @Description  Mendaftarkan akun dengan role superadmin, admin, atau kasir. Endpoint ini sebaiknya dinonaktifkan di production.
// @Tags         Autentikasi
// @Accept       json
// @Produce      json
// @Param        body  body      usecase.RegisterRequest  true  "Data registrasi"
// @Success      201   {object}  response.Response
// @Failure      400   {object}  response.ErrorResponse
// @Failure      409   {object}  response.ErrorResponse  "Username sudah digunakan"
// @Router       /api/v1/auth/register [post]
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	req := new(usecase.RegisterRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	user, err := h.authUC.RegisterUser(c.Context(), req)
	if err != nil {
		return response.Conflict(c, err.Error())
	}
	return response.Created(c, "Akun berhasil dibuat", user)
}
