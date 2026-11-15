-- Auto-end sesi saat waktu sewa habis, dijalankan oleh server.
--
-- Sebelumnya auto-end hanya ada di aplikasi Flutter (RentalScreen). Bila
-- aplikasi/PC mati, tidak ada yang mengakhiri sesi sehingga status tetap
-- 'active' dan durasi terus dihitung dari start_time.
--
-- Fungsi ini menutup sesi tepat pada end_scheduled_at tanpa tagihan tambahan.

CREATE OR REPLACE FUNCTION "__SP__AutoEndSession"(p_session_id UUID)
RETURNS TABLE (id UUID, console_id UUID, status VARCHAR)
LANGUAGE plpgsql AS $$
DECLARE
    v_session sessions%ROWTYPE;
    v_now     TIMESTAMPTZ := NOW();
    v_end     TIMESTAMPTZ;
    v_min     INT;
BEGIN
    SELECT * INTO v_session FROM sessions WHERE sessions.id = p_session_id FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'SESSION_NOT_FOUND';
    END IF;
    IF v_session.status <> 'active' THEN
        RAISE EXCEPTION 'SESSION_NOT_ACTIVE: %', v_session.status;
    END IF;

    v_end := COALESCE(v_session.end_scheduled_at, v_now);
    v_min := GREATEST(EXTRACT(EPOCH FROM (v_end - v_session.start_time))::INT / 60, 1);

    UPDATE sessions
       SET end_time = v_end,
           duration_minutes = v_min,
           status = 'completed',
           notes = TRIM(COALESCE(notes, '') || ' [auto-end: waktu habis]'),
           updated_at = v_now
     WHERE sessions.id = p_session_id;

    UPDATE consoles
       SET status = 'available', updated_at = v_now
     WHERE consoles.id = v_session.console_id;

    INSERT INTO tv_activity_logs (id, console_id, event, session_id, is_authorized, duration_minutes, created_at)
    VALUES (uuid_generate_v4(), v_session.console_id, 'off', p_session_id, TRUE, v_min, v_now);

    RETURN QUERY
        SELECT s.id, s.console_id, s.status FROM sessions s WHERE s.id = p_session_id;
END;
$$;