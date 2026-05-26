package usecase

import (
	"context"
	"errors"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"
	"byone-arena/pkg/config"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// AuthUseCase mendefinisikan logika bisnis untuk autentikasi
type AuthUseCase interface {
	Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error)
	RegisterUser(ctx context.Context, req *RegisterRequest) (*entity.User, error)
}

type authUseCase struct {
	userRepo  repository.UserRepository
	shiftRepo repository.ShiftRepository
	cfg       *config.Config
}

// NewAuthUseCase membuat instance baru AuthUseCase
func NewAuthUseCase(userRepo repository.UserRepository, shiftRepo repository.ShiftRepository, cfg *config.Config) AuthUseCase {
	return &authUseCase{
		userRepo:  userRepo,
		shiftRepo: shiftRepo,
		cfg:       cfg,
	}
}

// LoginRequest payload untuk login
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse respons setelah login berhasil
type LoginResponse struct {
	Token     string       `json:"token"`
	ExpiresAt time.Time    `json:"expiresAt"`
	User      *entity.User `json:"user"`
}

// RegisterRequest payload untuk mendaftarkan pengguna baru
type RegisterRequest struct {
	Username string          `json:"username" validate:"required,min=3,max=50"`
	Password string          `json:"password" validate:"required,min=8"`
	FullName string          `json:"fullName" validate:"required"`
	Role     entity.UserRole `json:"role"     validate:"required,oneof=superadmin admin kasir"`
}

// JWTClaims mendefinisikan klaim dalam token JWT
type JWTClaims struct {
	UserID   uuid.UUID       `json:"userId"`
	Username string          `json:"username"`
	Role     entity.UserRole `json:"role"`
	jwt.RegisteredClaims
}

func (uc *authUseCase) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	user, err := uc.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("username atau password salah")
	}
	if !user.IsActive {
		return nil, errors.New("akun tidak aktif, hubungi administrator")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, errors.New("username atau password salah")
	}

	// Validasi shift untuk kasir
	if user.Role == entity.UserRoleKasir {
		if err := uc.validateKasirShift(ctx, user.ID); err != nil {
			return nil, err
		}
	}

	expiresAt := time.Now().Add(time.Duration(uc.cfg.JWTExpireHours) * time.Hour)
	claims := &JWTClaims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(uc.cfg.JWTSecret))
	if err != nil {
		return nil, errors.New("gagal membuat token")
	}

	return &LoginResponse{
		Token:     tokenString,
		ExpiresAt: expiresAt,
		User:      user,
	}, nil
}

// validateKasirShift mengecek apakah kasir memiliki shift aktif di jam saat ini
func (uc *authUseCase) validateKasirShift(ctx context.Context, userID uuid.UUID) error {
	shifts, err := uc.shiftRepo.FindActiveByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if len(shifts) == 0 {
		return errors.New("tidak ada jadwal shift aktif untuk akun ini, hubungi administrator")
	}

	now := time.Now()
	for _, shift := range shifts {
		if shift.IsLoginAllowed(now) {
			return nil // Ada minimal satu shift yang membolehkan login
		}
	}

	return errors.New("login tidak diizinkan di luar jam shift Anda")
}

func (uc *authUseCase) RegisterUser(ctx context.Context, req *RegisterRequest) (*entity.User, error) {
	existing, err := uc.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("username sudah digunakan")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("gagal memproses password")
	}

	now := time.Now()
	user := &entity.User{
		ID:        uuid.New(),
		Username:  req.Username,
		Password:  string(hashedPassword),
		FullName:  req.FullName,
		Role:      req.Role,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := uc.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

