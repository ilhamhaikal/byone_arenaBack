-- =============================================================================
-- 000035_seed_default_users.up.sql
--
-- Setup pertama kali menyediakan 3 akun dengan level berbeda:
--
--   ┌─────────────┬────────────┬─────────────────────────────────────────────┐
--   │ Username    │ Role       │ Akses                                       │
--   ├─────────────┼────────────┼─────────────────────────────────────────────┤
--   │ owner       │ superadmin │ SEMUA akses (termasuk lisensi)              │
--   │ admin       │ admin      │ SEMUA menu & aksi KECUALI lisensi           │
--   │ kasir       │ kasir      │ Dashboard, Konsol (CRUD), Controller,       │
--   │             │            │ Member, semua menu Penjualan & Rental       │
--   └─────────────┴────────────┴─────────────────────────────────────────────┘
--
-- Password awal:
--   owner / owner123   (sudah dibuat oleh migrasi 000012)
--   admin / admin123
--   kasir / kasir123
--
-- ⚠️  WAJIB ganti password setelah login pertama.
-- Hash bcrypt cost 10 di bawah adalah untuk password tersebut.
-- =============================================================================

-- ── 1. Akun ADMIN ───────────────────────────────────────────────────────────
-- Semua menu + semua aksi + pengaturan harga, tanpa lisensi.
INSERT INTO users (username, password, full_name, role, is_active, permissions)
VALUES (
    'admin',
    '$2a$10$Z.GNYkJZzsXPU32TIAHJDeFKM9h3EemqfW.n9AahxLy2Hcs6N72du',
    'Admin',
    'admin',
    TRUE,
    '[
      "dashboard","rental","console","tv_log","member","discount","voucher",
      "promo_bonus","menu","food_order","notif","report","booking",
      "rental_harian","user_management","controller",
      "rental.write","rental.delete","console.write","console.delete",
      "member.write","member.delete","discount.write","discount.delete",
      "voucher.write","voucher.delete","promo_bonus.write","promo_bonus.delete",
      "menu.write","menu.delete","food_order.write","food_order.delete",
      "notif.write","notif.delete","booking.write","booking.delete",
      "rental_harian.write","rental_harian.delete",
      "user_management.write","user_management.delete",
      "member_price","daily_price"
    ]'::jsonb
)
ON CONFLICT (username) DO NOTHING;

-- ── 2. Akun KASIR ───────────────────────────────────────────────────────────
-- Hanya Konsol yang mendapat CRUD penuh. Menu Penjualan/Rental untuk operasional.
INSERT INTO users (username, password, full_name, role, is_active, permissions)
VALUES (
    'kasir',
    '$2a$10$gIrr/GOjLLi1AdYm7Y4.FOa1Xjg0fhaLUtehB6Y/LKHQAJgm4ocIm',
    'Kasir',
    'kasir',
    TRUE,
    '[
      "dashboard","rental","console","member","menu","food_order","booking",
      "rental_harian","controller",
      "rental.write","console.write","member.write","food_order.write",
      "booking.write","rental_harian.write","console.delete"
    ]'::jsonb
)
ON CONFLICT (username) DO NOTHING;

-- Kasir wajib punya shift aktif untuk bisa login. Buat shift 24 jam default.
INSERT INTO shifts (user_id, name, start_hour, end_hour, is_24_hour, status)
SELECT u.id, 'Shift Default 24 Jam', 0, 23, TRUE, 'active'
FROM users u
WHERE u.username = 'kasir'
  AND NOT EXISTS (
      SELECT 1 FROM shifts s WHERE s.user_id = u.id
  );

-- ── 3. Samakan admin lama dengan kebijakan baru ─────────────────────────────
-- Admin sekarang juga boleh mengatur harga member/harian (bukan lisensi).
UPDATE users
SET permissions = (
    SELECT COALESCE(jsonb_agg(DISTINCT t.p), '[]'::jsonb)
    FROM jsonb_array_elements_text(
        permissions || '["member_price","daily_price"]'::jsonb
    ) AS t(p)
)
WHERE role = 'admin'
  AND jsonb_typeof(permissions) = 'array';

-- ── 4. Normalisasi kasir lama yang masih memakai default sebelumnya ─────────
-- Hanya menyentuh permission kosong atau yang persis sama dengan default lama,
-- sehingga izin custom tetap dipertahankan.
UPDATE users
SET permissions = '[
  "dashboard","rental","console","member","menu","food_order","booking",
  "rental_harian","controller",
  "rental.write","console.write","member.write","food_order.write",
  "booking.write","rental_harian.write","console.delete"
]'::jsonb
WHERE role = 'kasir'
  AND (
      permissions = '[]'::jsonb
      OR (
          SELECT jsonb_agg(x ORDER BY x)
          FROM jsonb_array_elements_text(permissions) AS t(x)
      ) = (
          SELECT jsonb_agg(x ORDER BY x)
          FROM jsonb_array_elements_text('[
            "dashboard","rental","console","tv_log","member","discount","menu",
            "food_order","notif","booking","rental_harian","controller",
            "rental.write","member.write","food_order.write","notif.write",
            "booking.write","rental_harian.write"
          ]'::jsonb) AS t(x)
      )
  );
