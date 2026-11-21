-- =============================================================================
-- 000038_console_manual_live.up.sql
--
-- Menandai TV yang sengaja dinyalakan admin TANPA sesi (live manual) untuk
-- kebutuhan mematikan konsol/PS.
--
--   manual_live = TRUE  → client tetap di HDMI, tanpa UI "belum ada sesi"
--   manual_live = FALSE → jika tidak ada sesi, client kembali ke screensaver
--
-- StartSession / EndSession / Sleep selalu mengembalikan manual_live = FALSE.
-- =============================================================================

ALTER TABLE consoles
    ADD COLUMN IF NOT EXISTS manual_live BOOLEAN NOT NULL DEFAULT FALSE;
