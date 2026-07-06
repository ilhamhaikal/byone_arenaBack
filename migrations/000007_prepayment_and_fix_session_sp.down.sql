-- Migration: 000007_prepayment_and_fix_session_sp.down.sql
DROP FUNCTION IF EXISTS byoneStartSessionWithPayment(UUID, UUID, TEXT, INTEGER, NUMERIC, VARCHAR);
DROP FUNCTION IF EXISTS byoneStartSession(UUID, UUID, TEXT, INTEGER);

-- Kembalikan byoneStartSession dari migration 000006
CREATE OR REPLACE FUNCTION byoneStartSession(
    p_console_id              UUID,
    p_customer_id             UUID,
    p_notes                   TEXT,
    p_booked_duration_minutes INTEGER DEFAULT 0
)
RETURNS TABLE (
    id UUID, console_id UUID, customer_id UUID,
    start_time TIMESTAMPTZ, end_time TIMESTAMPTZ, end_scheduled_at TIMESTAMPTZ,
    booked_duration_minutes INT, duration_minutes INT,
    total_price NUMERIC, status VARCHAR, notes TEXT,
    created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ
)
LANGUAGE plpgsql AS $$
DECLARE v_session_id UUID; v_status VARCHAR; v_end_scheduled TIMESTAMPTZ;
BEGIN
    SELECT c.status INTO v_status FROM consoles c WHERE c.id = p_console_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'CONSOLE_NOT_FOUND: Konsol tidak ditemukan'; END IF;
    IF v_status != 'available' THEN RAISE EXCEPTION 'CONSOLE_NOT_AVAILABLE: Konsol tidak tersedia (status: %)', v_status; END IF;
    IF EXISTS (SELECT 1 FROM sessions s2 WHERE s2.console_id = p_console_id AND s2.status = 'active') THEN
        RAISE EXCEPTION 'SESSION_ALREADY_ACTIVE: Konsol masih memiliki sesi aktif'; END IF;
    IF p_booked_duration_minutes > 0 THEN v_end_scheduled := NOW() + (p_booked_duration_minutes * INTERVAL '1 minute'); END IF;
    v_session_id := uuid_generate_v4();
    INSERT INTO sessions (id, console_id, customer_id, start_time, booked_duration_minutes, end_scheduled_at, status, notes, created_at, updated_at)
    VALUES (v_session_id, p_console_id, p_customer_id, NOW(), p_booked_duration_minutes, v_end_scheduled, 'active', p_notes, NOW(), NOW());
    UPDATE consoles SET status = 'in_use', updated_at = NOW() WHERE id = p_console_id;
    RETURN QUERY SELECT s.id, s.console_id, s.customer_id, s.start_time, s.end_time, s.end_scheduled_at,
        s.booked_duration_minutes, s.duration_minutes, s.total_price, s.status, s.notes, s.created_at, s.updated_at
    FROM sessions s WHERE s.id = v_session_id;
END; $$;
