package handler

import (
	"strings"

	"byone-arena/pkg/spname"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// paymentMethodSentinel dipakai saat memanggil stored procedure untuk pembayaran
// non-tunai (QRIS/transfer). SP lama selalu memvalidasi "uang tunai >= total" dan
// menghitung kembalian; dengan nilai besar, validasi lolos, lalu baris payment
// langsung dikoreksi oleh finalizePaymentMethod().
const paymentMethodSentinel = 9999999.0 // maksimum praktis untuk kolom numeric(10,2)

// normalizePaymentMethod memastikan hanya metode yang dikenal yang dipakai.
func normalizePaymentMethod(method string) string {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "qris":
		return "qris"
	case "transfer":
		return "transfer"
	default:
		return "cash"
	}
}

// isCashless true bila pembayaran tidak memakai uang tunai.
func isCashless(method string) bool {
	m := normalizePaymentMethod(method)
	return m == "qris" || m == "transfer"
}

// finalizePaymentMethod mengoreksi baris payment setelah SP:
// metode diset sesuai pilihan, uang diterima = total, kembalian 0.
func finalizePaymentMethod(db *gorm.DB, paymentID uuid.UUID, method string) {
	if db == nil || paymentID == uuid.Nil {
		return
	}
	m := normalizePaymentMethod(method)
	if m == "cash" {
		return
	}
	db.Exec(
		"SELECT "+spname.Ident("FinalizePaymentMethod")+"(?::uuid, ?::varchar)",
		paymentID, m,
	)
}
