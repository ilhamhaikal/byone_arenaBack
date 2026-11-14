-- Promo bonus waktu (mis. weekend: main minimal 60 menit gratis 30 menit).
--
-- Aturan disimpan di tabel agar bisa diubah tanpa deploy. Bonus dicatat per
-- sesi (promo_bonus_minutes) sehingga laporan tetap bisa menghitung berapa
-- menit gratis yang diberikan.

CREATE TABLE IF NOT EXISTS promo_bonus_rules (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name             VARCHAR(100) NOT NULL,
    -- Hari dalam format DOW PostgreSQL: 0=Minggu .. 6=Sabtu. Contoh "6,7".
    days_of_week     VARCHAR(20)  NOT NULL DEFAULT '6,7',
    min_paid_minutes INT          NOT NULL DEFAULT 60,
    bonus_minutes    INT          NOT NULL DEFAULT 30,
    valid_from       DATE,
    valid_to         DATE,
    is_active        BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Seed aturan default sekali saja (weekend, min 1 jam, bonus 30 menit).
INSERT INTO promo_bonus_rules (name, days_of_week, min_paid_minutes, bonus_minutes, is_active)
SELECT 'Bonus Weekend 30 Menit', '6,7', 60, 30, TRUE
WHERE NOT EXISTS (SELECT 1 FROM promo_bonus_rules);

ALTER TABLE sessions ADD COLUMN IF NOT EXISTS promo_bonus_minutes INT NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS promo_rule_id UUID REFERENCES promo_bonus_rules(id) ON DELETE SET NULL;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS promo_applied_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_sessions_promo_rule ON sessions (promo_rule_id);

-- EndSession: harga dasar memakai menit yang DIBAYAR, dan kelebihan waktu
-- dihitung setelah menit bonus (bank waktu/poin/promo) habis.
CREATE OR REPLACE FUNCTION "__SP__EndSession"(p_session_id UUID)
RETURNS TABLE (
    id UUID, console_id UUID, customer_id UUID, start_time TIMESTAMPTZ,
    end_time TIMESTAMPTZ, duration_minutes INT, total_price NUMERIC,
    status VARCHAR, notes TEXT, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ
) LANGUAGE plpgsql AS $$
DECLARE
    v_session       sessions%ROWTYPE;
    v_duration_min  INT;
    v_now           TIMESTAMPTZ := NOW();
    v_paid_min      INT;
    v_allowed_min   INT;
    v_base_amount   NUMERIC(10,2) := 0;
    v_allowed_amt   NUMERIC(10,2) := 0;
    v_actual_amt    NUMERIC(10,2) := 0;
    v_overtime      NUMERIC(10,2) := 0;
    v_total_price   NUMERIC(10,2) := 0;
BEGIN
    SELECT * INTO v_session FROM sessions WHERE sessions.id = p_session_id FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'SESSION_NOT_FOUND';
    END IF;
    IF v_session.status <> 'active' THEN
        RAISE EXCEPTION 'SESSION_NOT_ACTIVE: %', v_session.status;
    END IF;

    v_duration_min := EXTRACT(EPOCH FROM (v_now - v_session.start_time))::INT / 60;
    IF v_duration_min < 1 THEN
        v_duration_min := 1;
    END IF;

    -- Menit yang dibayar: paid_duration_minutes bila ada, jika tidak pakai booked.
    v_paid_min := COALESCE(NULLIF(v_session.paid_duration_minutes, 0), v_session.booked_duration_minutes);
    IF v_paid_min IS NULL OR v_paid_min < 0 THEN
        v_paid_min := 0;
    END IF;

    -- Menit yang boleh dimainkan = paid + semua bonus (booked sudah termasuk bonus).
    v_allowed_min := GREATEST(COALESCE(v_session.booked_duration_minutes, 0), v_paid_min);

    IF v_paid_min > 0 THEN
        v_base_amount := COALESCE(v_session.total_price, 0);
        IF v_base_amount <= 0 THEN
            SELECT pc.base_amount INTO v_base_amount
            FROM "__SP__CalculatePrice"(v_session.console_id, v_paid_min) pc;
        END IF;

        IF v_duration_min > v_allowed_min THEN
            SELECT pc.base_amount INTO v_allowed_amt
            FROM "__SP__CalculatePrice"(v_session.console_id, v_allowed_min) pc;

            SELECT pc.base_amount INTO v_actual_amt
            FROM "__SP__CalculatePrice"(v_session.console_id, v_duration_min) pc;

            v_overtime := GREATEST(v_actual_amt - v_allowed_amt, 0);

            IF v_overtime > 0 THEN
                INSERT INTO payments (
                    id, session_id, amount, discount_amount, auto_discount_amount,
                    total_payment, payment_method, payment_status,
                    cash_received, change_amount, notes, created_at, updated_at
                ) VALUES (
                    uuid_generate_v4(), p_session_id, v_overtime, 0, 0,
                    v_overtime, 'cash', 'pending',
                    0, 0, 'Kelebihan waktu (otomatis)', v_now, v_now
                );
            END IF;
        END IF;

        v_total_price := v_base_amount + v_overtime;
    ELSE
        -- Open-ended: tagih sesuai durasi aktual, ikut pricing tiers.
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