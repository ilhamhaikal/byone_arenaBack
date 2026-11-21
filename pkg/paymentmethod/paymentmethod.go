// Package paymentmethod menyatukan aturan metode pembayaran dan finalisasi
// non-tunai. Dipakai handler & repository agar koreksi metode selalu berada
// dalam transaksi yang sama dengan stored procedure pembuatan/konfirmasi.
package paymentmethod

import (
	"strings"

	"byone-arena/pkg/spname"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Sentinel adalah nilai uang tunai besar yang dipakai SP lama agar validasi
// "cash >= total" lolos untuk QRIS/transfer. Nilai ini WAJIB dikoreksi dalam
// transaksi yang sama lewat Finalize.
const Sentinel = 9999999.0

// Normalize memastikan hanya metode yang dikenal yang dipakai.
func Normalize(method string) string {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "qris":
		return "qris"
	case "transfer":
		return "transfer"
	default:
		return "cash"
	}
}

// IsCashless true bila pembayaran tidak memakai uang tunai.
func IsCashless(method string) bool {
	m := Normalize(method)
	return m == "qris" || m == "transfer"
}

// Finalize mengoreksi baris payment setelah SP: metode sesuai pilihan,
// cash_received = total_payment, change_amount = 0. Untuk cash no-op.
// Mengembalikan error agar pemanggil bisa rollback seluruh transaksi.
func Finalize(tx *gorm.DB, paymentID uuid.UUID, method string) error {
	if tx == nil || paymentID == uuid.Nil {
		return nil
	}
	m := Normalize(method)
	if m == "cash" {
		return nil
	}
	return tx.Exec(
		"SELECT "+spname.Ident("FinalizePaymentMethod")+"(?::uuid, ?::varchar)",
		paymentID, m,
	).Error
}
