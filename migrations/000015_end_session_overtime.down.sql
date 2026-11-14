-- Kembalikan perilaku lama: durasi dipotong ke booked (overtime gratis).
-- Catatan: migrasi ini TIDAK menghapus payment 'pending' kelebihan waktu yang
-- sudah terlanjur dibuat. Periksa manual sebelum rollback produksi.

CREATE OR REPLACE FUNCTION "__SP__EndSession"(p_session_id UUID)
RETURNS TABLE (
    id UUID, console_id UUID, customer_id UUID, start_time TIMESTAMPTZ,
    end_time TIMESTAMPTZ, duration_minutes INT, total_price NUMERIC,
    status VARCHAR, notes TEXT, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ
) LANGUAGE plpgsql AS $$
DECLARE
    v_session       sessions%ROWTYPE;
    v_price_per_hour NUMERIC;
    v_duration_min  INT;
    v_total_price   NUMERIC;
    v_now           TIMESTAMPTZ := NOW();
BEGIN
    SELECT * INTO v_session FROM sessions WHERE sessions.id = p_session_id FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'SESSION_NOT_FOUND';
    END IF;
    IF v_session.status <> 'active' THEN
        RAISE EXCEPTION 'SESSION_NOT_ACTIVE: %', v_session.status;
    END IF;

    SELECT price_per_hour INTO v_price_per_hour FROM consoles WHERE consoles.id = v_session.console_id;

    v_duration_min := EXTRACT(EPOCH FROM (v_now - v_session.start_time))::INT / 60;
    IF v_session.booked_duration_minutes > 0 AND v_duration_min > v_session.booked_duration_minutes THEN
        v_duration_min := v_session.booked_duration_minutes;
    END IF;

    IF v_session.booked_duration_minutes > 0 AND COALESCE(v_session.total_price, 0) > 0 THEN
        v_total_price := v_session.total_price;
    ELSIF v_session.booked_duration_minutes > 0 THEN
        SELECT pc.base_amount INTO v_total_price
        FROM "__SP__CalculatePrice"(v_session.console_id, v_session.booked_duration_minutes) pc;
    ELSE
        v_total_price := ROUND((v_duration_min::NUMERIC / 60.0) * v_price_per_hour, 0);
    END IF;

    IF v_duration_min < 1 THEN
        v_duration_min := 1;
    END IF;
    IF v_total_price < 0 THEN
        v_total_price := 0;
    END IF;

    UPDATE sessions
       SET end_time = v_now, duration_minutes = v_duration_min,
           total_price = v_total_price, status = 'completed', updated_at = v_now
     WHERE sessions.id = p_session_id;

    UPDATE consoles SET status = 'available', updated_at = v_now WHERE consoles.id = v_session.console_id;

    INSERT INTO tv_activity_logs (id, console_id, event, session_id, is_authorized, duration_minutes, created_at)
    VALUES (uuid_generate_v4(), v_session.console_id, 'off', p_session_id, TRUE, v_duration_min, v_now);

    RETURN QUERY
        SELECT s.id, s.console_id, s.customer_id, s.start_time, s.end_time,
               s.duration_minutes, s.total_price, s.status, s.notes,
               s.created_at, s.updated_at
          FROM sessions s WHERE s.id = p_session_id;
END;
$$;