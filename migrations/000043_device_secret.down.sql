ALTER TABLE consoles DROP COLUMN IF EXISTS device_secret_hash;
ALTER TABLE pending_devices DROP COLUMN IF EXISTS device_secret_hash;
