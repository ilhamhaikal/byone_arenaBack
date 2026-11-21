-- Secret perangkat TV (dibuat di TV). Server hanya menyimpan hash SHA-256.
ALTER TABLE pending_devices
    ADD COLUMN IF NOT EXISTS device_secret_hash VARCHAR(64);

ALTER TABLE consoles
    ADD COLUMN IF NOT EXISTS device_secret_hash VARCHAR(64);
