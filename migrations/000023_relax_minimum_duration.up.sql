-- Migration: 000023_relax_minimum_duration.up.sql
-- Ubah minimal durasi dari 30 menit → 1 menit (untuk debugging TV unit)

-- 1. byoneStartSessionWithPayment: < 30 → <= 0
CREATE OR REPLACE FUNCTION "byoneStartSessionWithPayment"(
    p_console_id              UUID,
    p_customer_id             UUID,
    p_notes                   TEXT,
    p_booked_duration_minutes INTEGER,
    p_cash_received           NUMERIC,
    p_voucher_code            VARCHAR DEFAULT NULL
)
RETURNS TABLE (
    session_id              UUID, session_status VARCHAR, session_start_time TIMESTAMPTZ,
    session_booked_minutes  INT, session_end_scheduled TIMESTAMPTZ,
    payment_id              UUID, base_amount NUMERIC,
    discount_amount         NUMERIC, auto_discount_amount NUMERIC,
    total_payment           NUMERIC, cash_received NUMERIC,
    change_amount           NUMERIC, voucher_id UUID, paid_at TIMESTAMPTZ
)
LANGUAGE plpgsql AS $$
DECLARE
    v_session_id UUID; v_payment_id UUID; v_console_status VARCHAR; v_price_per_hour NUMERIC(10,2);
    v_end_scheduled TIMESTAMPTZ; v_amount NUMERIC(10,2);
    v_voucher_discount NUMERIC(10,2):=0; v_auto_discount NUMERIC(10,2):=0;
    v_total_discount NUMERIC(10,2); v_final_amount NUMERIC(10,2); v_change NUMERIC(10,2);
    v_voucher_id UUID:=NULL; v_is_member BOOLEAN:=FALSE; v_now TIMESTAMPTZ:=NOW();
BEGIN
    IF p_booked_duration_minutes <= 0 THEN
        RAISE EXCEPTION 'INVALID_DURATION: Durasi minimal 1 menit';
    END IF;
    SELECT c.status, c.price_per_hour INTO v_console_status, v_price_per_hour
    FROM consoles c WHERE c.id=p_console_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'CONSOLE_NOT_FOUND'; END IF;
    IF v_console_status!='available' THEN RAISE EXCEPTION 'CONSOLE_NOT_AVAILABLE'; END IF;
    IF EXISTS(SELECT 1 FROM sessions s2 WHERE s2.console_id=p_console_id AND s2.status='active') THEN
        RAISE EXCEPTION 'SESSION_ALREADY_ACTIVE'; END IF;
    SELECT pc.base_amount INTO v_amount FROM "byoneCalculatePrice"(p_console_id, p_booked_duration_minutes) pc;
    v_end_scheduled:=v_now+(p_booked_duration_minutes*INTERVAL'1 minute');
    IF p_customer_id IS NOT NULL THEN
        SELECT COALESCE(is_member,FALSE) INTO v_is_member FROM customers WHERE id=p_customer_id; END IF;
    v_auto_discount:="byoneEvaluateDiscountRules"(v_amount,v_now,v_is_member);
    IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code)!='' THEN
        SELECT va.voucher_id,va.discount_amount INTO v_voucher_id,v_voucher_discount
        FROM "byoneApplyVoucher"(p_voucher_code,v_amount) va;
        UPDATE vouchers SET usage_count=usage_count+1,updated_at=v_now WHERE id=v_voucher_id; END IF;
    v_total_discount:=v_auto_discount+v_voucher_discount;
    IF v_total_discount>v_amount THEN v_total_discount:=v_amount;
        IF v_auto_discount>v_amount THEN v_auto_discount:=v_amount; v_voucher_discount:=0;
        ELSE v_voucher_discount:=v_amount-v_auto_discount; END IF; END IF;
    v_final_amount:=GREATEST(v_amount-v_total_discount,0);
    IF p_cash_received<v_final_amount THEN RAISE EXCEPTION 'INSUFFICIENT_CASH'; END IF;
    v_change:=p_cash_received-v_final_amount;
    v_session_id:=uuid_generate_v4();
    INSERT INTO sessions(id,console_id,customer_id,start_time,booked_duration_minutes,end_scheduled_at,total_price,status,notes,created_at,updated_at)
    VALUES(v_session_id,p_console_id,p_customer_id,v_now,p_booked_duration_minutes,v_end_scheduled,v_amount,'active',p_notes,v_now,v_now);
    UPDATE consoles SET status='in_use',updated_at=v_now WHERE consoles.id=p_console_id;
    v_payment_id:=uuid_generate_v4();
    INSERT INTO payments(id,session_id,amount,discount_amount,auto_discount_amount,total_payment,payment_method,payment_status,cash_received,change_amount,voucher_id,notes,paid_at,created_at,updated_at)
    VALUES(v_payment_id,v_session_id,v_amount,v_voucher_discount,v_auto_discount,v_final_amount,'cash','paid',p_cash_received,v_change,v_voucher_id,p_notes,v_now,v_now,v_now);
    RETURN QUERY SELECT v_session_id,'active'::VARCHAR,v_now,p_booked_duration_minutes,v_end_scheduled,
        v_payment_id,v_amount,v_voucher_discount,v_auto_discount,v_final_amount,p_cash_received,v_change,v_voucher_id,v_now;
END; $$;

-- 2. byonePreviewPrice: < 30 → <= 0
CREATE OR REPLACE FUNCTION "byonePreviewPrice"(
    p_console_id UUID, p_duration_minutes INTEGER,
    p_voucher_code VARCHAR DEFAULT NULL, p_customer_id UUID DEFAULT NULL)
RETURNS TABLE(price_per_hour NUMERIC, duration_minutes INT, base_amount NUMERIC,
    auto_discount NUMERIC, voucher_discount NUMERIC, total_discount NUMERIC,
    final_amount NUMERIC, voucher_applied BOOLEAN, voucher_name VARCHAR,
    price_breakdown JSONB, tier_count INT, message VARCHAR)
LANGUAGE plpgsql AS $$
DECLARE
    v_price_per_hour NUMERIC(10,2); v_status VARCHAR; v_amount NUMERIC(10,2);
    v_breakdown JSONB; v_tier_count INT; v_auto_discount NUMERIC(10,2):=0;
    v_voucher_discount NUMERIC(10,2):=0; v_voucher_id UUID; v_voucher_name VARCHAR;
    v_is_member BOOLEAN:=FALSE; v_total_disc NUMERIC(10,2); v_final NUMERIC(10,2);
    v_voucher_ok BOOLEAN:=FALSE; v_msg VARCHAR:='';
BEGIN
    IF p_duration_minutes <= 0 THEN RAISE EXCEPTION 'INVALID_DURATION: Durasi minimal 1 menit'; END IF;
    SELECT c.price_per_hour,c.status INTO v_price_per_hour,v_status FROM consoles c WHERE c.id=p_console_id;
    IF NOT FOUND THEN RAISE EXCEPTION 'CONSOLE_NOT_FOUND'; END IF;
    IF v_status!='available' THEN v_msg:='Konsol sedang tidak tersedia'; END IF;
    SELECT pc.base_amount,pc.breakdown,pc.tier_count INTO v_amount,v_breakdown,v_tier_count FROM "byoneCalculatePrice"(p_console_id,p_duration_minutes) pc;
    IF p_customer_id IS NOT NULL THEN SELECT COALESCE(is_member,FALSE) INTO v_is_member FROM customers WHERE id=p_customer_id; END IF;
    v_auto_discount:="byoneEvaluateDiscountRules"(v_amount,NOW(),v_is_member);
    IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code)!='' THEN
        BEGIN SELECT va.voucher_id,va.discount_amount INTO v_voucher_id,v_voucher_discount FROM "byoneApplyVoucher"(p_voucher_code,v_amount) va;
            v_voucher_ok:=TRUE; SELECT name INTO v_voucher_name FROM vouchers WHERE id=v_voucher_id;
        EXCEPTION WHEN OTHERS THEN v_voucher_discount:=0; v_voucher_ok:=FALSE; v_msg:=v_msg||' Voucher tidak valid.'; END; END IF;
    v_total_disc:=v_auto_discount+v_voucher_discount;
    IF v_total_disc>v_amount THEN v_total_disc:=v_amount; END IF;
    v_final:=GREATEST(v_amount-v_total_disc,0);
    IF v_msg='' AND v_status='available' THEN v_msg:='Konsol tersedia. Total: Rp '||v_final::TEXT||' untuk '||p_duration_minutes::TEXT||' menit'; END IF;
    RETURN QUERY SELECT v_price_per_hour,p_duration_minutes,v_amount,v_auto_discount,v_voucher_discount,v_total_disc,v_final,v_voucher_ok,v_voucher_name,v_breakdown,v_tier_count,v_msg;
END; $$;

-- 3. byoneExtendSession: < 30 → <= 0
CREATE OR REPLACE FUNCTION "byoneExtendSession"(
    p_session_id UUID, p_additional_minutes INTEGER, p_cash_received NUMERIC,
    p_voucher_code VARCHAR DEFAULT NULL, p_notes TEXT DEFAULT NULL)
RETURNS TABLE(session_id UUID, session_booked_minutes INT, session_end_scheduled TIMESTAMPTZ,
    payment_id UUID, payment_amount NUMERIC, payment_discount NUMERIC, payment_total NUMERIC,
    payment_cash_received NUMERIC, payment_change NUMERIC, payment_voucher_id UUID,
    payment_status VARCHAR, payment_paid_at TIMESTAMPTZ)
LANGUAGE plpgsql AS $$
DECLARE
    v_session sessions%ROWTYPE; v_payment_id UUID; v_amount NUMERIC(10,2);
    v_voucher_discount NUMERIC(10,2):=0; v_voucher_id UUID:=NULL;
    v_final_amount NUMERIC(10,2); v_change NUMERIC(10,2);
    v_new_booked INT; v_new_end TIMESTAMPTZ; v_now TIMESTAMPTZ:=NOW();
BEGIN
    SELECT * INTO v_session FROM sessions WHERE sessions.id=p_session_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'SESSION_NOT_FOUND'; END IF;
    IF v_session.status!='active' THEN RAISE EXCEPTION 'SESSION_NOT_ACTIVE'; END IF;
    IF p_additional_minutes <= 0 THEN RAISE EXCEPTION 'INVALID_DURATION: Minimal tambah 1 menit'; END IF;
    DECLARE v_new_total NUMERIC(10,2); v_old_total NUMERIC(10,2):=v_session.total_price;
        v_total_minutes INT:=v_session.booked_duration_minutes+p_additional_minutes;
    BEGIN
        SELECT pc.base_amount INTO v_new_total FROM "byoneCalculatePrice"(v_session.console_id,v_total_minutes) pc;
        v_amount:=v_new_total-v_old_total; IF v_amount<0 THEN v_amount:=0; END IF; END;
    IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code)!='' THEN
        BEGIN SELECT va.voucher_id,va.discount_amount INTO v_voucher_id,v_voucher_discount FROM "byoneApplyVoucher"(p_voucher_code,v_amount) va;
        EXCEPTION WHEN OTHERS THEN v_voucher_discount:=0; v_voucher_id:=NULL; END;
        IF v_voucher_id IS NOT NULL THEN UPDATE vouchers SET usage_count=usage_count+1,updated_at=v_now WHERE id=v_voucher_id; END IF; END IF;
    IF v_voucher_discount>v_amount THEN v_voucher_discount:=v_amount; END IF;
    v_final_amount:=v_amount-v_voucher_discount; IF v_final_amount<0 THEN v_final_amount:=0; END IF;
    v_change:=GREATEST(p_cash_received-v_final_amount,0);
    v_new_booked:=v_session.booked_duration_minutes+p_additional_minutes;
    v_new_end:=v_now+(v_new_booked*INTERVAL'1 minute');
    UPDATE sessions SET booked_duration_minutes=v_new_booked,end_scheduled_at=v_new_end,total_price=v_session.total_price+v_amount,updated_at=v_now WHERE sessions.id=p_session_id;
    v_payment_id:=uuid_generate_v4();
    INSERT INTO payments(id,session_id,amount,discount_amount,auto_discount_amount,total_payment,payment_method,payment_status,cash_received,change_amount,voucher_id,notes,created_at,updated_at)
    VALUES(v_payment_id,p_session_id,v_amount,v_voucher_discount,0,v_final_amount,'cash','pending',p_cash_received,v_change,v_voucher_id,p_notes,v_now,v_now);
    RETURN QUERY SELECT p_session_id,v_new_booked,v_new_end,v_payment_id,v_amount,v_voucher_discount,v_final_amount,p_cash_received,v_change,v_voucher_id,'pending'::VARCHAR,NULL::TIMESTAMPTZ;
END; $$;

-- 4. byoneCreateBooking: tetap min 30 menit (tidak diubah)
--    Booking tidak termasuk — user hanya minta sesi yg diubah

-- 5. Booking constraint tetap >= 30 (tidak diubah)
