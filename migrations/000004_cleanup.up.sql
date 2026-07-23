-- =============================================================================
-- 000004_cleanup.up.sql — Data cleanup untuk fresh setup
-- Bersihkan notifikasi stale (dari 000029)
-- =============================================================================

-- Nonaktifkan semua notifikasi "Sesi Diperpanjang" yang tersisa
UPDATE tv_notifications
SET is_active = false, updated_at = NOW()
WHERE title = 'Sesi Diperpanjang' AND is_active = true;

-- Nonaktifkan semua notifikasi "Pembayaran Tertunda" yang tersisa
UPDATE tv_notifications
SET is_active = false, updated_at = NOW()
WHERE title = 'Pembayaran Tertunda' AND is_active = true;
