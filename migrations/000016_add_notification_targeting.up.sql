-- Migration: 000016_add_notification_targeting.up.sql
-- Tambah kemampuan target spesifik konsol + sesi aktif untuk notifikasi

-- =============================================
-- 1. Tambah kolom target_console_ids (JSONB array of UUID)
-- =============================================
ALTER TABLE tv_notifications ADD COLUMN IF NOT EXISTS target_console_ids JSONB DEFAULT '[]';
ALTER TABLE tv_notifications ADD COLUMN IF NOT EXISTS active_sessions_only BOOLEAN NOT NULL DEFAULT FALSE;

-- =============================================
-- 2. Update byonePreviewPrice — handle voucher invalid gracefully (sudah ok)
--    Tidak ada perubahan SP
-- =============================================
