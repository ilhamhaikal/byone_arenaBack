package entity

import "strings"

// Katalog izin (permission) terpusat untuk menu & aksi sensitif.
//
// Kolom users.permissions menyimpan array JSON berisi key di bawah ini.
// Frontend memakai daftar yang sama untuk menampilkan checkbox hak akses;
// backend memakainya untuk mengisi default saat user dibuat / role berubah.
//
// Aturan:
//   - Menu baru WAJIB ditambahkan ke MenuPermissionKeys agar bisa diberikan
//     lewat halaman Manajemen User.
//   - Izin sensitif (mis. mengubah harga) hanya boleh diberikan superadmin.
//   - "license" bukan permission yang bisa diberikan: menu lisensi selalu
//     hanya untuk role superadmin.
const (
	PermDashboard      = "dashboard"
	PermRental         = "rental"
	PermConsole        = "console"
	PermTVLog          = "tv_log"
	PermMember         = "member"
	PermDiscount       = "discount"
	PermVoucher        = "voucher"
	PermPromoBonus     = "promo_bonus"
	PermMenu           = "menu"
	PermFoodOrder      = "food_order"
	PermNotif          = "notif"
	PermReport         = "report"
	PermBooking        = "booking"
	PermRentalHarian   = "rental_harian"
	PermUserManagement = "user_management"
	PermController     = "controller"

	// Izin aksi sensitif (bukan menu).
	PermMemberPrice  = "member_price"
	PermDailyPrice   = "daily_price"
	PermConsolePrice = "console.price"
)

// MenuPermissionKeys adalah seluruh menu yang bisa diatur hak aksesnya.
// Urutannya mengikuti urutan menu di sidebar.
var MenuPermissionKeys = []string{
	PermDashboard,
	PermRental,
	PermConsole,
	PermTVLog,
	PermMember,
	PermDiscount,
	PermVoucher,
	PermPromoBonus,
	PermMenu,
	PermFoodOrder,
	PermNotif,
	PermReport,
	PermBooking,
	PermRentalHarian,
	PermUserManagement,
	PermController,
}

// SensitivePermissionKeys adalah izin yang hanya boleh diberikan superadmin.
var SensitivePermissionKeys = []string{
	PermMemberPrice,
	PermDailyPrice,
	PermConsolePrice,
}

// WritableMenuKeys adalah menu yang memiliki aksi tambah/ubah/hapus.
// Menu read-only (dashboard, log TV, laporan, controller) tidak punya aksi.
var WritableMenuKeys = []string{
	PermRental,
	PermConsole,
	PermMember,
	PermDiscount,
	PermVoucher,
	PermPromoBonus,
	PermMenu,
	PermFoodOrder,
	PermNotif,
	PermBooking,
	PermRentalHarian,
	PermUserManagement,
}

// RoleDefaultMenuKeys adalah menu default untuk kasir:
// Dashboard, seluruh menu grup Rental & Penjualan, Konsol, Member, Controller.
// Kasir tidak mendapat Log TV, Notifikasi, Diskon, Voucher, Bonus Waktu,
// Laporan, Manajemen User, dan Lisensi.
var RoleDefaultMenuKeys = []string{
	PermDashboard,
	PermRental,
	PermConsole,
	PermMember,
	PermMenu,
	PermFoodOrder,
	PermBooking,
	PermRentalHarian,
	PermController,
}

// RoleDefaultWriteMenusKasir adalah menu yang boleh diubah kasir secara default.
// Menu Penjualan "Menu"/"Bahan" hanya bisa dilihat; harga & resep tetap
// kewenangan admin.
var RoleDefaultWriteMenusKasir = []string{
	PermRental,
	PermConsole,
	PermMember,
	PermFoodOrder,
	PermBooking,
	PermRentalHarian,
}

// RoleDefaultDeleteMenusKasir adalah menu yang boleh dihapus kasir.
// Sesuai kebijakan: hanya Konsol yang mendapat CRUD penuh untuk kasir.
var RoleDefaultDeleteMenusKasir = []string{
	PermConsole,
}

// WritePermission menghasilkan key izin untuk tambah/ubah data pada menu.
// Contoh: WritePermission("member") == "member.write".
func WritePermission(menu string) string { return menu + ".write" }

// DeletePermission menghasilkan key izin untuk menghapus data pada menu.
// Contoh: DeletePermission("member") == "member.delete".
func DeletePermission(menu string) string { return menu + ".delete" }

// ActionPermissionKeys menghasilkan seluruh key aksi (write + delete) untuk
// menu yang bisa diubah.
func ActionPermissionKeys() []string {
	out := make([]string, 0, len(WritableMenuKeys)*2)
	for _, menu := range WritableMenuKeys {
		out = append(out, WritePermission(menu), DeletePermission(menu))
	}
	return out
}

// IsActionPermission mengecek apakah key berbentuk "<menu>.write" atau
// "<menu>.delete" untuk menu yang bisa diubah.
func IsActionPermission(key string) bool {
	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return false
	}
	if parts[1] != "write" && parts[1] != "delete" {
		return false
	}
	for _, menu := range WritableMenuKeys {
		if menu == parts[0] {
			return true
		}
	}
	return false
}

// IsMenuPermission mengecek apakah key termasuk menu yang bisa diberikan.
func IsMenuPermission(key string) bool {
	for _, k := range MenuPermissionKeys {
		if k == key {
			return true
		}
	}
	return false
}

// IsSensitivePermission mengecek apakah key termasuk izin aksi sensitif.
func IsSensitivePermission(key string) bool {
	for _, k := range SensitivePermissionKeys {
		if k == key {
			return true
		}
	}
	return false
}

// IsGrantablePermission mengecek apakah key valid dan boleh disimpan di
// users.permissions. Menu lisensi sengaja tidak termasuk karena selalu
// dibatasi lewat role superadmin.
func IsGrantablePermission(key string) bool {
	return IsMenuPermission(key) || IsSensitivePermission(key) || IsActionPermission(key)
}

// DefaultPermissionsForRole mengembalikan daftar izin default untuk sebuah role.
//
// Superadmin mendapat SEMUA menu + izin sensitif agar akun pertama (owner)
// langsung punya akses penuh. Admin mendapat seluruh menu, sedangkan kasir
// hanya menu operasional.
func DefaultPermissionsForRole(role UserRole) []string {
	switch role {
	case UserRoleSuperAdmin:
		all := make([]string, 0, len(MenuPermissionKeys)+len(ActionPermissionKeys())+len(SensitivePermissionKeys))
		all = append(all, MenuPermissionKeys...)
		all = append(all, ActionPermissionKeys()...)
		all = append(all, SensitivePermissionKeys...)
		return all
	case UserRoleAdmin:
		// Admin: semua menu + aksi, termasuk pengaturan harga, KECUALI
		// lisensi (lisensi selalu hanya untuk superadmin/owner).
		all := make([]string, 0,
			len(MenuPermissionKeys)+
				len(ActionPermissionKeys())+
				len(SensitivePermissionKeys))
		all = append(all, MenuPermissionKeys...)
		all = append(all, ActionPermissionKeys()...)
		all = append(all, SensitivePermissionKeys...)
		return all
	default:
		permissions := make([]string, 0,
			len(RoleDefaultMenuKeys)+
				len(RoleDefaultWriteMenusKasir)+
				len(RoleDefaultDeleteMenusKasir))
		permissions = append(permissions, RoleDefaultMenuKeys...)
		for _, menu := range RoleDefaultWriteMenusKasir {
			permissions = append(permissions, WritePermission(menu))
		}
		for _, menu := range RoleDefaultDeleteMenusKasir {
			permissions = append(permissions, DeletePermission(menu))
		}
		return permissions
	}
}

// FilterGrantablePermissions membuang key yang tidak dikenal / tidak boleh
// diberikan, lalu menghilangkan duplikat. Urutan dipertahankan.
func FilterGrantablePermissions(keys []string) []string {
	seen := make(map[string]bool, len(keys))
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		if key == "" || seen[key] || !IsGrantablePermission(key) {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}

	// Izin aksi tidak boleh ada tanpa izin lihat menunya. Ini mencegah user
	// bisa memanggil endpoint tulis tetapi tidak bisa membuka menunya.
	for _, key := range append([]string(nil), out...) {
		if !IsActionPermission(key) {
			continue
		}
		menu := strings.SplitN(key, ".", 2)[0]
		if !seen[menu] {
			seen[menu] = true
			out = append(out, menu)
		}
	}
	return out
}

// WithoutSensitivePermissions membuang izin sensitif dari daftar.
func WithoutSensitivePermissions(keys []string) []string {
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		if IsSensitivePermission(key) {
			continue
		}
		out = append(out, key)
	}
	return out
}

// SensitivePermissionsFrom mengambil hanya izin sensitif dari daftar.
func SensitivePermissionsFrom(keys []string) []string {
	out := make([]string, 0, len(SensitivePermissionKeys))
	for _, key := range keys {
		if IsSensitivePermission(key) {
			out = append(out, key)
		}
	}
	return out
}
