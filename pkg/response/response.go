package response

import "github.com/gofiber/fiber/v2"

// Response adalah struktur standar untuk semua respons API
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorResponse adalah struktur standar untuk respons error
type ErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// OK mengirim respons sukses dengan status 200
func OK(c *fiber.Ctx, message string, data interface{}) error {
	return c.Status(fiber.StatusOK).JSON(Response{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// Created mengirim respons sukses dengan status 201
func Created(c *fiber.Ctx, message string, data interface{}) error {
	return c.Status(fiber.StatusCreated).JSON(Response{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// BadRequest mengirim respons error 400
func BadRequest(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
		Success: false,
		Message: message,
	})
}

// Unauthorized mengirim respons error 401
func Unauthorized(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
		Success: false,
		Message: message,
	})
}

// Forbidden mengirim respons error 403
func Forbidden(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{
		Success: false,
		Message: message,
	})
}

// NotFound mengirim respons error 404
func NotFound(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{
		Success: false,
		Message: message,
	})
}

// InternalServerError mengirim respons error 500
func InternalServerError(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
		Success: false,
		Message: message,
	})
}

// Conflict mengirim respons error 409
func Conflict(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusConflict).JSON(ErrorResponse{
		Success: false,
		Message: message,
	})
}
