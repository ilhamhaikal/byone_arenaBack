-- Migration: 000006_add_android_tv_support.up.sql
-- Tambah dukungan Android TV: ip_address, tipe AndroidTV, durasi pre-book, dan jadwal selesai sesi

-- =============================================
-- 1. Konsol: tambah kolom ip_address
-- =============================================
ALTER TABLE consoles ADD COLUMN IF NOT EXISTS ip_address VARCHAR(50);

-- Perluas constraint tipe konsol agar mendukung AndroidTV
ALTER TABLE consoles DROP CONSTRAINT IF EXISTS consoles_console_type_check;
ALTER TABLE consoles ADD CONSTRAINT consoles_console_type_check
    CHECK (console_type IN ('PS3', 'PS4', 'PS5', 'AndroidTV'));

-- =============================================
-- 2. Sesi: tambah durasi pre-book & jadwal selesai
-- =============================================
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS booked_duration_minutes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS end_scheduled_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_sessions_end_scheduled_at ON sessions(end_scheduled_at);

-- =============================================
-- 3. Update sp_start_session agar terima durasi pre-book
-- =============================================
DROP FUNCTION IF EXISTS sp_start_session(UUID, UUID, TEXT);

CREATE OR REPLACE FUNCTION sp_start_session(
    p_console_id              UUID,
    p_customer_id             UUID,   -- nullable (walk-in)
    p_notes                   TEXT,
    p_booked_duration_minutes INTEGER DEFAULT 0
)
RETURNS TABLE (
    id                       UUID,
    console_id               UUID,
    customer_id              UUID,
    start_time               TIMESTAMPTZ,
    end_time                 TIMESTAMPTZ,
    end_scheduled_at         TIMESTAMPTZ,
    booked_duration_minutes  INT,
    duration_minutes         INT,
    total_price              NUMERIC,
    status                   VARCHAR,
    notes                    TEXT,
    created_at               TIMESTAMPTZ,
    updated_at               TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_session_id    UUID;
    v_status        VARCHAR;
    v_end_scheduled TIMESTAMPTZ;
BEGIN
    -- Cek ketersediaan konsol (dengan row lock untuk hindari race condition)
    SELECT c.status INTO v_status
    FROM consoles c
    WHERE c.id = p_console_id
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'CONSOLE_NOT_FOUND: Konsol tidak ditemukan';
    END IF;

    IF v_status != 'available' THEN
        RAISE EXCEPTION 'CONSOLE_NOT_AVAILABLE: Konsol tidak tersedia (status: %)', v_status;
    END IF;

    -- Cek sesi aktif yang mungkin masih ada (data integrity)
    IF EXISTS (
        SELECT 1 FROM sessions s
        WHERE s.console_id = p_console_id AND s.status = 'active'
    ) THEN
        RAISE EXCEPTION 'SESSION_ALREADY_ACTIVE: Konsol masih memiliki sesi aktif';
    END IF;

    -- Hitung jadwal selesai jika durasi diberikan (> 0)
    IF p_booked_duration_minutes > 0 THEN
        v_end_scheduled := NOW() + (p_booked_duration_minutes * INTERVAL '1 minute');
    END IF;

    -- Buat sesi baru
    v_session_id := uuid_generate_v4();

    INSERT INTO sessions (
        id, console_id, customer_id,
        start_time, booked_duration_minutes, end_scheduled_at,
        status, notes, created_at, updated_at
    )
    VALUES (
        v_session_id, p_console_id, p_customer_id,
        NOW(), p_booked_duration_minutes, v_end_scheduled,
        'active', p_notes, NOW(), NOW()
    );

    -- Tandai konsol sebagai sedang digunakan
    UPDATE consoles
    SET status = 'in_use', updated_at = NOW()
    WHERE id = p_console_id;

    -- Kembalikan data sesi yang baru dibuat
    RETURN QUERY
        SELECT s.id, s.console_id, s.customer_id,
               s.start_time, s.end_time, s.end_scheduled_at, s.booked_duration_minutes,
               s.duration_minutes, s.total_price, s.status, s.notes,
               s.created_at, s.updated_at
        FROM sessions s
        WHERE s.id = v_session_id;
END;
$$;
