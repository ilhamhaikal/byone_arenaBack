-- Safety net untuk sesi yang terlupakan / TV mati.
--
-- Masalah: bila aplikasi kasir mati atau TV tidak pernah diakhiri, sesi tetap
-- "active" berjam-jam. Saat akhirnya diakhiri, EndSession menghitung kelebihan
-- waktu berdasarkan jam dinding (bisa puluhan jam) sehingga muncul tagihan
-- besar yang tidak wajar.
--
-- Perbaikan:
--   1. Kelebihan waktu dibatasi `app_settings.max_overtime_minutes`
--      (default 120 menit). Durasi aktual tetap dicatat.
--   2. Fungsi ForceCloseSession menutup sesi tanpa tagihan tambahan.
--   3. Server menjalankan janitor untuk menutup sesi yang telat > 6 jam dan
--      TV-nya tidak mengirim heartbeat > 30 menit.

INSERT INTO app_settings (key, value)
VALUES ('max_overtime_minutes', '120')
ON CONFLICT (key) DO NOTHING;

CREATE OR REPLACE FUNCTION "__SP__EndSession"(p_session_id UUID)
RETURNS TABLE (
    id UUID, console_id UUID, customer_id UUID, start_time TIMESTAMPTZ,
    end_time TIMESTAMPTZ, duration_minutes INT, total_price NUMERIC,
    status VARCHAR, notes TEXT, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ
) LANGUAGE plpgsql AS $$
DECLARE
    v_session        sessions%ROWTYPE;
    v_duration_min   INT;
    v_now            TIMESTAMPTZ := NOW();
    v_paid_min       INT;
    v_allowed_min    INT;
    v_max_overtime   INT := 120;
    v_billable_min   INT;
    v_base_amount    NUMERIC(10,2) := 0;
    v_allowed_amt    NUMERIC(10,2) := 0;
    v_billable_amt   NUMERIC(10,2) := 0;
    v_overtime       NUMERIC(10,2) := 0;
    v_total_price    NUMERIC(10,2) := 0;
    v_capped         BOOLEAN := FALSE;
    v_note           TEXT;
BEGIN
    SELECT * INTO v_session FROM sessions WHERE sessions.id = p_session_id FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'SESSION_NOT_FOUND';
    END IF;
    IF v_session.status <> 'active' THEN
        RAISE EXCEPTION 'SESSION_NOT_ACTIVE: %', v_session.status;
    END IF;

    SELECT NULLIF(TRIM(value), '')::INT INTO v_max_overtime
      FROM app_settings WHERE key = 'max_overtime_minutes';
    IF v_max_overtime IS NULL OR v_max_overtime < 0 THEN
        v_max_overtime := 120;
    END IF;

    v_duration_min := EXTRACT(EPOCH FROM (v_now - v_session.start_time))::INT / 60;
    IF v_duration_min < 1 THEN
        v_duration_min := 1;
    END IF;

    v_paid_min := COALESCE(NULLIF(v_session.paid_duration_minutes, 0), v_session.booked_duration_minutes);
    IF v_paid_min IS NULL OR v_paid_min < 0 THEN
        v_paid_min := 0;
    END IF;

    v_allowed_min := GREATEST(COALESCE(v_session.booked_duration_minutes, 0), v_paid_min);

    IF v_paid_min > 0 THEN
        v_base_amount := COALESCE(v_session.total_price, 0);
        IF v_base_amount <= 0 THEN
            SELECT pc.base_amount INTO v_base_amount
            FROM "__SP__CalculatePrice"(v_session.console_id, v_paid_min) pc;
        END IF;

        -- Batasi menit yang boleh ditagih (dasar + maksimum overtime).
        v_billable_min := v_duration_min;
        IF v_billable_min > v_allowed_min + v_max_overtime THEN
            v_billable_min := v_allowed_min + v_max_overtime;
            v_capped := TRUE;
        END IF;

        IF v_billable_min > v_allowed_min THEN
            SELECT pc.base_amount INTO v_allowed_amt
            FROM "__SP__CalculatePrice"(v_session.console_id, v_allowed_min) pc;

            SELECT pc.base_amount INTO v_billable_amt
            FROM "__SP__CalculatePrice"(v_session.console_id, v_billable_min) pc;

            v_overtime := GREATEST(v_billable_amt - v_allowed_amt, 0);

            IF v_overtime > 0 THEN
                v_note := 'Kelebihan waktu (otomatis)';
                IF v_capped THEN
                    v_note := v_note || ' — dibatasi ' || v_max_overtime || ' menit';
                END IF;
                INSERT INTO payments (
                    id, session_id, amount, discount_amount, auto_discount_amount,
                    total_payment, payment_method, payment_status,
                    cash_received, change_amount, notes, created_at, updated_at
                ) VALUES (
                    uuid_generate_v4(), p_session_id, v_overtime, 0, 0,
                    v_overtime, 'cash', 'pending',
                    0, 0, v_note, v_now, v_now
                );
            END IF;
        END IF;

        v_total_price := v_base_amount + v_overtime;
    ELSE
        SELECT pc.base_amount INTO v_total_price
        FROM "__SP__CalculatePrice"(v_session.console_id, v_duration_min) pc;
    END IF;

    IF v_total_price < 0 THEN
        v_total_price := 0;
    END IF;

    UPDATE sessions
       SET end_time = v_now,
           duration_minutes = v_duration_min,
           total_price = v_total_price,
           status = 'completed',
           updated_at = v_now
     WHERE sessions.id = p_session_id;

    UPDATE consoles
       SET status = 'available', updated_at = v_now
     WHERE consoles.id = v_session.console_id;

    INSERT INTO tv_activity_logs (id, console_id, event, session_id, is_authorized, duration_minutes, created_at)
    VALUES (uuid_generate_v4(), v_session.console_id, 'off', p_session_id, TRUE, v_duration_min, v_now);

    RETURN QUERY
        SELECT s.id, s.console_id, s.customer_id, s.start_time, s.end_time,
               s.duration_minutes, s.total_price, s.status, s.notes,
               s.created_at, s.updated_at
          FROM sessions s WHERE s.id = p_session_id;
END;
$$;

-- Menutup sesi yang terlupakan tanpa tagihan kelebihan waktu.
CREATE OR REPLACE FUNCTION "__SP__ForceCloseSession"(p_session_id UUID)
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
       SET end_time = v_now,
           duration_minutes = v_min,
           status = 'completed',
           notes = TRIM(COALESCE(notes, '') || ' [auto-close: tidak ada aktivitas]'),
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