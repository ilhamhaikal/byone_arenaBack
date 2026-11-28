package handler

import (
	"errors"
	"strings"

	"byone-arena/internal/license"
	"byone-arena/pkg/response"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
)

// LicenseHandler menangani status, aktivasi, dan riwayat lisensi.
type LicenseHandler struct {
	svc       *license.Service
	validator *validator.Validator
}

func NewLicenseHandler(svc *license.Service, v *validator.Validator) *LicenseHandler {
	return &LicenseHandler{svc: svc, validator: v}
}

// Status godoc
// @Summary      Status lisensi server
// @Description  Mengembalikan status lisensi (aktif/expiring/grace/expired/trial),
// jumlah unit terpakai, tanggal kedaluwarsa, dan ID server untuk pengajuan lisensi.
// @Tags         Lisensi
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} response.Response{data=license.Status}
// @Router       /api/v1/license/status [get]
func (h *LicenseHandler) Status(c *fiber.Ctx) error {
	return response.OK(c, "Status lisensi", h.svc.Status(c.Context()))
}

type activateLicenseRequest struct {
	LicenseKey string `json:"licenseKey" validate:"required"`
	Force      bool   `json:"force"`
}

// Activate godoc
// @Summary      Impor/aktifkan lisensi
// @Description  Memverifikasi tanda tangan lisensi lalu menyimpannya. Bila lisensi
// baru menurunkan masa berlaku atau jumlah unit, respons berisi
// `requiresConfirmation: true` dan daftar `warnings`; kirim ulang dengan
// `force: true` setelah admin menyetujui.
// @Tags         Lisensi
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body handler.activateLicenseRequest true "Teks lisensi"
// @Success      200 {object} response.Response
// @Router       /api/v1/license/activate [post]
func (h *LicenseHandler) Activate(c *fiber.Ctx) error {
	req := new(activateLicenseRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	key := strings.TrimSpace(req.LicenseKey)
	st, warnings, err := h.svc.ActivateOnline(c.Context(), key, req.Force)

	// Perlu konfirmasi (downgrade masa berlaku/unit) — bukan error fatal.
	if errors.Is(err, license.ErrNeedsConfirmation) {
		return response.OK(c, "Lisensi perlu konfirmasi", fiber.Map{
			"activated":            false,
			"requiresConfirmation": true,
			"warnings":             warnings,
			"status":               h.svc.Status(c.Context()),
		})
	}
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	return response.OK(c, "Lisensi berhasil diaktifkan", fiber.Map{
		"activated": true,
		"warnings":  warnings,
		"status":    st,
	})
}

type startOnlineActivationRequest struct {
	MachineLabel string `json:"machineLabel"`
	OSInfo       string `json:"osInfo"`
}

// OnlineStart godoc
// @Summary      Buat kode aktivasi QR
// @Description  Membuat request aktivasi ke license server vendor. Secret tidak
// pernah dikirim ke frontend; polling dilakukan oleh server billing.
// @Tags         Lisensi
// @Security     BearerAuth
// @Router       /api/v1/license/online/start [post]
func (h *LicenseHandler) OnlineStart(c *fiber.Ctx) error {
	if len(c.Body()) > 4096 {
		return response.BadRequest(c, "Request terlalu besar")
	}
	req := new(startOnlineActivationRequest)
	if len(c.Body()) > 0 {
		if err := c.BodyParser(req); err != nil {
			return response.BadRequest(c, "Format request tidak valid")
		}
	}
	if len(req.MachineLabel) > 100 || len(req.OSInfo) > 100 {
		return response.BadRequest(c, "machineLabel/osInfo terlalu panjang")
	}
	result, err := h.svc.StartOnlineActivation(c.Context(), req.MachineLabel, req.OSInfo)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Kode aktivasi dibuat", result)
}

// OnlineStatus godoc
// @Summary      Cek status aktivasi QR
// @Description  Server billing melakukan polling ke vendor dan menyimpan token
// jika sudah diterbitkan.
// @Tags         Lisensi
// @Security     BearerAuth
// @Router       /api/v1/license/online/status [get]
func (h *LicenseHandler) OnlineStatus(c *fiber.Ctx) error {
	result, err := h.svc.PollOnlineActivation(c.Context())
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Status aktivasi", result)
}

// History godoc
// @Summary      Riwayat lisensi
// @Tags         Lisensi
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} response.Response{data=[]license.HistoryRow}
// @Router       /api/v1/license/history [get]
func (h *LicenseHandler) History(c *fiber.Ctx) error {
	rows, err := h.svc.History(c.Context(), 30)
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil riwayat lisensi")
	}
	return response.OK(c, "Riwayat lisensi", rows)
}
