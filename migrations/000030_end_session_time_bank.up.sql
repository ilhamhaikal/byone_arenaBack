-- =============================================================================
-- 000030_end_session_time_bank.up.sql
--
-- BUGFIX: fitur "simpan waktu" (Bank Waktu) hilang karena EndSession di-override
-- oleh migrasi 000015/000016/000019 tanpa logika kredit bank waktu.
--
-- Perbaikan (member-only):
--   - Sisa menit BERBAYAR yang tidak terpakai disimpan ke time_balance_minutes.
--   - Saldo bank waktu yang dipakai lalu tidak terpakai dikembalikan.
--   - Hanya untuk customer is_member = TRUE.
--   - Tidak mengkredit sesi gratis (voucher 100%), unlimited_play,
--     bonus poin/promo, atau extend yang masih pending.
--   - Mengembalikan time_bank_earned_minutes agar UI bisa menampilkan info.
-- =============================================================================

-- Pastikan setting poin default ada (1 unit = 15 poin -> 60 menit).
INSERT INTO app_settings (key, value) VALUES ('loyalty_point_threshold', '15')
ON CONFLICT (key) DO NOTHING;
INSERT INTO app_settings (key, value) VALUES ('loyalty_point_redeem_minutes', '60')
ON CONFLICT (key) DO NOTHING;

-- Return type berubah (tambah kolom), jadi harus DROP dulu.
DROP FUNCTION IF EXISTS "__SP__EndSession"(UUID);

CREATE OR REPLACE FUNCTION "__SP__EndSession"(p_session_id UUID)
RETURNS TABLE (
    id UUID, console_id UUID, customer_id UUID, start_time TIMESTAMPTZ,
    end_time TIMESTAMPTZ, duration_minutes INT, total_price NUMERIC,
    status VARCHAR, notes TEXT, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ,
    time_bank_earned_minutes INT
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
    -- Bank waktu
    v_is_member      BOOLEAN := FALSE;
    v_is_unlimited   BOOLEAN := FALSE;
    v_paid_amount    NUMERIC(10,2) := 0;
    v_has_pending    BOOLEAN := FALSE;
    v_free_min       INT := 0;
    v_credit_base    INT := 0;
    v_free_consumed  INT := 0;
    v_paid_leftover  INT := 0;
    v_bank_refund    INT := 0;
    v_credit         INT := 0;
    v_new_balance    INT := 0;
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

    -- ── Bank Waktu (member-only) ────────────────────────────────────────────
    IF v_session.customer_id IS NOT NULL THEN
        SELECT COALESCE(is_member, FALSE) INTO v_is_member
          FROM customers WHERE customers.id = v_session.customer_id;

        IF v_is_member THEN
            -- Sesi "main sepuasnya" (unlimited_play) tidak menghasilkan bank waktu.
            SELECT EXISTS (
                SELECT 1 FROM payments p
                JOIN vouchers vc ON vc.id = p.voucher_id
                WHERE p.session_id = p_session_id AND vc.discount_type = 'unlimited_play'
            ) INTO v_is_unlimited;

            IF NOT v_is_unlimited THEN
                -- Total uang yang BENAR-BENAR masuk (paid), dan cek tagihan pending.
                SELECT COALESCE(SUM(total_payment), 0) INTO v_paid_amount
                  FROM payments WHERE payments.session_id = p_session_id AND payment_status = 'paid';
                SELECT EXISTS (
                    SELECT 1 FROM payments WHERE payments.session_id = p_session_id AND payment_status = 'pending'
                ) INTO v_has_pending;

                -- Menit gratis (tidak boleh dikonversi menjadi saldo).
                v_free_min := COALESCE(v_session.time_bank_minutes_used, 0)
                            + COALESCE(v_session.bonus_minutes_from_points, 0)
                            + COALESCE(v_session.promo_bonus_minutes, 0);

                v_credit_base := COALESCE(v_session.paid_duration_minutes, 0);
                IF v_credit_base <= 0 THEN
                    v_credit_base := GREATEST(COALESCE(v_session.booked_duration_minutes, 0) - v_free_min, 0);
                END IF;

                -- Sisa menit berbayar yang tidak terpakai (hanya bila ada uang masuk
                -- dan tidak ada tagihan pending).
                IF v_paid_amount > 0 AND NOT v_has_pending THEN
                    v_paid_leftover := GREATEST(v_credit_base - LEAST(v_duration_min, v_credit_base), 0);
                END IF;

                -- Kembalikan saldo bank waktu yang dipakai lalu tidak terpakai
                -- (asumsi: menit berbayar dipakai lebih dulu, lalu bank waktu).
                v_free_consumed := GREATEST(v_duration_min - v_credit_base, 0);
                v_bank_refund := GREATEST(COALESCE(v_session.time_bank_minutes_used, 0) - v_free_consumed, 0);

                v_credit := v_paid_leftover + v_bank_refund;

                IF v_credit > 0 THEN
                    UPDATE customers
                       SET time_balance_minutes = COALESCE(time_balance_minutes, 0) + v_credit,
                           updated_at = v_now
                     WHERE customers.id = v_session.customer_id
                     RETURNING customers.time_balance_minutes INTO v_new_balance;

                    INSERT INTO customer_time_bank_logs
                        (customer_id, session_id, change_type, minutes, balance_after, created_at)
                    VALUES
                        (v_session.customer_id, p_session_id, 'earn', v_credit, v_new_balance, v_now);
                END IF;
            END IF;
        END IF;
    END IF;

    RETURN QUERY
        SELECT s.id, s.console_id, s.customer_id, s.start_time, s.end_time,
               s.duration_minutes, s.total_price, s.status, s.notes,
               s.created_at, s.updated_at, v_credit
          FROM sessions s WHERE s.id = p_session_id;
END;
$$;
