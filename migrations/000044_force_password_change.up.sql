-- Akun seed default wajib mengganti password sebelum bisa memakai API.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT FALSE;

-- Hanya hash password default yang persis sama yang ditandai. Akun yang
-- password-nya sudah diganti tidak terpengaruh.
UPDATE users
SET must_change_password = TRUE
WHERE password IN (
    '$2a$10$wzuagVfPhmuV/WWn8fTl1uuQY17S4eLiPj9UjdCTIAsXRmS108vFq', -- owner123
    '$2a$10$Z.GNYkJZzsXPU32TIAHJDeFKM9h3EemqfW.n9AahxLy2Hcs6N72du', -- admin123
    '$2a$10$gIrr/GOjLLi1AdYm7Y4.FOa1Xjg0fhaLUtehB6Y/LKHQAJgm4ocIm'  -- kasir123
);

-- Shift 24 jam default untuk kasir bukan jadwal operasional nyata.
DELETE FROM shifts
WHERE name = 'Shift Default 24 Jam'
  AND user_id IN (SELECT id FROM users WHERE username = 'kasir');
