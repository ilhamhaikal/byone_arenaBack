-- =============================================================================
-- 000031_pairing_code.up.sql
--
-- Pairing TV dengan KODE 6 DIGIT (menggantikan alur "set IP statis").
--
-- Alur:
--   1. TV install & buka aplikasi -> POST /devices/announce
--   2. Server membuat kode 6 digit (berlaku 15 menit) -> TV menampilkannya
--   3. Admin mengetik kode di aplikasi admin -> POST /devices/bind-by-code
--   4. Server mengikat konsol ke device_id + IP perangkat.
--
-- Identitas perangkat memakai device_id (UUID yang dibuat aplikasi TV), bukan
-- IP, sehingga TV yang IP-nya berubah (DHCP) tetap dikenali.
-- =============================================================================

ALTER TABLE pending_devices
    ADD COLUMN IF NOT EXISTS device_id       UUID,
    ADD COLUMN IF NOT EXISTS pairing_code    VARCHAR(6),
    ADD COLUMN IF NOT EXISTS code_expires_at TIMESTAMPTZ;

-- IP tidak lagi menjadi kunci unik: satu IP bisa dipakai ulang DHCP oleh
-- perangkat lain, dan identitas kini memakai device_id.
ALTER TABLE pending_devices DROP CONSTRAINT IF EXISTS pending_devices_ip_key;
CREATE INDEX IF NOT EXISTS idx_pending_devices_ip ON pending_devices (ip);

CREATE UNIQUE INDEX IF NOT EXISTS idx_pending_devices_device_id
    ON pending_devices (device_id) WHERE device_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_pending_devices_pairing_code
    ON pending_devices (pairing_code) WHERE pairing_code IS NOT NULL;

-- Konsol menyimpan device_id TV yang terhubung (dipakai saat bind & cek status).
ALTER TABLE consoles ADD COLUMN IF NOT EXISTS device_id UUID;
CREATE UNIQUE INDEX IF NOT EXISTS idx_consoles_device_id
    ON consoles (device_id) WHERE device_id IS NOT NULL;
