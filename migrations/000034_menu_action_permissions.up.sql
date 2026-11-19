-- =============================================================================
-- 000034_menu_action_permissions.up.sql
--
-- Menambahkan izin AKSI per menu:
--   <menu>.write   → boleh tambah/ubah data pada menu tersebut
--   <menu>.delete  → boleh menghapus data pada menu tersebut
--
-- Izin menu (mis. "member") tetap berarti "boleh melihat menu".
-- Dengan pemisahan ini, user bisa diberi akses LIHAT SAJA, atau lihat + ubah.
--
-- Default:
--   • superadmin/admin : semua aksi untuk menu yang mereka punya
--   • kasir            : hanya aksi operasional (rental, member, pesanan,
--                        notifikasi, booking, rental harian)
--
-- Migrasi ini juga mencabut menu Voucher dari kasir: halaman Voucher adalah
-- manajemen voucher (backend butuh izin voucher), sedangkan pengecekan kode
-- voucher saat transaksi tetap terbuka untuk semua user terautentikasi.
-- =============================================================================

-- ── Superadmin & admin: write + delete untuk menu yang mereka miliki ────────
UPDATE users u
SET permissions = (
    SELECT COALESCE(jsonb_agg(DISTINCT p), '[]'::jsonb)
    FROM (
        SELECT jsonb_array_elements_text(u.permissions) AS p
        UNION
        SELECT m.menu || s.suffix
        FROM jsonb_array_elements_text(u.permissions) AS m(menu)
        CROSS JOIN (VALUES ('.write'), ('.delete')) AS s(suffix)
        WHERE m.menu IN (
            'rental', 'console', 'member', 'discount', 'voucher',
            'promo_bonus', 'menu', 'food_order', 'notif', 'booking',
            'rental_harian', 'user_management'
        )
    ) q
)
WHERE u.role IN ('superadmin', 'admin')
  AND jsonb_typeof(u.permissions) = 'array';

-- ── Kasir: hanya aksi tambah/ubah pada menu operasional ─────────────────────
UPDATE users u
SET permissions = (
    SELECT COALESCE(jsonb_agg(DISTINCT p), '[]'::jsonb)
    FROM (
        SELECT jsonb_array_elements_text(u.permissions) AS p
        UNION
        SELECT m.menu || '.write'
        FROM jsonb_array_elements_text(u.permissions) AS m(menu)
        WHERE m.menu IN (
            'rental', 'member', 'food_order', 'notif', 'booking', 'rental_harian'
        )
    ) q
)
WHERE u.role = 'kasir'
  AND jsonb_typeof(u.permissions) = 'array';

-- ── Voucher bukan menu kasir (pengecekan kode tetap bisa) ───────────────────
UPDATE users
SET permissions = permissions - 'voucher'
WHERE role = 'kasir'
  AND jsonb_typeof(permissions) = 'array';
