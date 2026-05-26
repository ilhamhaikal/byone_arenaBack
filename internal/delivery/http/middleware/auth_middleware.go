package middleware

import (
	"strings"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/usecase"
	"byone-arena/pkg/config"
	"byone-arena/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// AuthMiddleware memverifikasi JWT token pada setiap request yang dilindungi
func AuthMiddleware(cfg *config.Config) fiber.Handler {
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

		// Simpan claims ke context untuk digunakan handler berikutnya
		c.Locals("user_id", claims.UserID)
		c.Locals("username", claims.Username)
		c.Locals("role", claims.Role)

		return c.Next()
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
