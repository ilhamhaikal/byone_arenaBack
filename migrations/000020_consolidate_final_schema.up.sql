-- Migration: 000020_consolidate_final_schema.up.sql
-- Konsolidasi final: drop old SP overloads + app_settings + nullable fixes
-- Jalanin ini sebelum deploy production

-- =============================================
-- 1. Cleanup old overloaded SPs
-- =============================================
DROP FUNCTION IF EXISTS "byoneSellMembership"(UUID, VARCHAR, NUMERIC, NUMERIC, DATE, DATE);
DROP FUNCTION IF EXISTS "byoneSellMembership"(UUID, VARCHAR, NUMERIC, NUMERIC);
DROP FUNCTION IF EXISTS "byoneCreateDailyRental"(UUID, UUID, DATE, DATE, NUMERIC, NUMERIC, TEXT);

-- =============================================
-- 2. app_settings table (global settings)
-- =============================================
CREATE TABLE IF NOT EXISTS app_settings (
    key         VARCHAR(50) PRIMARY KEY,
    value       TEXT NOT NULL,
    description VARCHAR(255),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO app_settings (key, value, description) VALUES
    ('membership_price', '0', 'Harga membership (lifetime)')
ON CONFLICT (key) DO NOTHING;

-- =============================================
-- 3. Nullable fixes
-- =============================================
ALTER TABLE bookings ALTER COLUMN customer_id DROP NOT NULL;
ALTER TABLE daily_rentals ALTER COLUMN customer_id DROP NOT NULL;
ALTER TABLE payments ALTER COLUMN session_id DROP NOT NULL;

-- =============================================
-- 4. Drop constraints for flexibility
-- =============================================
ALTER TABLE customers DROP CONSTRAINT IF EXISTS customers_membership_type_check;
ALTER TABLE consoles DROP CONSTRAINT IF EXISTS consoles_console_type_check;
ALTER TABLE consoles DROP CONSTRAINT IF EXISTS consoles_status_check;
ALTER TABLE consoles ADD CONSTRAINT consoles_status_check
    CHECK (status IN ('available', 'in_use', 'maintenance', 'rented_out'));

-- =============================================
-- 5. Final SP: byoneSellMembership (simplified)
-- =============================================
CREATE OR REPLACE FUNCTION "byoneSellMembership"(p_customer_id UUID, p_cash_received NUMERIC)
RETURNS TABLE (customer_id UUID, membership_price NUMERIC, payment_id UUID, change_amount NUMERIC, message VARCHAR)
LANGUAGE plpgsql AS $$
DECLARE v_customer customers%ROWTYPE; v_price NUMERIC(10,2); v_payment_id UUID; v_change NUMERIC(10,2); v_now TIMESTAMPTZ := NOW();
BEGIN
    SELECT * INTO v_customer FROM customers WHERE customers.id = p_customer_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'CUSTOMER_NOT_FOUND'; END IF;
    IF v_customer.is_member THEN RAISE EXCEPTION 'ALREADY_MEMBER'; END IF;
    SELECT COALESCE(value::NUMERIC, 0) INTO v_price FROM app_settings WHERE key = 'membership_price';
    IF v_price = 0 THEN RAISE EXCEPTION 'PRICE_NOT_SET'; END IF;
    IF p_cash_received < v_price THEN RAISE EXCEPTION 'INSUFFICIENT_CASH'; END IF;
    v_change := p_cash_received - v_price;
    UPDATE customers SET is_member=TRUE, membership_type='member', membership_start=CURRENT_DATE, membership_expiry=NULL, membership_price=v_price, updated_at=v_now WHERE customers.id=p_customer_id;
    v_payment_id := uuid_generate_v4();
    INSERT INTO payments (id,session_id,amount,total_payment,payment_method,payment_status,cash_received,change_amount,notes,paid_at,created_at,updated_at)
    VALUES (v_payment_id,NULL,v_price,v_price,'cash','paid',p_cash_received,v_change,'Membership lifetime',v_now,v_now,v_now);
    RETURN QUERY SELECT p_customer_id, v_price, v_payment_id, v_change, ('Member aktif. Kembalian: Rp '||v_change::TEXT)::VARCHAR;
END; $$;

-- =============================================
-- 6. Voucher: update discount_type constraint + SP + free_days_used column
-- =============================================
ALTER TABLE vouchers DROP CONSTRAINT IF EXISTS vouchers_discount_type_check;
ALTER TABLE vouchers ADD CONSTRAINT vouchers_discount_type_check
    CHECK (discount_type IN ('percentage', 'fixed_amount', 'free_days'));

ALTER TABLE daily_rentals ADD COLUMN IF NOT EXISTS free_days_used INT NOT NULL DEFAULT 0;
ALTER TABLE daily_rentals ADD COLUMN IF NOT EXISTS discount_amount NUMERIC(10,2) NOT NULL DEFAULT 0;
ALTER TABLE daily_rentals ADD COLUMN IF NOT EXISTS final_amount NUMERIC(10,2) NOT NULL DEFAULT 0;
ALTER TABLE daily_rentals DROP COLUMN IF EXISTS deposit_amount;

-- Drop old overloads
DROP FUNCTION IF EXISTS "byoneApplyVoucher"(VARCHAR, NUMERIC);
DROP FUNCTION IF EXISTS "byoneApplyVoucher"(VARCHAR, NUMERIC, NUMERIC, INT);

CREATE OR REPLACE FUNCTION "byoneApplyVoucher"(
    p_code VARCHAR, p_total_price NUMERIC,
    p_daily_price NUMERIC DEFAULT NULL, p_total_days INT DEFAULT NULL)
RETURNS TABLE(voucher_id UUID, discount_amount NUMERIC, free_days_used INT)
LANGUAGE plpgsql AS $$
DECLARE v_voucher vouchers%ROWTYPE; v_discount NUMERIC(10,2):=0; v_free_days INT:=0;
BEGIN
    SELECT * INTO v_voucher FROM vouchers WHERE code=UPPER(p_code);
    IF NOT FOUND THEN RAISE EXCEPTION 'VOUCHER_NOT_FOUND'; END IF;
    IF NOT v_voucher.is_active THEN RAISE EXCEPTION 'VOUCHER_INACTIVE'; END IF;
    IF v_voucher.expires_at IS NOT NULL AND v_voucher.expires_at<NOW() THEN RAISE EXCEPTION 'VOUCHER_EXPIRED'; END IF;
    IF v_voucher.max_usage>0 AND v_voucher.usage_count>=v_voucher.max_usage THEN RAISE EXCEPTION 'VOUCHER_LIMIT_REACHED'; END IF;
    IF v_voucher.discount_type='free_days' THEN
        IF p_daily_price IS NULL OR p_total_days IS NULL THEN RAISE EXCEPTION 'VOUCHER_INVALID_TARGET'; END IF;
        v_free_days:=LEAST(v_voucher.discount_value::INT, p_total_days);
        v_discount:=v_free_days*p_daily_price;
    ELSIF v_voucher.discount_type='percentage' THEN
        IF p_total_price<v_voucher.min_purchase THEN RAISE EXCEPTION 'VOUCHER_MIN_PURCHASE'; END IF;
        v_discount:=ROUND((p_total_price*v_voucher.discount_value/100)::NUMERIC,2);
        IF v_voucher.max_discount>0 AND v_discount>v_voucher.max_discount THEN v_discount:=v_voucher.max_discount; END IF;
    ELSE
        IF p_total_price<v_voucher.min_purchase THEN RAISE EXCEPTION 'VOUCHER_MIN_PURCHASE'; END IF;
        v_discount:=v_voucher.discount_value;
        IF v_discount>p_total_price THEN v_discount:=p_total_price; END IF;
    END IF;
    RETURN QUERY SELECT v_voucher.id, v_discount, v_free_days;
END; $$;

-- =============================================
-- 7. Final SP: byoneCreateDailyRental (free_days tambah hari, harga tetap)
-- =============================================
DROP FUNCTION IF EXISTS "byoneCreateDailyRental"(UUID, UUID, DATE, DATE, NUMERIC, NUMERIC, VARCHAR, TEXT);
DROP FUNCTION IF EXISTS "byoneCreateDailyRental"(UUID, UUID, DATE, DATE, NUMERIC, NUMERIC, VARCHAR);
DROP FUNCTION IF EXISTS "byoneCreateDailyRental"(UUID, UUID, DATE, DATE, NUMERIC, NUMERIC, TEXT);
DROP FUNCTION IF EXISTS "byoneCreateDailyRental"(UUID, UUID, DATE, DATE, NUMERIC, VARCHAR, TEXT);

CREATE OR REPLACE FUNCTION "byoneCreateDailyRental"(
    p_console_id UUID, p_customer_id UUID, p_start_date DATE, p_end_date DATE,
    p_daily_price NUMERIC,
    p_voucher_code VARCHAR DEFAULT NULL, p_notes TEXT DEFAULT NULL)
RETURNS TABLE(rental_id UUID, total_days INT, total_amount NUMERIC,
    discount_amount NUMERIC, free_days_used INT, final_amount NUMERIC, status VARCHAR)
LANGUAGE plpgsql AS $$
DECLARE v_rental_id UUID; v_console_stat VARCHAR; v_total_days INT; v_total_amount NUMERIC(10,2);
v_voucher_discount NUMERIC(10,2):=0; v_free_days INT:=0; v_final_amount NUMERIC(10,2);
v_actual_end_date DATE; v_voucher_type VARCHAR(20); v_now TIMESTAMPTZ:=NOW();
BEGIN
    SELECT c.status INTO v_console_stat FROM consoles c WHERE c.id=p_console_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'CONSOLE_NOT_FOUND'; END IF;
    IF v_console_stat!='available' THEN RAISE EXCEPTION 'CONSOLE_NOT_AVAILABLE'; END IF;
    IF p_end_date<=p_start_date THEN RAISE EXCEPTION 'INVALID_DATE'; END IF;
    v_total_days:=p_end_date-p_start_date; IF v_total_days<1 THEN v_total_days:=1; END IF;
    v_total_amount:=v_total_days*p_daily_price;
    IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code)!='' THEN
        SELECT discount_type INTO v_voucher_type FROM vouchers WHERE code=UPPER(p_voucher_code);
        IF v_voucher_type='free_days' THEN
            -- Free days: tambah hari, harga TETAP (bukan diskon)
            BEGIN SELECT va.free_days_used INTO v_free_days
                FROM "byoneApplyVoucher"(p_voucher_code,v_total_amount,p_daily_price,v_total_days) va;
                UPDATE vouchers SET usage_count=usage_count+1,updated_at=v_now WHERE code=UPPER(p_voucher_code);
            EXCEPTION WHEN OTHERS THEN v_free_days:=0; END;
            v_voucher_discount:=0;
        ELSE
            -- Diskon biasa: percentage / fixed_amount
            BEGIN SELECT va.discount_amount INTO v_voucher_discount
                FROM "byoneApplyVoucher"(p_voucher_code,v_total_amount) va;
                UPDATE vouchers SET usage_count=usage_count+1,updated_at=v_now WHERE code=UPPER(p_voucher_code);
            EXCEPTION WHEN OTHERS THEN v_voucher_discount:=0; END;
            v_free_days:=0;
        END IF;
    END IF;
    IF v_voucher_discount>v_total_amount THEN v_voucher_discount:=v_total_amount; END IF;
    v_final_amount:=v_total_amount-v_voucher_discount;
    v_rental_id:=uuid_generate_v4();
    IF v_free_days>0 THEN v_total_days:=v_total_days+v_free_days; v_actual_end_date:=p_start_date+v_total_days;
    ELSE v_actual_end_date:=p_end_date; END IF;
    INSERT INTO daily_rentals(id,console_id,customer_id,start_date,end_date,daily_price,total_days,free_days_used,total_amount,discount_amount,final_amount,status,notes,created_at,updated_at)
    VALUES(v_rental_id,p_console_id,p_customer_id,p_start_date,v_actual_end_date,p_daily_price,v_total_days,v_free_days,v_total_amount,v_voucher_discount,v_final_amount,'active',p_notes,v_now,v_now);
    UPDATE consoles SET status='rented_out',updated_at=v_now WHERE consoles.id=p_console_id;
    RETURN QUERY SELECT v_rental_id,v_total_days,v_total_amount,v_voucher_discount,v_free_days,v_final_amount,'active'::VARCHAR;
END; $$;

-- =============================================
-- 7b. Fix: byoneCreatePayment (sp_apply_voucher → byoneApplyVoucher)
-- =============================================
CREATE OR REPLACE FUNCTION "byoneCreatePayment"(
    p_session_id UUID, p_cash_received NUMERIC,
    p_notes TEXT DEFAULT NULL, p_voucher_code VARCHAR DEFAULT NULL)
RETURNS TABLE(payment_id UUID, amount NUMERIC, discount_amount NUMERIC,
    auto_discount_amount NUMERIC, total_payment NUMERIC,
    cash_received NUMERIC, change_amount NUMERIC, voucher_id UUID, paid_at TIMESTAMPTZ)
LANGUAGE plpgsql AS $$
DECLARE v_session sessions%ROWTYPE; v_payment_id UUID; v_amount NUMERIC(10,2);
v_voucher_discount NUMERIC(10,2):=0; v_auto_discount NUMERIC(10,2):=0;
v_total_discount NUMERIC(10,2); v_final_amount NUMERIC(10,2); v_change NUMERIC(10,2);
v_voucher_id UUID:=NULL; v_is_member BOOLEAN:=FALSE; v_now TIMESTAMPTZ:=NOW();
BEGIN
    SELECT * INTO v_session FROM sessions WHERE id=p_session_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'SESSION_NOT_FOUND'; END IF;
    IF v_session.status!='completed' THEN RAISE EXCEPTION 'SESSION_NOT_COMPLETED'; END IF;
    IF EXISTS(SELECT 1 FROM payments WHERE session_id=p_session_id AND payment_status!='refunded') THEN
        RAISE EXCEPTION 'PAYMENT_EXISTS'; END IF;
    v_amount:=v_session.total_price;
    IF v_session.customer_id IS NOT NULL THEN
        SELECT COALESCE(is_member,FALSE) INTO v_is_member FROM customers WHERE id=v_session.customer_id; END IF;
    v_auto_discount:=COALESCE(sp_evaluate_discount_rules(v_amount,v_session.start_time,v_is_member),0);
    IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code)!='' THEN
        BEGIN SELECT va.voucher_id,va.discount_amount INTO v_voucher_id,v_voucher_discount
            FROM "byoneApplyVoucher"(p_voucher_code,v_amount) va;
            UPDATE vouchers SET usage_count=usage_count+1,updated_at=v_now WHERE id=v_voucher_id;
        EXCEPTION WHEN OTHERS THEN v_voucher_discount:=0; END;
    END IF;
    v_total_discount:=v_auto_discount+v_voucher_discount;
    IF v_total_discount>v_amount THEN v_total_discount:=v_amount;
        IF v_auto_discount>v_amount THEN v_auto_discount:=v_amount; v_voucher_discount:=0;
        ELSE v_voucher_discount:=v_amount-v_auto_discount; END IF; END IF;
    v_final_amount:=GREATEST(v_amount-v_total_discount,0);
    IF p_cash_received<v_final_amount THEN RAISE EXCEPTION 'INSUFFICIENT_CASH'; END IF;
    v_change:=p_cash_received-v_final_amount;
    v_payment_id:=uuid_generate_v4();
    INSERT INTO payments(session_id,amount,discount_amount,auto_discount_amount,total_payment,payment_method,payment_status,cash_received,change_amount,voucher_id,notes,paid_at,created_at,updated_at)
    VALUES(p_session_id,v_amount,v_voucher_discount,v_auto_discount,v_final_amount,'cash','paid',p_cash_received,v_change,v_voucher_id,p_notes,v_now,v_now,v_now);
    RETURN QUERY SELECT v_payment_id,v_amount,v_voucher_discount,v_auto_discount,v_final_amount,p_cash_received,v_change,v_voucher_id,v_now;
END; $$;

-- =============================================
-- 8. Final SP: byoneCreateBooking (customerId optional)
-- =============================================
CREATE OR REPLACE FUNCTION "byoneCreateBooking"(
    p_console_id UUID, p_customer_id UUID, p_booking_date DATE,
    p_start_hour INT, p_start_minute INT, p_duration_minutes INT, p_notes TEXT DEFAULT NULL)
RETURNS TABLE (booking_id UUID, status VARCHAR)
LANGUAGE plpgsql AS $$
DECLARE v_id UUID; v_start TIMESTAMPTZ; v_end TIMESTAMPTZ; v_conflict INT; v_ce BOOLEAN;
BEGIN
    IF p_duration_minutes<30 THEN RAISE EXCEPTION 'INVALID_DURATION'; END IF;
    SELECT EXISTS(SELECT 1 FROM consoles WHERE id=p_console_id) INTO v_ce;
    IF NOT v_ce THEN RAISE EXCEPTION 'CONSOLE_NOT_FOUND'; END IF;
    v_start:=p_booking_date+make_time(p_start_hour,p_start_minute,0);
    v_end:=v_start+(p_duration_minutes*INTERVAL'1 minute');
    SELECT COUNT(*) INTO v_conflict FROM (
        SELECT s.console_id FROM sessions s WHERE s.console_id=p_console_id AND s.status='active'
        AND s.start_time<v_end AND COALESCE(s.end_scheduled_at,s.start_time+(s.booked_duration_minutes*INTERVAL'1 minute'))>v_start
        UNION ALL
        SELECT b.console_id FROM bookings b WHERE b.console_id=p_console_id AND b.status IN('pending','confirmed')
        AND b.booking_date=p_booking_date AND make_time(b.start_hour,b.start_minute,0)<v_end::TIME
        AND make_time(b.start_hour,b.start_minute,0)+(b.duration_minutes*INTERVAL'1 minute')>v_start::TIME
    ) conflicts;
    IF v_conflict>0 THEN RAISE EXCEPTION 'BOOKING_CONFLICT'; END IF;
    v_id:=uuid_generate_v4();
    INSERT INTO bookings(id,console_id,customer_id,booking_date,start_hour,start_minute,duration_minutes,status,notes,created_at,updated_at)
    VALUES(v_id,p_console_id,p_customer_id,p_booking_date,p_start_hour,p_start_minute,p_duration_minutes,'pending',p_notes,NOW(),NOW());
    RETURN QUERY SELECT v_id,'pending'::VARCHAR;
END; $$;
