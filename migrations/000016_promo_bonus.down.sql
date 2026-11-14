DROP FUNCTION IF EXISTS "__SP__EndSession"(UUID);

-- Kembalikan EndSession versi 000015 (harga dasar = booked, tanpa promo).
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
    v_base_amount    NUMERIC(10,2) := 0;
    v_booked_amount  NUMERIC(10,2) := 0;
    v_actual_amount  NUMERIC(10,2) := 0;
    v_overtime       NUMERIC(10,2) := 0;
    v_total_price    NUMERIC(10,2) := 0;
BEGIN
    SELECT * INTO v_session FROM sessions WHERE sessions.id = p_session_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'SESSION_NOT_FOUND'; END IF;
    IF v_session.status <> 'active' THEN RAISE EXCEPTION 'SESSION_NOT_ACTIVE: %', v_session.status; END IF;

    v_duration_min := EXTRACT(EPOCH FROM (v_now - v_session.start_time))::INT / 60;
    IF v_duration_min < 1 THEN v_duration_min := 1; END IF;

    IF COALESCE(v_session.booked_duration_minutes, 0) > 0 THEN
        v_base_amount := COALESCE(v_session.total_price, 0);
        IF v_base_amount <= 0 THEN
            SELECT pc.base_amount INTO v_base_amount
            FROM "__SP__CalculatePrice"(v_session.console_id, v_session.booked_duration_minutes) pc;
        END IF;

        IF v_duration_min > v_session.booked_duration_minutes THEN
            SELECT pc.base_amount INTO v_booked_amount
            FROM "__SP__CalculatePrice"(v_session.console_id, v_session.booked_duration_minutes) pc;
            SELECT pc.base_amount INTO v_actual_amount
            FROM "__SP__CalculatePrice"(v_session.console_id, v_duration_min) pc;
            v_overtime := GREATEST(v_actual_amount - v_booked_amount, 0);
            IF v_overtime > 0 THEN
                INSERT INTO payments (id, session_id, amount, discount_amount, auto_discount_amount,
                    total_payment, payment_method, payment_status, cash_received, change_amount, notes, created_at, updated_at)
                VALUES (uuid_generate_v4(), p_session_id, v_overtime, 0, 0, v_overtime, 'cash', 'pending', 0, 0,
                    'Kelebihan waktu (otomatis)', v_now, v_now);
            END IF;
        END IF;
        v_total_price := v_base_amount + v_overtime;
    ELSE
        SELECT pc.base_amount INTO v_total_price
        FROM "__SP__CalculatePrice"(v_session.console_id, v_duration_min) pc;
    END IF;

    IF v_total_price < 0 THEN v_total_price := 0; END IF;

    UPDATE sessions SET end_time=v_now, duration_minutes=v_duration_min, total_price=v_total_price,
           status='completed', updated_at=v_now WHERE sessions.id=p_session_id;
    UPDATE consoles SET status='available', updated_at=v_now WHERE consoles.id=v_session.console_id;
    INSERT INTO tv_activity_logs (id, console_id, event, session_id, is_authorized, duration_minutes, created_at)
    VALUES (uuid_generate_v4(), v_session.console_id, 'off', p_session_id, TRUE, v_duration_min, v_now);

    RETURN QUERY SELECT s.id, s.console_id, s.customer_id, s.start_time, s.end_time,
               s.duration_minutes, s.total_price, s.status, s.notes, s.created_at, s.updated_at
          FROM sessions s WHERE s.id = p_session_id;
END;
$$;

ALTER TABLE sessions DROP COLUMN IF EXISTS promo_applied_at;
ALTER TABLE sessions DROP COLUMN IF EXISTS promo_rule_id;
ALTER TABLE sessions DROP COLUMN IF EXISTS promo_bonus_minutes;
DROP TABLE IF EXISTS promo_bonus_rules;