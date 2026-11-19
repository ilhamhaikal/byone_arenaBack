package usecase

import (
	"context"
	"encoding/json"
	"errors"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// UserUseCase mendefinisikan logika bisnis untuk manajemen user
type UserUseCase interface {
	GetAllUsers(ctx context.Context) ([]*entity.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
	CreateUser(ctx context.Context, req *CreateUserRequest, currentUserID uuid.UUID, currentUserRole entity.UserRole) (*entity.User, error)
	UpdateUser(ctx context.Context, id uuid.UUID, req *UpdateUserRequest, currentUserID uuid.UUID, currentUserRole entity.UserRole) (*entity.User, error)
	DeleteUser(ctx context.Context, id uuid.UUID, currentUserID uuid.UUID, currentUserRole entity.UserRole) error
	ChangePassword(ctx context.Context, userID uuid.UUID, req *ChangePasswordRequest) error
}

type userUseCase struct {
	userRepo repository.UserRepository
}

// NewUserUseCase membuat instance baru UserUseCase
func NewUserUseCase(userRepo repository.UserRepository) UserUseCase {
	return &userUseCase{userRepo: userRepo}
}

// CreateUserRequest payload untuk membuat user baru
type CreateUserRequest struct {
	Username    string   `json:"username"    validate:"required,min=3,max=50"`
	Password    string   `json:"password"    validate:"required,min=6,max=100"`
	FullName    string   `json:"fullName"    validate:"required,min=1,max=100"`
	Role        string   `json:"role"        validate:"required,oneof=admin superadmin kasir"`
	Permissions []string `json:"permissions"` // opsional, default sesuai role
}

// UpdateUserRequest payload untuk update user
type UpdateUserRequest struct {
	Username    *string  `json:"username,omitempty"    validate:"omitempty,min=3,max=50"`
	Password    *string  `json:"password,omitempty"    validate:"omitempty,min=6,max=100"`
	FullName    *string  `json:"fullName,omitempty"    validate:"omitempty,min=1,max=100"`
	Role        *string  `json:"role,omitempty"        validate:"omitempty,oneof=admin superadmin kasir"`
	IsActive    *bool    `json:"isActive,omitempty"`
	Permissions []string `json:"permissions,omitempty"` // opsional, update daftar menu
}

// ChangePasswordRequest payload untuk ganti password sendiri
type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword" validate:"required"`
	NewPassword string `json:"newPassword" validate:"required,min=6,max=100"`
}

func (uc *userUseCase) GetAllUsers(ctx context.Context) ([]*entity.User, error) {
	return uc.userRepo.FindAll(ctx)
}

func (uc *userUseCase) GetUserByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	user, err := uc.userRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user tidak ditemukan")
	}
	return user, nil
}

func (uc *userUseCase) CreateUser(ctx context.Context, req *CreateUserRequest, currentUserID uuid.UUID, currentUserRole entity.UserRole) (*entity.User, error) {
	// Validasi hierarki: hanya boleh membuat user dengan role <= current user
	targetRole := entity.UserRole(req.Role)
	if !canManageRole(currentUserRole, targetRole) {
		return nil, errors.New("anda tidak memiliki izin untuk membuat user dengan role tersebut")
	}
	// Cek duplikasi username
	existing, _ := uc.userRepo.FindByUsername(ctx, req.Username)
	if existing != nil {
		return nil, errors.New("username sudah digunakan")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("gagal mengenkripsi password")
	}

	user := &entity.User{
		Username: req.Username,
		Password: string(hash),
		FullName: req.FullName,
		Role:     entity.UserRole(req.Role),
		IsActive: true,
	}

	// Set permissions: dari request, atau default sesuai role.
	// Izin sensitif (harga member/harian) hanya boleh diberikan superadmin.
	permissions := entity.FilterGrantablePermissions(req.Permissions)
	if currentUserRole != entity.UserRoleSuperAdmin {
		permissions = entity.WithoutSensitivePermissions(permissions)
	}
	if len(permissions) == 0 {
		permissions = defaultPermissionsForRole(targetRole, currentUserRole)
	}
	user.Permissions = marshalPermissions(permissions)

	if err := uc.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (uc *userUseCase) UpdateUser(ctx context.Context, id uuid.UUID, req *UpdateUserRequest, currentUserID uuid.UUID, currentUserRole entity.UserRole) (*entity.User, error) {
	user, err := uc.userRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user tidak ditemukan")
	}

	// Validasi hierarki
	if !canManageRole(currentUserRole, user.Role) {
		return nil, errors.New("anda tidak memiliki izin untuk mengubah user ini")
	}

	// Cek apakah user mengedit dirinya sendiri
	isSelfEdit := user.ID == currentUserID

	if req.Username != nil {
		if *req.Username != user.Username {
			existing, _ := uc.userRepo.FindByUsername(ctx, *req.Username)
			if existing != nil {
				return nil, errors.New("username sudah digunakan")
			}
			user.Username = *req.Username
		}
	}
	if req.Password != nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, errors.New("gagal mengenkripsi password")
		}
		user.Password = string(hash)
	}
	if req.FullName != nil {
		user.FullName = *req.FullName
	}

	// Role & permissions: superadmin bisa ubah siapa saja (termasuk diri sendiri),
	// admin hanya bisa ubah milik kasir, kasir tidak bisa ubah siapa pun
	if req.Role != nil || req.Permissions != nil {
		// Admin tidak boleh ubah role/permissions dirinya sendiri
		if isSelfEdit && currentUserRole == entity.UserRoleAdmin {
			return nil, errors.New("admin tidak dapat mengubah hak akses sendiri, hanya superadmin yang dapat melakukannya")
		}

		oldRole := user.Role
		if req.Role != nil {
			newRole := entity.UserRole(*req.Role)
			// Validasi role TUJUAN, bukan hanya role user saat ini. Tanpa ini
			// admin bisa menaikkan kasir menjadi admin/superadmin.
			if !canManageRole(currentUserRole, newRole) {
				return nil, errors.New("anda tidak memiliki izin untuk memberikan role tersebut")
			}
			user.Role = newRole
		}

		existingPermissions := unmarshalPermissions(user.Permissions)
		switch {
		case req.Permissions != nil:
			permissions := entity.FilterGrantablePermissions(req.Permissions)
			if currentUserRole != entity.UserRoleSuperAdmin {
				// Admin tidak boleh menambah/menghapus izin sensitif yang sudah
				// diberikan superadmin; izin lama dipertahankan.
				permissions = entity.WithoutSensitivePermissions(permissions)
				permissions = append(permissions, entity.SensitivePermissionsFrom(existingPermissions)...)
				permissions = entity.FilterGrantablePermissions(permissions)
			}
			if len(permissions) == 0 {
				permissions = defaultPermissionsForRole(user.Role, currentUserRole)
			}
			user.Permissions = marshalPermissions(permissions)
		case user.Role != oldRole:
			// Role berubah tanpa daftar izin eksplisit: ikutkan default role
			// baru supaya menu yang baru ditambahkan tidak hilang.
			user.Permissions = marshalPermissions(
				defaultPermissionsForRole(user.Role, currentUserRole),
			)
		}
	}

	if req.IsActive != nil {
		// Tidak boleh nonaktifkan diri sendiri
		if isSelfEdit && !*req.IsActive {
			return nil, errors.New("tidak dapat menonaktifkan akun sendiri")
		}
		user.IsActive = *req.IsActive
	}

	if err := uc.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (uc *userUseCase) DeleteUser(ctx context.Context, id uuid.UUID, currentUserID uuid.UUID, currentUserRole entity.UserRole) error {
	// Tidak boleh hapus diri sendiri
	if id == currentUserID {
		return errors.New("tidak dapat menghapus akun sendiri")
	}

	user, err := uc.userRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("user tidak ditemukan")
	}

	// Validasi hierarki
	if !canManageRole(currentUserRole, user.Role) {
		return errors.New("anda tidak memiliki izin untuk menghapus user ini")
	}

	return uc.userRepo.Delete(ctx, id)
}

func (uc *userUseCase) ChangePassword(ctx context.Context, userID uuid.UUID, req *ChangePasswordRequest) error {
	user, err := uc.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("user tidak ditemukan")
	}

	// Verifikasi password lama
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		return errors.New("password lama tidak sesuai")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("gagal mengenkripsi password baru")
	}

	user.Password = string(hash)
	return uc.userRepo.Update(ctx, user)
}

// canManageRole mengecek apakah currentRole boleh mengelola targetRole.
// Hierarki: superadmin > admin > kasir
// - superadmin bisa mengelola siapa saja (termasuk superadmin lain & diri sendiri)
// - admin hanya bisa mengelola kasir
// - kasir tidak bisa mengelola siapa pun
func canManageRole(currentRole, targetRole entity.UserRole) bool {
	switch currentRole {
	case entity.UserRoleSuperAdmin:
		return true // superadmin bisa semua
	case entity.UserRoleAdmin:
		return targetRole == entity.UserRoleKasir // admin hanya bisa kelola kasir
	default:
		return false // kasir tidak bisa kelola siapa pun
	}
}

// defaultPermissionsForRole mengembalikan izin default untuk role tujuan.
// Jika yang mengatur bukan superadmin, izin sensitif tidak ikut diberikan.
func defaultPermissionsForRole(role, actorRole entity.UserRole) []string {
	permissions := entity.DefaultPermissionsForRole(role)
	if actorRole != entity.UserRoleSuperAdmin {
		permissions = entity.WithoutSensitivePermissions(permissions)
	}
	return permissions
}

// marshalPermissions menyimpan daftar izin sebagai JSON array.
func marshalPermissions(permissions []string) string {
	if permissions == nil {
		permissions = []string{}
	}
	b, err := json.Marshal(permissions)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// unmarshalPermissions membaca users.permissions (JSONB) yang berbentuk array.
func unmarshalPermissions(raw string) []string {
	if raw == "" {
		return nil
	}
	var permissions []string
	if err := json.Unmarshal([]byte(raw), &permissions); err != nil {
		return nil
	}
	return permissions
}
