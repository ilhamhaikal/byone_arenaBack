-- Migration: 000025_add_paynow_to_extend.up.sql
-- Tambah opsi payNow: true = langsung paid, false = pending

CREATE OR REPLACE FUNCTION "byoneExtendSession"(
    p_session_id UUID, p_additional_minutes INTEGER, p_cash_received NUMERIC,
    p_pay_now BOOLEAN DEFAULT TRUE,   -- NEW: true = paid, false = pending
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
    v_status VARCHAR; v_paid_at TIMESTAMPTZ;
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

    -- Tentukan status: payNow=true → paid, payNow=false → pending
    IF p_pay_now THEN v_status:='paid'; v_paid_at:=v_now;
    ELSE v_status:='pending'; v_paid_at:=NULL; END IF;

    v_payment_id:=uuid_generate_v4();
    INSERT INTO payments(id,session_id,amount,discount_amount,auto_discount_amount,total_payment,payment_method,payment_status,cash_received,change_amount,voucher_id,notes,paid_at,created_at,updated_at)
    VALUES(v_payment_id,p_session_id,v_amount,v_voucher_discount,0,v_final_amount,'cash',v_status,p_cash_received,v_change,v_voucher_id,p_notes,v_paid_at,v_now,v_now);
    RETURN QUERY SELECT p_session_id,v_new_booked,v_new_end,v_payment_id,v_amount,v_voucher_discount,v_final_amount,p_cash_received,v_change,v_voucher_id,v_status,v_paid_at;
END; $$;
