-- Down migration sengaja tidak mengembalikan SP ke versi lama yang bug.
-- Kolom is_package juga dipertahankan karena backend versi sekarang membacanya.
-- Yang dilepas hanya index bantu.
DROP INDEX IF EXISTS idx_sessions_is_package;