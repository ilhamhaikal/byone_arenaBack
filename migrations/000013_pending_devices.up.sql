-- Pairing TV: perangkat yang sudah menemukan server tetapi belum terhubung
-- ke konsol. Admin menghubungkan lewat aplikasi; server menulis IP perangkat
-- ke consoles.ip_address.
CREATE TABLE IF NOT EXISTS pending_devices (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ip          VARCHAR(50)  NOT NULL UNIQUE,
    name        VARCHAR(100) NOT NULL DEFAULT '',
    model       VARCHAR(100) NOT NULL DEFAULT '',
    app_version VARCHAR(30)  NOT NULL DEFAULT '',
    last_seen   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pending_devices_last_seen
    ON pending_devices (last_seen);

-- Konsol tipe "Switch"/"AndroidTV" dipakai oleh kode (entity), tetapi
-- constraint lama hanya mengizinkan bentuk uppercase. Longgarkan agar tidak
-- gagal saat membuat tipe tersebut.
ALTER TABLE consoles DROP CONSTRAINT IF EXISTS consoles_console_type_check;
ALTER TABLE consoles ADD CONSTRAINT consoles_console_type_check
    CHECK (console_type IN ('PS3', 'PS4', 'PS5', 'SWITCH', 'Switch', 'AndroidTV', 'ANDROIDTV'));