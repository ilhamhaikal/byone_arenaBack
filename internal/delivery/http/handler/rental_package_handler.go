package handler

import (
	"byone-arena/internal/license"
	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// RentalPackageHandler menangani HTTP request untuk paket rental.
type RentalPackageHandler struct {
	uc        usecase.RentalPackageUseCase
	validator *validator.Validator
	license   *license.Service
}

// NewRentalPackageHandler membuat instance baru RentalPackageHandler
func NewRentalPackageHandler(uc usecase.RentalPackageUseCase, v *validator.Validator, licenseSvc *license.Service) *RentalPackageHandler {
	return &RentalPackageHandler{uc: uc, validator: v, license: licenseSvc}
}

// List godoc
// @Summary      Daftar paket rental
// @Tags         Paket Rental
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]entity.RentalPackage}
// @Router       /api/v1/rental-packages [get]
func (h *RentalPackageHandler) List(c *fiber.Ctx) error {
	list, err := h.uc.List(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil paket")
	}
	return response.OK(c, "Daftar paket berhasil diambil", list)
}

// Save godoc
// @Summary      Simpan paket rental (buat/update)
// @Tags         Paket Rental
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  usecase.SaveRentalPackageRequest  true  "Data paket"
// @Success      201  {object}  response.Response{data=entity.RentalPackage}
// @Router       /api/v1/rental-packages [post]
func (h *RentalPackageHandler) Save(c *fiber.Ctx) error {
	req := new(usecase.SaveRentalPackageRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}
	pkg, err := h.uc.Save(c.Context(), req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "Paket berhasil disimpan", pkg)
}

// Delete godoc
// @Summary      Hapus paket rental
// @Tags         Paket Rental
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Package ID (UUID)"
// @Success      200  {object}  response.Response
// @Router       /api/v1/rental-packages/{id} [delete]
func (h *RentalPackageHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	if err := h.uc.Delete(c.Context(), id); err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Paket dihapus", nil)
}

// Start godoc
// @Summary      Mulai sesi rental dengan paket
// @Tags         Paket Rental
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  usecase.StartWithPackageRequest  true  "Console + package"
// @Success      201  {object}  response.Response{data=usecase.StartSessionResponse}
// @Router       /api/v1/sessions/start-package [post]
func (h *RentalPackageHandler) Start(c *fiber.Ctx) error {
	req := new(usecase.StartWithPackageRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}
	if h.license != nil {
		if err := h.license.EnsureCanStartBusiness(c.Context()); err != nil {
			return response.Forbidden(c, err.Error())
		}
	}
	result, err := h.uc.StartWithPackage(c.Context(), req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "Sesi paket berhasil dimulai", result)
}
