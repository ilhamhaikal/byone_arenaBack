-- =============================================================================
-- 000005_overrides.up.sql — Semua perbaikan & fitur baru SP
-- Di-apply setelah 000002, meng-override StartSessionWithPayment,
-- EndSession, dan ExtendSession dengan versi final:
--   ✅ Bank Waktu & Poin Loyalitas
--   ✅ Zero paid duration (gratis dari bank waktu/poin)
--   ✅ ExtendSession pakai paid_duration (bukan booked_duration)
--   ✅ Voucher unlimited_play (main sepuasnya, member-only)
--   ✅ EndSession skip bank waktu untuk sesi unlimited
-- =============================================================================

-- =============================================================================
-- 1. StartSessionWithPayment (FINAL — bank waktu + unlimited_play + member-only)
-- =============================================================================
DROP FUNCTION IF EXISTS "__SP__StartSessionWithPayment"(UUID,UUID,TEXT,INT,NUMERIC,VARCHAR);
DROP FUNCTION IF EXISTS "__SP__StartSessionWithPayment"(UUID,UUID,TEXT,INT,NUMERIC,VARCHAR,INT,INT);

CREATE OR REPLACE FUNCTION "__SP__StartSessionWithPayment"(
    p_console_id UUID, p_customer_id UUID, p_notes TEXT,
    p_booked_duration_minutes INTEGER, p_cash_received NUMERIC,
    p_voucher_code VARCHAR DEFAULT NULL,
    p_use_time_bank_minutes INTEGER DEFAULT 0,
    p_redeem_point_units INTEGER DEFAULT 0
)
RETURNS TABLE(
    session_id UUID, session_status VARCHAR, session_start_time TIMESTAMPTZ,
    session_booked_minutes INT, session_end_scheduled TIMESTAMPTZ,
    payment_id UUID, base_amount NUMERIC, discount_amount NUMERIC,
    auto_discount_amount NUMERIC, total_payment NUMERIC,
    cash_received NUMERIC, change_amount NUMERIC, voucher_id UUID,
    paid_at TIMESTAMPTZ,
    paid_duration_minutes INT, time_bank_minutes_used INT,
    bonus_minutes_from_points INT, points_redeemed INT,
    points_earned INT,
    customer_time_balance_after INT, customer_points_after INT
)
LANGUAGE plpgsql AS $$
DECLARE
    v_session_id UUID;
    v_payment_id UUID;
    v_console_status VARCHAR;
    v_price_per_hour NUMERIC(10,2);
    v_end_scheduled TIMESTAMPTZ;
    v_amount NUMERIC(10,2):=0;
    v_voucher_discount NUMERIC(10,2):=0;
    v_auto_discount NUMERIC(10,2):=0;
    v_total_discount NUMERIC(10,2):=0;
    v_final_amount NUMERIC(10,2):=0;
    v_change NUMERIC(10,2):=0;
    v_voucher_id UUID:=NULL;
    v_is_member BOOLEAN:=FALSE;
    v_now TIMESTAMPTZ:=NOW();
    v_customer customers%ROWTYPE;
    v_threshold INT;
    v_redeem_minutes INT;
    v_points_needed INT:=0;
    v_bonus_from_points INT:=0;
    v_points_earned INT:=0;
    v_time_balance_after INT:=0;
    v_points_after INT:=0;
    v_total_minutes INT;
    v_is_unlimited BOOLEAN:=FALSE;
    v_unlimited_max INT:=1440;
BEGIN
    IF p_use_time_bank_minutes<0 THEN p_use_time_bank_minutes:=0; END IF;
    IF p_redeem_point_units<0 THEN p_redeem_point_units:=0; END IF;
    IF p_booked_duration_minutes<0 THEN p_booked_duration_minutes:=0; END IF;

    -- Deteksi voucher unlimited_play
    IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code)!='' THEN
        SELECT v.id, v.discount_value INTO v_voucher_id, v_voucher_discount
        FROM vouchers v
        WHERE v.code=UPPER(p_voucher_code)
          AND v.is_active=TRUE
          AND v.discount_type='unlimited_play'
          AND (v.expires_at IS NULL OR v.expires_at>v_now)
          AND (v.max_usage=0 OR v.usage_count<v.max_usage);
        IF FOUND THEN
            v_is_unlimited:=TRUE;
            SELECT COALESCE(value::INT,1440) INTO v_unlimited_max FROM app_settings WHERE key='unlimited_max_minutes';
        END IF;
    END IF;

    IF v_is_unlimited THEN
        p_booked_duration_minutes:=v_unlimited_max;
    END IF;

    IF p_booked_duration_minutes=0 AND p_use_time_bank_minutes=0 AND p_redeem_point_units=0 THEN
        RAISE EXCEPTION 'INVALID_DURATION: minimal 1 menit (bayar atau bonus)';
    END IF;

    SELECT c.status,c.price_per_hour INTO v_console_status,v_price_per_hour FROM consoles c WHERE c.id=p_console_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'CONSOLE_NOT_FOUND'; END IF;
    IF v_console_status!='available' THEN RAISE EXCEPTION 'CONSOLE_NOT_AVAILABLE: %',v_console_status; END IF;
    IF EXISTS(SELECT 1 FROM sessions s2 WHERE s2.console_id=p_console_id AND s2.status='active') THEN RAISE EXCEPTION 'SESSION_ALREADY_ACTIVE'; END IF;

    IF p_customer_id IS NOT NULL THEN
        SELECT * INTO v_customer FROM customers WHERE id=p_customer_id FOR UPDATE;
        IF FOUND THEN v_is_member:=COALESCE(v_customer.is_member,FALSE); END IF;
    END IF;

    -- Voucher main sepuasnya HANYA untuk member
    IF v_is_unlimited AND NOT v_is_member THEN
        RAISE EXCEPTION 'NOT_A_MEMBER: voucher main sepuasnya hanya untuk member';
    END IF;

    IF p_use_time_bank_minutes>0 THEN
        IF NOT v_is_member THEN RAISE EXCEPTION 'NOT_A_MEMBER: bank waktu hanya untuk member'; END IF;
        IF p_use_time_bank_minutes>v_customer.time_balance_minutes THEN
            RAISE EXCEPTION 'INSUFFICIENT_TIME_BANK: saldo bank waktu hanya % menit',v_customer.time_balance_minutes;
        END IF;
    END IF;

    IF p_redeem_point_units>0 THEN
        IF NOT v_is_member THEN RAISE EXCEPTION 'NOT_A_MEMBER: tukar poin hanya untuk member'; END IF;
        SELECT COALESCE(value::INT,15) INTO v_threshold FROM app_settings WHERE key='loyalty_point_threshold';
        SELECT COALESCE(value::INT,60) INTO v_redeem_minutes FROM app_settings WHERE key='loyalty_point_redeem_minutes';
        v_points_needed:=p_redeem_point_units*v_threshold;
        IF v_points_needed>v_customer.loyalty_points THEN
            RAISE EXCEPTION 'INSUFFICIENT_POINTS: butuh % poin, saldo hanya % poin',v_points_needed,v_customer.loyalty_points;
        END IF;
        v_bonus_from_points:=p_redeem_point_units*v_redeem_minutes;
    END IF;

    -- Harga: unlimited = flat dari voucher, normal = CalculatePrice
    IF v_is_unlimited THEN
        v_amount:=v_voucher_discount;
        v_voucher_discount:=0;
    ELSIF p_booked_duration_minutes>0 THEN
        SELECT pc.base_amount INTO v_amount FROM "__SP__CalculatePrice"(p_console_id,p_booked_duration_minutes) pc;
    END IF;

    v_total_minutes:=p_booked_duration_minutes+p_use_time_bank_minutes+v_bonus_from_points;
    IF v_total_minutes<1 THEN v_total_minutes:=1; END IF;
    v_end_scheduled:=v_now+(v_total_minutes*INTERVAL'1 minute');

    IF NOT v_is_unlimited AND v_amount>0 THEN
        v_auto_discount:="__SP__EvaluateDiscountRules"(v_amount,v_now,v_is_member);
        IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code)!='' AND v_voucher_id IS NULL THEN
            BEGIN
                SELECT va.voucher_id,va.discount_amount INTO v_voucher_id,v_voucher_discount FROM "__SP__ApplyVoucher"(p_voucher_code,v_amount) va;
                UPDATE vouchers SET usage_count=usage_count+1,updated_at=v_now WHERE id=v_voucher_id;
            EXCEPTION WHEN OTHERS THEN
                v_voucher_discount:=0; v_voucher_id:=NULL;
            END;
        END IF;
        v_total_discount:=v_auto_discount+v_voucher_discount;
        IF v_total_discount>v_amount THEN
            v_total_discount:=v_amount;
            IF v_auto_discount>v_amount THEN v_auto_discount:=v_amount; v_voucher_discount:=0;
            ELSE v_voucher_discount:=v_amount-v_auto_discount;
            END IF;
        END IF;
        v_final_amount:=GREATEST(v_amount-v_total_discount,0);
    ELSIF v_is_unlimited THEN
        v_final_amount:=v_amount;
        IF v_voucher_id IS NOT NULL THEN
            UPDATE vouchers SET usage_count=usage_count+1,updated_at=v_now WHERE id=v_voucher_id;
        END IF;
    END IF;

    IF v_final_amount>0 AND p_cash_received<v_final_amount THEN
        RAISE EXCEPTION 'INSUFFICIENT_CASH: butuh Rp %.0f',v_final_amount;
    END IF;
    v_change:=GREATEST(p_cash_received-v_final_amount,0);

    v_session_id:=uuid_generate_v4();
    INSERT INTO sessions(id,console_id,customer_id,start_time,booked_duration_minutes,end_scheduled_at,total_price,status,notes,paid_duration_minutes,time_bank_minutes_used,bonus_minutes_from_points,points_redeemed,points_earned,created_at,updated_at)
    VALUES(v_session_id,p_console_id,p_customer_id,v_now,v_total_minutes,v_end_scheduled,v_amount,'active',p_notes,p_booked_duration_minutes,p_use_time_bank_minutes,v_bonus_from_points,v_points_needed,0,v_now,v_now);
    UPDATE consoles SET status='in_use',updated_at=v_now WHERE consoles.id=p_console_id;
    INSERT INTO tv_activity_logs(id,console_id,event,session_id,is_authorized,created_at) VALUES(uuid_generate_v4(),p_console_id,'on',v_session_id,TRUE,v_now);

    v_payment_id:=uuid_generate_v4();
    INSERT INTO payments(id,session_id,amount,discount_amount,auto_discount_amount,total_payment,payment_method,payment_status,cash_received,change_amount,voucher_id,notes,paid_at,created_at,updated_at)
    VALUES(v_payment_id,v_session_id,v_amount,
           CASE WHEN v_is_unlimited THEN 0 ELSE v_voucher_discount END,
           v_auto_discount,v_final_amount,'cash','paid',p_cash_received,v_change,v_voucher_id,p_notes,v_now,v_now,v_now);

    v_time_balance_after:=COALESCE(v_customer.time_balance_minutes,0);
    v_points_after:=COALESCE(v_customer.loyalty_points,0);

    IF v_is_member THEN
        IF p_use_time_bank_minutes>0 THEN
            v_time_balance_after:=v_time_balance_after-p_use_time_bank_minutes;
            INSERT INTO customer_time_bank_logs(customer_id,session_id,change_type,minutes,balance_after,created_at)
            VALUES(p_customer_id,v_session_id,'use',p_use_time_bank_minutes,v_time_balance_after,v_now);
        END IF;
        IF v_points_needed>0 THEN
            v_points_after:=v_points_after-v_points_needed;
            INSERT INTO customer_point_logs(customer_id,session_id,change_type,points,balance_after,created_at)
            VALUES(p_customer_id,v_session_id,'redeem',v_points_needed,v_points_after,v_now);
        END IF;
        v_points_earned:=FLOOR(p_booked_duration_minutes/60.0)::INT;
        IF v_points_earned>0 THEN
            v_points_after:=v_points_after+v_points_earned;
            INSERT INTO customer_point_logs(customer_id,session_id,change_type,points,balance_after,created_at)
            VALUES(p_customer_id,v_session_id,'earn',v_points_earned,v_points_after,v_now);
        END IF;
        UPDATE customers SET time_balance_minutes=v_time_balance_after,loyalty_points=v_points_after,updated_at=v_now WHERE id=p_customer_id;
        UPDATE sessions SET points_earned=v_points_earned WHERE id=v_session_id;
    END IF;

    RETURN QUERY SELECT v_session_id,'active'::VARCHAR,v_now,v_total_minutes,v_end_scheduled,
        v_payment_id,v_amount,CASE WHEN v_is_unlimited THEN 0 ELSE v_voucher_discount END,
        v_auto_discount,v_final_amount,
        p_cash_received,v_change,v_voucher_id,v_now,
        p_booked_duration_minutes,p_use_time_bank_minutes,v_bonus_from_points,
        v_points_needed,v_points_earned,v_time_balance_after,v_points_after;
END;
$$;

-- =============================================================================
-- 2. EndSession (FINAL — bank waktu + unlimited skip)
-- =============================================================================
CREATE OR REPLACE FUNCTION "__SP__EndSession"(p_session_id UUID)
RETURNS TABLE (
    id UUID, console_id UUID, customer_id UUID,
    start_time TIMESTAMPTZ, end_time TIMESTAMPTZ,
    duration_minutes INT, total_price NUMERIC,
    status VARCHAR, notes TEXT,
    created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ
)
LANGUAGE plpgsql AS $$
DECLARE
    v_session sessions%ROWTYPE;
    v_price_per_hour NUMERIC;
    v_duration_min INT;
    v_total_price NUMERIC;
    v_now TIMESTAMPTZ:=NOW();
    v_leftover_minutes INT;
    v_is_member BOOLEAN:=FALSE;
    v_new_balance INT;
    v_is_unlimited BOOLEAN:=FALSE;
BEGIN
    SELECT * INTO v_session FROM sessions WHERE sessions.id=p_session_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'SESSION_NOT_FOUND'; END IF;
    IF v_session.status!='active' THEN RAISE EXCEPTION 'SESSION_NOT_ACTIVE: %', v_session.status; END IF;

    SELECT price_per_hour INTO v_price_per_hour FROM consoles WHERE consoles.id=v_session.console_id;

    v_duration_min:=EXTRACT(EPOCH FROM (v_now-v_session.start_time))::INT/60;
    IF v_session.booked_duration_minutes>0 AND v_duration_min>v_session.booked_duration_minutes THEN
        v_duration_min:=v_session.booked_duration_minutes;
    END IF;

    IF v_session.booked_duration_minutes>0 AND COALESCE(v_session.total_price,0)>0 THEN
        v_total_price:=v_session.total_price;
    ELSIF v_session.booked_duration_minutes>0 THEN
        SELECT pc.base_amount INTO v_total_price FROM "__SP__CalculatePrice"(v_session.console_id,v_session.booked_duration_minutes) pc;
    ELSE
        v_total_price:=ROUND((v_duration_min::NUMERIC/60.0)*v_price_per_hour,2);
    END IF;

    IF v_duration_min<1 THEN v_duration_min:=1; END IF;
    IF v_total_price<0 THEN v_total_price:=0; END IF;

    UPDATE sessions SET end_time=v_now,duration_minutes=v_duration_min,total_price=v_total_price,status='completed',updated_at=v_now
    WHERE sessions.id=p_session_id;

    UPDATE consoles SET status='available',updated_at=v_now WHERE consoles.id=v_session.console_id;

    INSERT INTO tv_activity_logs(id,console_id,event,session_id,is_authorized,duration_minutes,created_at)
    VALUES(uuid_generate_v4(),v_session.console_id,'off',p_session_id,TRUE,v_duration_min,v_now);

    -- Cek apakah sesi ini pakai voucher unlimited_play
    SELECT EXISTS(
        SELECT 1 FROM payments p
        JOIN vouchers v ON v.id=p.voucher_id
        WHERE p.session_id=p_session_id AND v.discount_type='unlimited_play'
    ) INTO v_is_unlimited;

    -- Kredit sisa waktu ke bank waktu member
    -- KECUALI untuk sesi unlimited_play (harga flat, tidak ada "sisa waktu berbayar")
    IF NOT v_is_unlimited AND v_session.customer_id IS NOT NULL THEN
        SELECT COALESCE(customers.is_member,FALSE) INTO v_is_member FROM customers WHERE customers.id=v_session.customer_id;
        IF v_is_member THEN
            v_leftover_minutes:=v_session.booked_duration_minutes-v_duration_min;
            IF v_leftover_minutes>0 THEN
                UPDATE customers SET time_balance_minutes=customers.time_balance_minutes+v_leftover_minutes,updated_at=v_now
                WHERE customers.id=v_session.customer_id
                RETURNING customers.time_balance_minutes INTO v_new_balance;
                INSERT INTO customer_time_bank_logs(customer_id,session_id,change_type,minutes,balance_after,created_at)
                VALUES(v_session.customer_id,p_session_id,'earn',v_leftover_minutes,v_new_balance,v_now);
            END IF;
        END IF;
    END IF;

    RETURN QUERY SELECT sessions.id,sessions.console_id,sessions.customer_id,
        sessions.start_time,sessions.end_time,sessions.duration_minutes,
        sessions.total_price,sessions.status,sessions.notes,
        sessions.created_at,sessions.updated_at
    FROM sessions WHERE sessions.id=p_session_id;
END;
$$;

-- =============================================================================
-- 3. ExtendSession (FINAL — harga dari paid_duration, bukan booked_duration)
-- =============================================================================
DROP FUNCTION IF EXISTS "__SP__ExtendSession"(UUID,INTEGER,NUMERIC,BOOLEAN,VARCHAR,TEXT);
DROP FUNCTION IF EXISTS "__SP__ExtendSession"(UUID,INTEGER,NUMERIC,VARCHAR,TEXT);

CREATE OR REPLACE FUNCTION "__SP__ExtendSession"(
    p_session_id UUID,
    p_additional_minutes INTEGER,
    p_cash_received NUMERIC,
    p_pay_now BOOLEAN DEFAULT TRUE,
    p_voucher_code VARCHAR DEFAULT NULL,
    p_notes TEXT DEFAULT NULL
)
RETURNS TABLE(
    session_id UUID,
    session_booked_minutes INT,
    session_end_scheduled TIMESTAMPTZ,
    payment_id UUID,
    payment_amount NUMERIC,
    payment_discount NUMERIC,
    payment_total NUMERIC,
    payment_cash_received NUMERIC,
    payment_change NUMERIC,
    payment_voucher_id UUID,
    payment_status VARCHAR,
    payment_paid_at TIMESTAMPTZ
)
LANGUAGE plpgsql AS $$
DECLARE
    v_session sessions%ROWTYPE;
    v_payment_id UUID;
    v_amount NUMERIC(10,2):=0;
    v_voucher_discount NUMERIC(10,2):=0;
    v_voucher_id UUID:=NULL;
    v_final_amount NUMERIC(10,2):=0;
    v_change NUMERIC(10,2):=0;
    v_new_booked INT;
    v_new_paid INT;
    v_old_paid INT;
    v_new_end TIMESTAMPTZ;
    v_now TIMESTAMPTZ:=NOW();
    v_status VARCHAR;
    v_paid_at TIMESTAMPTZ;
BEGIN
    SELECT * INTO v_session FROM sessions WHERE sessions.id=p_session_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'SESSION_NOT_FOUND'; END IF;
    IF v_session.status!='active' THEN RAISE EXCEPTION 'SESSION_NOT_ACTIVE'; END IF;
    IF p_additional_minutes<=0 THEN RAISE EXCEPTION 'INVALID_DURATION: minimal 1 menit'; END IF;

    -- Kunci: hitung dari PAID duration, bukan BOOKED (yg sudah termasuk bank waktu & poin)
    v_old_paid:=COALESCE(v_session.paid_duration_minutes,0);
    v_new_paid:=v_old_paid+p_additional_minutes;
    v_new_booked:=v_session.booked_duration_minutes+p_additional_minutes;

    -- Harga = selisih CalculatePrice(paid_baru) - CalculatePrice(paid_lama)
    DECLARE
        v_old_paid_price NUMERIC(10,2):=0;
        v_new_paid_price NUMERIC(10,2);
    BEGIN
        SELECT pc.base_amount INTO v_new_paid_price FROM "__SP__CalculatePrice"(v_session.console_id, v_new_paid) pc;
        IF v_old_paid>0 THEN
            SELECT pc2.base_amount INTO v_old_paid_price FROM "__SP__CalculatePrice"(v_session.console_id, v_old_paid) pc2;
        END IF;
        v_amount:=GREATEST(v_new_paid_price-v_old_paid_price,0);
    END;

    IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code)!='' AND v_amount>0 THEN
        BEGIN
            SELECT va.voucher_id,va.discount_amount INTO v_voucher_id,v_voucher_discount FROM "__SP__ApplyVoucher"(p_voucher_code,v_amount) va;
        EXCEPTION WHEN OTHERS THEN
            v_voucher_discount:=0; v_voucher_id:=NULL;
        END;
        IF v_voucher_id IS NOT NULL THEN
            UPDATE vouchers SET usage_count=usage_count+1,updated_at=v_now WHERE id=v_voucher_id;
        END IF;
    END IF;

    IF v_voucher_discount>v_amount THEN v_voucher_discount:=v_amount; END IF;
    v_final_amount:=GREATEST(v_amount-v_voucher_discount,0);

    IF p_pay_now THEN
        IF p_cash_received<v_final_amount THEN RAISE EXCEPTION 'INSUFFICIENT_CASH: butuh Rp %.0f',v_final_amount; END IF;
        v_change:=p_cash_received-v_final_amount;
        v_status:='paid';
        v_paid_at:=v_now;
    ELSE
        v_change:=0;
        v_status:='pending';
        v_paid_at:=NULL;
    END IF;

    v_new_end:=COALESCE(v_session.end_scheduled_at,v_now)+(p_additional_minutes*INTERVAL'1 minute');

    UPDATE sessions SET booked_duration_minutes=v_new_booked, paid_duration_minutes=v_new_paid, end_scheduled_at=v_new_end, updated_at=v_now
    WHERE sessions.id=p_session_id;

    v_payment_id:=uuid_generate_v4();
    INSERT INTO payments(id,session_id,amount,discount_amount,auto_discount_amount,total_payment,payment_method,payment_status,cash_received,change_amount,voucher_id,notes,paid_at,created_at,updated_at)
    VALUES(v_payment_id,p_session_id,v_amount,0,v_voucher_discount,v_final_amount,'cash',v_status,
           CASE WHEN p_pay_now THEN p_cash_received ELSE 0 END, v_change,v_voucher_id,p_notes,v_paid_at,v_now,v_now);

    RETURN QUERY SELECT p_session_id,v_new_booked,v_new_end,
        v_payment_id,v_amount,v_voucher_discount,v_final_amount,
        CASE WHEN p_pay_now THEN p_cash_received ELSE 0 END, v_change,v_voucher_id,v_status,v_paid_at;
END;
$$;

-- +migrate Down
-- (up-only)

