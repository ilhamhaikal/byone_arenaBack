-- Notifikasi manual (himbauan/promosi) tidak boleh muncul otomatis saat sesi
-- aktif. Notifikasi sistem (pesanan, perpanjangan, pembayaran tertunda) tetap
-- memakai auto_show = TRUE.
ALTER TABLE tv_notifications
    ADD COLUMN IF NOT EXISTS auto_show BOOLEAN NOT NULL DEFAULT TRUE;

-- Waktu terakhir admin menekan "Tampilkan" untuk notifikasi manual.
ALTER TABLE tv_notifications
    ADD COLUMN IF NOT EXISTS last_triggered_at TIMESTAMPTZ;

-- Data lama dianggap manual supaya tidak terus muncul sendiri. Notifikasi
-- sistem baru akan memakai default TRUE saat disisipkan stored procedure.
UPDATE tv_notifications SET auto_show = FALSE;
