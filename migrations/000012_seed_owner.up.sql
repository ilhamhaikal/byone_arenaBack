-- ============================================
-- 12. SEED AKUN OWNER
-- ============================================
-- Membuat 1 akun awal agar sistem bisa dipakai setelah migrasi pertama.
-- Idempotent: aman dijalankan berulang (ON CONFLICT username -> skip).
--
-- Kredensial awal:
--   Username : owner
--   Password : owner123
--   Role     : superadmin (akses penuh / owner)
--
-- ⚠️  WAJIB ganti password setelah login pertama.
-- Hash di bawah adalah bcrypt cost 10 dari "owner123".
-- ============================================

INSERT INTO users (username, password, full_name, role, is_active, permissions)
VALUES (
    'owner',
    '$2a$10$wzuagVfPhmuV/WWn8fTl1uuQY17S4eLiPj9UjdCTIAsXRmS108vFq',
    'Owner',
    'superadmin',
    TRUE,
    '[]'::jsonb
)
ON CONFLICT (username) DO NOTHING;
