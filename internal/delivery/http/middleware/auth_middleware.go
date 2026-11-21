package middleware

import (
	"encoding/json"
	"strings"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/usecase"
	"byone-arena/pkg/config"
	"byone-arena/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// AuthMiddleware memverifikasi JWT token pada setiap request yang dilindungi.
//
// Selain validasi tanda tangan token, status user dicek ulang ke database pada
// setiap request. Tanpa ini, user yang dinonaktifkan atau dihapus tetap bisa
// memakai token lamanya sampai 24 jam (JWT_EXPIRE_HOURS), dan perubahan role
// tidak langsung berlaku.
func AuthMiddleware(cfg *config.Config, db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Unauthorized(c, "Token autentikasi tidak ditemukan")
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			return response.Unauthorized(c, "Format token tidak valid")
		}

		tokenString := parts[1]
		claims := &usecase.JWTClaims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.ErrUnauthorized
			}
			return []byte(cfg.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			return response.Unauthorized(c, "Token tidak valid atau sudah kadaluarsa")
		}

		if db != nil {
			var user entity.User
			if err := db.WithContext(c.Context()).
				Select("id", "username", "role", "is_active", "permissions", "must_change_password").
				First(&user, "id = ?", claims.UserID).Error; err != nil {
				return response.Unauthorized(c, "Sesi tidak valid, silakan login ulang")
			}
			if !user.IsActive {
				return response.Unauthorized(c, "Akun tidak aktif, hubungi administrator")
			}
			// Gunakan data terbaru dari DB, bukan klaim token (role bisa berubah).
			c.Locals("userID", user.ID)
			c.Locals("username", user.Username)
			c.Locals("role", user.Role)
			c.Locals("mustChangePassword", user.MustChangePassword)

			var perms []string
			if strings.TrimSpace(user.Permissions) != "" {
				_ = json.Unmarshal([]byte(user.Permissions), &perms)
			}
			if perms == nil {
				perms = []string{}
			}
			c.Locals("permissions", perms)
			return c.Next()
		}

		c.Locals("userID", claims.UserID)
		c.Locals("username", claims.Username)
		c.Locals("role", claims.Role)
		c.Locals("permissions", []string{})

		return c.Next()
	}
}

// RequirePasswordChanged memblokir semua endpoint terproteksi sampai user
// mengganti password default. Hanya /auth/me dan /users/change-password yang
// diizinkan.
func RequirePasswordChanged() fiber.Handler {
	return func(c *fiber.Ctx) error {
		mustChange, _ := c.Locals("mustChangePassword").(bool)
		if !mustChange {
			return c.Next()
		}
		switch c.Path() {
		case "/api/v1/auth/me", "/api/v1/users/change-password":
			return c.Next()
		default:
			return response.ForbiddenCode(c, "PASSWORD_CHANGE_REQUIRED",
				"Wajib mengganti password sebelum melanjutkan")
		}
	}
}

// AdminOnly membatasi akses hanya untuk pengguna dengan role admin atau superadmin
func AdminOnly() fiber.Handler {
	return func(c *fiber.Ctx) error {
		roleValue := c.Locals("role")
		role, ok := roleValue.(entity.UserRole)
		if !ok {
			if roleString, stringOK := roleValue.(string); stringOK {
				role = entity.UserRole(roleString)
				ok = true
			}
		}

		if !ok || (role != entity.UserRoleAdmin && role != entity.UserRoleSuperAdmin) {
			return response.Forbidden(c, "Hanya admin atau superadmin yang dapat mengakses endpoint ini")
		}
		return c.Next()
	}
}

// SuperAdminOnly membatasi akses hanya untuk superadmin/owner.
// Dipakai untuk operasi sensitif seperti aktivasi lisensi.
func SuperAdminOnly() fiber.Handler {
	return func(c *fiber.Ctx) error {
		roleValue := c.Locals("role")
		role, ok := roleValue.(entity.UserRole)
		if !ok {
			if roleString, stringOK := roleValue.(string); stringOK {
				role = entity.UserRole(roleString)
				ok = true
			}
		}
		if !ok || role != entity.UserRoleSuperAdmin {
			return response.Forbidden(c, "Hanya owner (superadmin) yang dapat mengakses endpoint ini")
		}
		return c.Next()
	}
}

// RequireWrite membatasi aksi tambah/ubah data pada sebuah menu.
// Superadmin selalu boleh; user lain harus punya izin "<menu>.write".
func RequireWrite(menu string) fiber.Handler {
	return RequirePermission(menu + ".write")
}

// RequireDelete membatasi aksi hapus data pada sebuah menu.
// Superadmin selalu boleh; user lain harus punya izin "<menu>.delete".
func RequireDelete(menu string) fiber.Handler {
	return RequirePermission(menu + ".delete")
}

// RequirePermission mengizinkan superadmin ATAU user yang memiliki izin
// tertentu (dari kolom users.permissions). Dipakai untuk pengaturan sensitif
// seperti harga member/harian.
func RequirePermission(permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		roleValue := c.Locals("role")
		role, ok := roleValue.(entity.UserRole)
		if !ok {
			if roleString, stringOK := roleValue.(string); stringOK {
				role = entity.UserRole(roleString)
				ok = true
			}
		}
		if ok && role == entity.UserRoleSuperAdmin {
			return c.Next()
		}

		if perms, pok := c.Locals("permissions").([]string); pok {
			for _, p := range perms {
				if p == permission {
					return c.Next()
				}
			}
		}

		return response.Forbidden(c,
			"Anda tidak memiliki akses untuk tindakan ini")
	}
}
