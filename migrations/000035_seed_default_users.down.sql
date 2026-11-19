-- =============================================================================
-- 000035_seed_default_users.down.sql
-- =============================================================================

-- Hapus shift lalu akun default yang dibuat migrasi ini.
DELETE FROM shifts
WHERE user_id IN (SELECT id FROM users WHERE username IN ('admin', 'kasir'));

DELETE FROM users WHERE username IN ('admin', 'kasir');

-- Kembalikan admin ke kebijakan sebelumnya (tanpa pengaturan harga).
UPDATE users
SET permissions = permissions - 'member_price' - 'daily_price'
WHERE role = 'admin'
  AND jsonb_typeof(permissions) = 'array';
