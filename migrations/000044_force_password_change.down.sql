-- Shift default yang dihapus tidak dibuat ulang otomatis (jadwal bersifat data
-- operasional). Rollback hanya mengembalikan skema.
ALTER TABLE users DROP COLUMN IF EXISTS must_change_password;
