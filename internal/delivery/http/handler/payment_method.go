package handler

import (
	"byone-arena/pkg/paymentmethod"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// paymentMethodSentinel dipakai saat memanggil stored procedure untuk pembayaran
// non-tunai (QRIS/transfer). Nilai besar agar validasi SP lama lolos, lalu
// baris payment dikoreksi dalam transaksi yang sama.
const paymentMethodSentinel = paymentmethod.Sentinel

// normalizePaymentMethod memastikan hanya metode yang dikenal yang dipakai.
func normalizePaymentMethod(method string) string {
	return paymentmethod.Normalize(method)
}

// isCashless true bila pembayaran tidak memakai uang tunai.
func isCashless(method string) bool {
	return paymentmethod.IsCashless(method)
}

// finalizePaymentMethod mengoreksi baris payment di dalam transaksi pemanggil.
// Error wajib diperiksa agar transaksi bisa di-rollback.
func finalizePaymentMethod(tx *gorm.DB, paymentID uuid.UUID, method string) error {
	return paymentmethod.Finalize(tx, paymentID, method)
}
