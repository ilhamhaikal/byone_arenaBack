-- =============================================================================
-- 000036_console_price_permission.up.sql
--
-- Izin baru: "console.price"
--   • Boleh mengubah harga per jam, harga harian, dan tarif bertingkat konsol.
--   • Kasir TIDAK mendapat izin ini secara default, tetapi tetap bisa mengubah
--     data konsol lain (nama, tipe, status, deskripsi, IP).
--
-- Admin & superadmin mendapat izin ini otomatis.
-- =============================================================================

UPDATE users
SET permissions = permissions || '["console.price"]'::jsonb
WHERE role IN ('superadmin', 'admin')
  AND jsonb_typeof(permissions) = 'array'
  AND NOT (permissions ? 'console.price');
