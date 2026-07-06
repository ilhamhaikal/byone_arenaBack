-- Migration: 000012_add_tv_control_and_notifications.up.sql
-- Sistem kontrol TV + notifikasi promosi ke Android TV

-- =============================================
-- 1. Konsol: tambah screen_status + control fields
-- =============================================
ALTER TABLE consoles ADD COLUMN IF NOT EXISTS screen_status VARCHAR(20) NOT NULL DEFAULT 'off'
    CHECK (screen_status IN ('on', 'off', 'screensaver'));

ALTER TABLE consoles ADD COLUMN IF NOT EXISTS adb_port INTEGER DEFAULT 5555;
ALTER TABLE consoles ADD COLUMN IF NOT EXISTS mac_address VARCHAR(20);

-- =============================================
-- 2. Tabel tv_notifications — notifikasi promosi
-- =============================================
CREATE TABLE IF NOT EXISTS tv_notifications (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title           VARCHAR(100) NOT NULL,              -- judul notifikasi
    message         TEXT NOT NULL,                      -- isi pesan
    image_url       VARCHAR(500),                       -- URL gambar promo (opsional)
    priority        VARCHAR(20) NOT NULL DEFAULT 'normal'
                        CHECK (priority IN ('low', 'normal', 'high')),
    -- Loop configuration
    loop_enabled    BOOLEAN NOT NULL DEFAULT FALSE,     -- aktifkan looping
    loop_interval   INT NOT NULL DEFAULT 30,            -- interval dalam detik (min 5)
    -- Targeting
    target_all      BOOLEAN NOT NULL DEFAULT TRUE,      -- kirim ke semua TV
    target_console_type VARCHAR(15),                     -- filter tipe konsol (PS3/PS4/PS5/AndroidTV)
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tv_notifications_active ON tv_notifications(is_active);
CREATE INDEX IF NOT EXISTS idx_tv_notifications_loop ON tv_notifications(loop_enabled, is_active);
