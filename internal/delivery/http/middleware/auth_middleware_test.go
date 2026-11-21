package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRequirePasswordChangedBlocksUntilAllowed(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("mustChangePassword", true)
		return c.Next()
	})
	app.Use(RequirePasswordChanged())
	app.Get("/api/v1/auth/me", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	app.Patch("/api/v1/users/change-password", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	app.Get("/api/v1/dashboard/summary", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	allowed := []struct {
		method string
		path   string
	}{
		{fiber.MethodGet, "/api/v1/auth/me"},
		{fiber.MethodPatch, "/api/v1/users/change-password"},
	}
	for _, tc := range allowed {
		resp, err := app.Test(httptest.NewRequest(tc.method, tc.path, nil))
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("%s %s status = %d, want 200", tc.method, tc.path, resp.StatusCode)
		}
	}

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/api/v1/dashboard/summary", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("dashboard status = %d, want 403", resp.StatusCode)
	}
}
