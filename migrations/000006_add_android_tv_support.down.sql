-- Migration: 000006_add_android_tv_support.down.sql

-- Kembalikan byoneStartSession ke versi lama (3 parameter)
DROP FUNCTION IF EXISTS byoneStartSession(UUID, UUID, TEXT, INTEGER);

CREATE OR REPLACE FUNCTION byoneStartSession(
    p_console_id  UUID,
    p_customer_id UUID,
    p_notes       TEXT
)
RETURNS TABLE (
    id               UUID,
    console_id       UUID,
    customer_id      UUID,
    start_time       TIMESTAMPTZ,
    end_time         TIMESTAMPTZ,
    duration_minutes INT,
    total_price      NUMERIC,
    status           VARCHAR,
    notes            TEXT,
    created_at       TIMESTAMPTZ,
    updated_at       TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_session_id UUID;
    v_status     VARCHAR;
BEGIN
    SELECT c.status INTO v_status FROM consoles c WHERE c.id = p_console_id FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'CONSOLE_NOT_FOUND: Konsol tidak ditemukan';
    END IF;
    IF v_status != 'available' THEN
        RAISE EXCEPTION 'CONSOLE_NOT_AVAILABLE: Konsol tidak tersedia (status: %)', v_status;
    END IF;
    IF EXISTS (SELECT 1 FROM sessions s WHERE s.console_id = p_console_id AND s.status = 'active') THEN
        RAISE EXCEPTION 'SESSION_ALREADY_ACTIVE: Konsol masih memiliki sesi aktif';
    END IF;
    v_session_id := uuid_generate_v4();
    INSERT INTO sessions (id, console_id, customer_id, start_time, status, notes, created_at, updated_at)
    VALUES (v_session_id, p_console_id, p_customer_id, NOW(), 'active', p_notes, NOW(), NOW());
    UPDATE consoles SET status = 'in_use', updated_at = NOW() WHERE id = p_console_id;
    RETURN QUERY
        SELECT s.id, s.console_id, s.customer_id, s.start_time, s.end_time,
               s.duration_minutes, s.total_price, s.status, s.notes, s.created_at, s.updated_at
        FROM sessions s WHERE s.id = v_session_id;
END;
$$;

DROP INDEX IF EXISTS idx_sessions_end_scheduled_at;
ALTER TABLE sessions DROP COLUMN IF EXISTS end_scheduled_at;
ALTER TABLE sessions DROP COLUMN IF EXISTS booked_duration_minutes;

ALTER TABLE consoles DROP CONSTRAINT IF EXISTS consoles_console_type_check;
ALTER TABLE consoles ADD CONSTRAINT consoles_console_type_check
    CHECK (console_type IN ('PS3', 'PS4', 'PS5'));
ALTER TABLE consoles DROP COLUMN IF EXISTS ip_address;
