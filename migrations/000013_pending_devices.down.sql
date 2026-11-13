DROP TABLE IF EXISTS pending_devices;

ALTER TABLE consoles DROP CONSTRAINT IF EXISTS consoles_console_type_check;
ALTER TABLE consoles ADD CONSTRAINT consoles_console_type_check
    CHECK (console_type IN ('PS3', 'PS4', 'PS5', 'SWITCH')) NOT VALID;