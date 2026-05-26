package handler

import (
	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
)

// AuthHandler menangani HTTP request untuk autentikasi
type AuthHandler struct {
	authUC    usecase.AuthUseCase
	validator *validator.Validator
}

// NewAuthHandler membuat instance baru AuthHandler
func NewAuthHandler(authUC usecase.AuthUseCase, v *validator.Validator) *AuthHandler {
	return &AuthHandler{authUC: authUC, validator: v}
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
	return response.OK(c, "Login berhasil", result)
}

// Register godoc
// @Summary      Daftarkan pengguna baru
// @Description  Mendaftarkan akun dengan role superadmin, admin, atau kasir. Endpoint ini sebaiknya dinonaktifkan di production.
// @Tags         Autentikasi
// @Accept       json
// @Produce      json
// @Param        body  body      usecase.RegisterRequest  true  "Data registrasi"
// @Success      201   {object}  response.Response{data=entity.User}
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

