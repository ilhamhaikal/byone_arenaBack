package handler

import (
	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// VoucherHandler menangani HTTP request untuk manajemen voucher diskon
type VoucherHandler struct {
	voucherUC usecase.VoucherUseCase
	validator *validator.Validator
}

// NewVoucherHandler membuat instance baru VoucherHandler
func NewVoucherHandler(voucherUC usecase.VoucherUseCase, v *validator.Validator) *VoucherHandler {
	return &VoucherHandler{voucherUC: voucherUC, validator: v}
}

// GetAll godoc
// @Summary      Ambil semua voucher
// @Description  Mengembalikan daftar seluruh voucher diskon beserta status dan statistik penggunaan
// @Tags         Voucher
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]entity.Voucher}
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse  "Hanya admin dan superadmin"
// @Failure      500  {object}  response.ErrorResponse
// @Router       /api/v1/vouchers [get]
func (h *VoucherHandler) GetAll(c *fiber.Ctx) error {
	vouchers, err := h.voucherUC.GetAllVouchers(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil data voucher")
	}
	return response.OK(c, "Data voucher berhasil diambil", vouchers)
}

// GetByID godoc
// @Summary      Ambil voucher berdasarkan ID
// @Description  Mengembalikan detail satu voucher diskon
// @Tags         Voucher
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Voucher ID (UUID)"
// @Success      200  {object}  response.Response{data=entity.Voucher}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/vouchers/{id} [get]
func (h *VoucherHandler) GetByID(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	voucher, err := h.voucherUC.GetVoucherByID(c.Context(), id)
	if err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.OK(c, "Data voucher berhasil diambil", voucher)
}

// GetByCode godoc
// @Summary      Cek voucher berdasarkan kode
// @Description  Mengecek validitas dan detail voucher berdasarkan kodenya. Dapat digunakan kasir untuk preview diskon sebelum pembayaran.
// @Tags         Voucher
// @Produce      json
// @Security     BearerAuth
// @Param        code  path      string  true  "Kode voucher (tidak case-sensitive)"
// @Success      200   {object}  response.Response{data=entity.Voucher}
// @Failure      400   {object}  response.ErrorResponse
// @Failure      401   {object}  response.ErrorResponse
// @Failure      404   {object}  response.ErrorResponse  "Kode tidak ditemukan"
// @Router       /api/v1/vouchers/code/{code} [get]
func (h *VoucherHandler) GetByCode(c *fiber.Ctx) error {
	code := c.Params("code")
	if code == "" {
		return response.BadRequest(c, "Kode voucher tidak boleh kosong")
	}
	voucher, err := h.voucherUC.GetVoucherByCode(c.Context(), code)
	if err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.OK(c, "Data voucher berhasil diambil", voucher)
}

// Create godoc
// @Summary      Buat voucher baru
// @Description  Membuat voucher diskon baru.\n\n**Nilai discountType yang valid:**\n- `percentage` — diskon persen dari total (discountValue = 0–100)\n- `fixed_amount` — diskon nominal Rp tetap\n\n**Format expiresAt:** RFC3339/ISO8601, contoh `2026-06-26T00:00:00Z`. Kirim `null` jika tidak ada batas waktu.\n\nKode otomatis diubah ke UPPERCASE.
// @Tags         Voucher
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      usecase.CreateVoucherRequest  true  "Data voucher baru"
// @Success      201   {object}  response.Response{data=entity.Voucher}
// @Failure      400   {object}  response.ErrorResponse  "Validasi gagal — cek discountType (percentage|fixed_amount) dan format expiresAt (RFC3339)"
// @Failure      401   {object}  response.ErrorResponse
// @Failure      403   {object}  response.ErrorResponse  "Hanya admin atau superadmin"
// @Router       /api/v1/vouchers [post]
func (h *VoucherHandler) Create(c *fiber.Ctx) error {
	req := new(usecase.CreateVoucherRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	voucher, err := h.voucherUC.CreateVoucher(c.Context(), req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "Voucher berhasil dibuat", voucher)
}

// Update godoc
// @Summary      Update data voucher
// @Description  Memperbarui informasi voucher. Semua field bersifat opsional.
// @Tags         Voucher
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                        true  "Voucher ID (UUID)"
// @Param        body  body      usecase.UpdateVoucherRequest  true  "Data voucher yang diperbarui"
// @Success      200   {object}  response.Response{data=entity.Voucher}
// @Failure      400   {object}  response.ErrorResponse
// @Failure      401   {object}  response.ErrorResponse
// @Failure      403   {object}  response.ErrorResponse
// @Failure      404   {object}  response.ErrorResponse
// @Router       /api/v1/vouchers/{id} [put]
func (h *VoucherHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	req := new(usecase.UpdateVoucherRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	voucher, err := h.voucherUC.UpdateVoucher(c.Context(), id, req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Voucher berhasil diperbarui", voucher)
}

// Toggle godoc
// @Summary      Aktifkan / nonaktifkan voucher
// @Description  Toggle status isActive voucher. Voucher nonaktif tidak bisa digunakan saat pembayaran.
// @Tags         Voucher
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Voucher ID (UUID)"
// @Success      200  {object}  response.Response{data=entity.Voucher}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/vouchers/{id}/toggle [patch]
func (h *VoucherHandler) Toggle(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	voucher, err := h.voucherUC.ToggleVoucher(c.Context(), id)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	msg := "Voucher berhasil dinonaktifkan"
	if voucher.IsActive {
		msg = "Voucher berhasil diaktifkan"
	}
	return response.OK(c, msg, voucher)
}

// Delete godoc
// @Summary      Hapus voucher
// @Description  Menghapus voucher secara permanen. Voucher yang sudah dipakai di transaksi tidak dapat dihapus.
// @Tags         Voucher
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Voucher ID (UUID)"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/vouchers/{id} [delete]
func (h *VoucherHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	if err := h.voucherUC.DeleteVoucher(c.Context(), id); err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Voucher berhasil dihapus", nil)
}
