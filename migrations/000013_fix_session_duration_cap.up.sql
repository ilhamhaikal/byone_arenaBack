-- Migration: 000013_fix_session_duration_cap.up.sql
-- Fix bug: durasi sesi tidak dibatasi booked_duration_minutes,
-- sehingga jika tidak di-end manual, durasi terus bertambah (72 jam untuk booking 1 jam).

CREATE OR REPLACE FUNCTION "byoneEndSession"(p_session_id UUID)
RETURNS TABLE (
    id UUID, console_id UUID, customer_id UUID,
    start_time TIMESTAMPTZ, end_time TIMESTAMPTZ,
    duration_minutes INT, total_price NUMERIC,
    status VARCHAR, notes TEXT,
    created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_session        sessions%ROWTYPE;
    v_price_per_hour NUMERIC;
    v_duration_min   INT;
    v_total_price    NUMERIC;
    v_now            TIMESTAMPTZ := NOW();
BEGIN
    SELECT * INTO v_session FROM sessions WHERE sessions.id = p_session_id FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'SESSION_NOT_FOUND: Sesi tidak ditemukan';
    END IF;
    IF v_session.status != 'active' THEN
        RAISE EXCEPTION 'SESSION_NOT_ACTIVE: Sesi sudah tidak aktif (status: %)', v_session.status;
    END IF;

    SELECT price_per_hour INTO v_price_per_hour FROM consoles WHERE consoles.id = v_session.console_id;

    -- Hitung durasi aktual (NOW - start)
    v_duration_min := EXTRACT(EPOCH FROM (v_now - v_session.start_time))::INT / 60;

    -- ⚠️  FIX: jika sesi memiliki booked_duration, cap durasi di booked duration.
    -- Ini mencegah durasi membengkak jika sesi tidak segera di-end (pre-paid).
    IF v_session.booked_duration_minutes > 0 THEN
        IF v_duration_min > v_session.booked_duration_minutes THEN
            v_duration_min := v_session.booked_duration_minutes;
        END IF;
    END IF;

    -- Hitung total price berdasarkan durasi yang sudah di-cap
    v_total_price := ROUND((v_duration_min::NUMERIC / 60.0) * v_price_per_hour, 2);

    -- Minimal 1 menit / minimal Rp 0
    IF v_duration_min < 1 THEN v_duration_min := 1; END IF;
    IF v_total_price  < 0 THEN v_total_price  := 0; END IF;

    UPDATE sessions
    SET end_time         = v_now,
        duration_minutes = v_duration_min,
        total_price      = v_total_price,
        status           = 'completed',
        updated_at       = v_now
    WHERE sessions.id = p_session_id;

    UPDATE consoles SET status = 'available', updated_at = v_now WHERE consoles.id = v_session.console_id;

    RETURN QUERY
        SELECT sessions.id, sessions.console_id, sessions.customer_id,
               sessions.start_time, sessions.end_time,
               sessions.duration_minutes, sessions.total_price,
               sessions.status, sessions.notes,
               sessions.created_at, sessions.updated_at
        FROM sessions WHERE sessions.id = p_session_id;
END;
$$;
