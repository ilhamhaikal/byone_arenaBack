// Package spname menyediakan penamaan stored procedure/function PostgreSQL
// yang bisa di-white-label per client (mengganti prefix "byone" menjadi
// prefix lain sesuai nama client), tanpa perlu mengubah source code Go
// maupun file migrasi SQL secara manual per deployment.
//
// Prefix dikonfigurasi lewat env SP_PREFIX (lihat pkg/config) dan harus
// di-set sekali di awal lifecycle aplikasi via Init(), sebelum query apa pun
// yang memanggil stored procedure dijalankan.
package spname

import (
	"fmt"
	"regexp"
)

var prefix = "byone"

// validPrefix membatasi prefix hanya huruf/angka dan diawali huruf, agar aman
// disisipkan langsung ke dalam teks SQL sebagai identifier (defense-in-depth,
// meski nilainya berasal dari konfigurasi terpercaya bukan input pengguna).
var validPrefix = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)

// Init mengatur prefix nama stored procedure yang dipakai oleh seluruh
// aplikasi. Panic jika prefix mengandung karakter yang tidak aman.
func Init(p string) {
	if p == "" {
		p = "byone"
	}
	if !validPrefix.MatchString(p) {
		panic(fmt.Sprintf("spname: SP_PREFIX tidak valid: %q (hanya huruf/angka, diawali huruf)", p))
	}
	prefix = p
}

// Name mengembalikan nama fungsi/procedure sesuai prefix aktif,
// misal Name("LogTvActivity") -> "byoneLogTvActivity" (atau prefix client lain).
func Name(suffix string) string {
	return prefix + suffix
}

// Ident mengembalikan identifier SQL siap-pakai (sudah di-quote), misal
// Ident("LogTvActivity") -> `"byoneLogTvActivity"`. Gunakan ini saat menyisipkan
// nama fungsi ke dalam raw SQL query string (PostgreSQL tidak mendukung
// parameter binding untuk nama identifier/fungsi).
func Ident(suffix string) string {
	return `"` + Name(suffix) + `"`
}
