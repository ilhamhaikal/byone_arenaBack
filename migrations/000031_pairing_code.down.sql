-- =============================================================================
-- 000031_pairing_code.down.sql
-- =============================================================================

DROP INDEX IF EXISTS idx_consoles_device_id;
ALTER TABLE consoles DROP COLUMN IF EXISTS device_id;

DROP INDEX IF EXISTS idx_pending_devices_pairing_code;
DROP INDEX IF EXISTS idx_pending_devices_device_id;
DROP INDEX IF EXISTS idx_pending_devices_ip;

ALTER TABLE pending_devices
    DROP COLUMN IF EXISTS code_expires_at,
    DROP COLUMN IF EXISTS pairing_code,
    DROP COLUMN IF EXISTS device_id;

CREATE UNIQUE INDEX IF NOT EXISTS pending_devices_ip_key ON pending_devices (ip);
