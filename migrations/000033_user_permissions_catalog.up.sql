-- =============================================================================
-- 000033_user_permissions_catalog.up.sql
--
-- 1) Menormalkan hak akses user (users.permissions) setelah penambahan menu
--    baru: Bonus Waktu (promo_bonus), Controller, Lisensi (khusus superadmin),
--    serta izin aksi sensitif Harga Member (member_price) & Harga Harian
--    (daily_price).
--
-- 2) Akun pertama / owner (superadmin) mendapat SEMUA akses secara eksplisit.
--    Sebelumnya permissions = '[]' dan frontend memakai fallback; sekarang
--    daftar izin tersimpan jelas di database.
--
-- 3) Memperbaiki __SP__SetAppSetting agar benar-benar UPSERT. Versi lama hanya
--    UPDATE, sehingga perubahan harga member/harian bisa "sukses" tanpa
--    mengubah apa pun bila baris setting belum ada.
--
-- Catatan: user dengan permissions non-kosong tetap dipertahankan; migrasi ini
-- hanya MENAMBAHKAN key menu baru yang belum ada, bukan menghapus izin custom.
-- =============================================================================

-- ── 1. SetAppSetting: upsert ────────────────────────────────────────────────
CREATE OR REPLACE FUNCTION "__SP__SetAppSetting"(p_key TEXT, p_value TEXT)
RETURNS VOID
LANGUAGE sql AS $$
    INSERT INTO app_settings (key, value, updated_at)
    VALUES (p_key, p_value, NOW())
    ON CONFLICT (key) DO UPDATE
       SET value = EXCLUDED.value,
           updated_at = NOW();
$$;

-- ── 2. Default lengkap per role ─────────────────────────────────────────────

-- Superadmin: seluruh menu + izin sensitif.
UPDATE users
SET permissions = '[
  "dashboard","rental","console","tv_log","member","discount","voucher",
  "promo_bonus","menu","food_order","notif","report","booking","rental_harian",
  "user_management","controller","member_price","daily_price"
]'::jsonb
WHERE role = 'superadmin'
  AND (permissions IS NULL OR permissions = '[]'::jsonb);

-- Admin: seluruh menu (tanpa izin sensitif).
UPDATE users
SET permissions = '[
  "dashboard","rental","console","tv_log","member","discount","voucher",
  "promo_bonus","menu","food_order","notif","report","booking","rental_harian",
  "user_management","controller"
]'::jsonb
WHERE role = 'admin'
  AND (permissions IS NULL OR permissions = '[]'::jsonb);

-- Kasir: menu operasional saja.
UPDATE users
SET permissions = '[
  "dashboard","rental","console","tv_log","member","discount","voucher",
  "menu","food_order","notif","booking","rental_harian","controller"
]'::jsonb
WHERE role = 'kasir'
  AND (permissions IS NULL OR permissions = '[]'::jsonb);

-- ── 3. Tambahkan menu baru ke daftar izin yang sudah ada ────────────────────

-- Superadmin: menu baru + izin sensitif baru.
UPDATE users
SET permissions = (
    SELECT COALESCE(jsonb_agg(DISTINCT t.p), '[]'::jsonb)
    FROM jsonb_array_elements_text(
        permissions || '["promo_bonus","controller","member_price","daily_price"]'::jsonb
    ) AS t(p)
)
WHERE role = 'superadmin'
  AND permissions <> '[]'::jsonb;

-- Admin: menu baru (Bonus Waktu & Controller).
UPDATE users
SET permissions = (
    SELECT COALESCE(jsonb_agg(DISTINCT t.p), '[]'::jsonb)
    FROM jsonb_array_elements_text(
        permissions || '["promo_bonus","controller"]'::jsonb
    ) AS t(p)
)
WHERE role = 'admin'
  AND permissions <> '[]'::jsonb;

-- Kasir: lengkapi menu operasional yang dulu hanya didapat dari fallback
-- frontend (Log TV, Notifikasi, Booking, Rental Harian, Controller).
-- Bonus waktu / laporan / voucher / manajemen user tetap TIDAK diberikan.
UPDATE users
SET permissions = (
    SELECT COALESCE(jsonb_agg(DISTINCT t.p), '[]'::jsonb)
    FROM jsonb_array_elements_text(
        permissions || '["tv_log","notif","booking","rental_harian","controller"]'::jsonb
    ) AS t(p)
)
WHERE role = 'kasir'
  AND permissions <> '[]'::jsonb;
