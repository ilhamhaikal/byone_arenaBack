package handler

import (
	"errors"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserHandler menangani HTTP request untuk manajemen user
type UserHandler struct {
	userUC    usecase.UserUseCase
	validator *validator.Validator
	db        *gorm.DB
}

// NewUserHandler membuat instance baru UserHandler
func NewUserHandler(userUC usecase.UserUseCase, v *validator.Validator, db *gorm.DB) *UserHandler {
	return &UserHandler{userUC: userUC, validator: v, db: db}
}

// GetAll godoc
// @Summary      Ambil semua user
// @Description  Mengembalikan daftar seluruh user (admin/superadmin only)
// @Tags         Users
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]entity.User}
// @Router       /api/v1/users [get]
func (h *UserHandler) GetAll(c *fiber.Ctx) error {
	users, err := h.userUC.GetAllUsers(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil data user")
	}
	return response.OK(c, "Data user berhasil diambil", users)
}

// GetByID godoc
// @Summary      Ambil user berdasarkan ID
// @Tags         Users
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "User ID"
// @Success      200  {object}  response.Response{data=entity.User}
// @Router       /api/v1/users/{id} [get]
func (h *UserHandler) GetByID(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	user, err := h.userUC.GetUserByID(c.Context(), id)
	if err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.OK(c, "Data user berhasil diambil", user)
}

// Create godoc
// @Summary      Tambah user baru
// @Description  Membuat user baru. Superadmin bisa buat siapa saja, admin hanya bisa buat kasir.
// @Tags         Users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  usecase.CreateUserRequest  true  "Data user"
// @Success      201  {object}  response.Response{data=entity.User}
// @Router       /api/v1/users [post]
func (h *UserHandler) Create(c *fiber.Ctx) error {
	currentUserID, currentUserRole, err := h.extractCurrentUser(c)
	if err != nil {
		return response.Unauthorized(c, err.Error())
	}

	req := new(usecase.CreateUserRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}
	user, err := h.userUC.CreateUser(c.Context(), req, currentUserID, currentUserRole)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "User berhasil dibuat", user)
}

// Update godoc
// @Summary      Update data user
// @Description  Memperbarui data user. Superadmin bisa ubah siapa saja, admin hanya bisa ubah kasir. Admin tidak bisa ubah hak akses sendiri.
// @Tags         Users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                      true  "User ID"
// @Param        body  body  usecase.UpdateUserRequest   true  "Data user yang diperbarui"
// @Success      200  {object}  response.Response{data=entity.User}
// @Router       /api/v1/users/{id} [put]
func (h *UserHandler) Update(c *fiber.Ctx) error {
	currentUserID, currentUserRole, err := h.extractCurrentUser(c)
	if err != nil {
		return response.Unauthorized(c, err.Error())
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	req := new(usecase.UpdateUserRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}
	user, err := h.userUC.UpdateUser(c.Context(), id, req, currentUserID, currentUserRole)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "User berhasil diperbarui", user)
}

// Delete godoc
// @Summary      Hapus user
// @Description  Menghapus user. Superadmin bisa hapus siapa saja (kecuali diri sendiri), admin hanya bisa hapus kasir.
// @Tags         Users
// @Security     BearerAuth
// @Param        id  path  string  true  "User ID"
// @Success      200  {object}  response.Response
// @Router       /api/v1/users/{id} [delete]
func (h *UserHandler) Delete(c *fiber.Ctx) error {
	currentUserID, currentUserRole, err := h.extractCurrentUser(c)
	if err != nil {
		return response.Unauthorized(c, err.Error())
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	if err := h.userUC.DeleteUser(c.Context(), id, currentUserID, currentUserRole); err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "User berhasil dihapus", nil)
}

// ChangePassword godoc
// @Summary      Ganti password sendiri
// @Description  User mengganti password sendiri (harus tahu password lama)
// @Tags         Users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  usecase.ChangePasswordRequest  true  "Password lama & baru"
// @Success      200  {object}  response.Response
// @Router       /api/v1/users/change-password [patch]
func (h *UserHandler) ChangePassword(c *fiber.Ctx) error {
	// Ambil user ID dari JWT context
	userID, ok := c.Locals("userID").(uuid.UUID)
	if !ok {
		return response.Unauthorized(c, "User tidak terautentikasi")
	}

	req := new(usecase.ChangePasswordRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	if err := h.userUC.ChangePassword(c.Context(), userID, req); err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Password berhasil diubah", nil)
}

// extractCurrentUser mengambil userID dan role dari JWT context
func (h *UserHandler) extractCurrentUser(c *fiber.Ctx) (uuid.UUID, entity.UserRole, error) {
	userID, ok := c.Locals("userID").(uuid.UUID)
	if !ok {
		return uuid.Nil, "", errors.New("user tidak terautentikasi")
	}

	roleVal := c.Locals("role")
	role, ok := roleVal.(entity.UserRole)
	if !ok {
		// Coba parse dari string
		if roleStr, ok2 := roleVal.(string); ok2 {
			role = entity.UserRole(roleStr)
		} else {
			return uuid.Nil, "", errors.New("role tidak valid")
		}
	}

	return userID, role, nil
}
