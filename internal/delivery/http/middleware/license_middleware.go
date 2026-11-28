package middleware

import (
	"strings"

	"byone-arena/internal/license"
	"byone-arena/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// licenseWriteAllowlist adalah operasi yang tetap boleh dilakukan saat lisensi
// berakhir agar operasional bisa ditutup dengan aman (mis. mengakhiri sesi,
// menerima pembayaran, menutup shift, melepas perangkat, dan aktivasi lisensi).
func licenseWriteAllowed(path string) bool {
	path = strings.TrimRight(path, "/")
	prefixes := []string{
		"/api/v1/license/",
		"/api/v1/auth/",
		"/api/v1/users/change-password",
		"/api/v1/payments",
		"/api/v1/shifts",
		"/api/v1/devices/",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	for _, suffix := range []string{"/end", "/cancel", "/return"} {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

// LicenseGate memblokir operasi tulis baru saat lisensi tidak mengizinkan
// operasional (expired/none/invalid/suspended/revoked), tetapi tetap
// mengizinkan pembacaan data dan operasi penutupan.
func LicenseGate(svc *license.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if svc == nil {
			return c.Next()
		}
		switch c.Method() {
		case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
			return c.Next()
		}
		if licenseWriteAllowed(c.Path()) {
			return c.Next()
		}
		st := svc.Status(c.Context())
		if st.CanStartBusiness {
			return c.Next()
		}
		return response.ForbiddenCode(c, "LICENSE_REQUIRED", st.Message)
	}
}

// RequireValidLicense dipakai pada endpoint publik yang dipakai TV/perangkat
// (WebSocket, overview, pairing). Instalasi APK tetap boleh, tetapi pemakaian
// API ditolak saat lisensi server billing tidak valid.
func RequireValidLicense(svc *license.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if svc == nil {
			return c.Next()
		}
		st := svc.Status(c.Context())
		if st.CanStartBusiness {
			return c.Next()
		}
		return response.ForbiddenCode(c, "LICENSE_REQUIRED", st.Message)
	}
}
