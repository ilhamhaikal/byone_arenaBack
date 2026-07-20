-- Fix: byoneCreateDailyRental — capture voucher_id for free_days too
DROP FUNCTION IF EXISTS "byoneCreateDailyRental"(UUID, UUID, DATE, DATE, NUMERIC, VARCHAR, TEXT);

CREATE OR REPLACE FUNCTION "byoneCreateDailyRental"(
    p_console_id    UUID,
    p_customer_id   UUID,
    p_start_date    DATE,
    p_end_date      DATE,
    p_daily_price   NUMERIC,
    p_voucher_code  VARCHAR DEFAULT NULL,
    p_notes         TEXT DEFAULT NULL
)
RETURNS TABLE(
    rental_id UUID, total_days INT, total_amount NUMERIC,
    discount_amount NUMERIC, free_days_used INT, final_amount NUMERIC, status VARCHAR
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_rental_id         UUID;
    v_console_stat      VARCHAR;
    v_total_days        INT;
    v_total_amount      NUMERIC(10,2);
    v_voucher_discount  NUMERIC(10,2) := 0;
    v_free_days         INT := 0;
    v_final_amount      NUMERIC(10,2);
    v_actual_end_date   DATE;
    v_voucher_type      VARCHAR(20);
    v_voucher_id        UUID := NULL;
    v_now               TIMESTAMPTZ := NOW();
BEGIN
    SELECT c.status INTO v_console_stat FROM consoles c WHERE c.id = p_console_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'CONSOLE_NOT_FOUND'; END IF;
    IF v_console_stat != 'available' THEN RAISE EXCEPTION 'CONSOLE_NOT_AVAILABLE'; END IF;
    IF p_end_date <= p_start_date THEN RAISE EXCEPTION 'INVALID_DATE'; END IF;

    v_total_days := p_end_date - p_start_date;
    IF v_total_days < 1 THEN v_total_days := 1; END IF;
    v_total_amount := v_total_days * p_daily_price;

    IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code) != '' THEN
        SELECT discount_type INTO v_voucher_type FROM vouchers WHERE code = UPPER(p_voucher_code);
        
        IF v_voucher_type = 'free_days' THEN
            BEGIN
                -- FIX: juga ambil voucher_id
                SELECT va.voucher_id, va.free_days_used INTO v_voucher_id, v_free_days
                FROM "byoneApplyVoucher"(p_voucher_code, v_total_amount, p_daily_price, v_total_days) va;
                UPDATE vouchers SET usage_count = usage_count + 1, updated_at = v_now
                WHERE code = UPPER(p_voucher_code);
            EXCEPTION WHEN OTHERS THEN v_free_days := 0; END;
            v_voucher_discount := 0;
        ELSE
            BEGIN
                SELECT va.voucher_id, va.discount_amount INTO v_voucher_id, v_voucher_discount
                FROM "byoneApplyVoucher"(p_voucher_code, v_total_amount) va;
                UPDATE vouchers SET usage_count = usage_count + 1, updated_at = v_now
                WHERE code = UPPER(p_voucher_code);
            EXCEPTION WHEN OTHERS THEN v_voucher_discount := 0; END;
            v_free_days := 0;
        END IF;
    END IF;

    IF v_voucher_discount > v_total_amount THEN v_voucher_discount := v_total_amount; END IF;
    v_final_amount := v_total_amount - v_voucher_discount;

    v_rental_id := uuid_generate_v4();
    IF v_free_days > 0 THEN
        v_total_days := v_total_days + v_free_days;
        v_actual_end_date := p_start_date + v_total_days;
    ELSE
        v_actual_end_date := p_end_date;
    END IF;

    INSERT INTO daily_rentals(id, console_id, customer_id, start_date, end_date,
        daily_price, total_days, free_days_used, total_amount,
        discount_amount, final_amount, voucher_id,
        status, notes, created_at, updated_at)
    VALUES(v_rental_id, p_console_id, p_customer_id, p_start_date, v_actual_end_date,
        p_daily_price, v_total_days, v_free_days, v_total_amount,
        v_voucher_discount, v_final_amount, v_voucher_id,
        'active', p_notes, v_now, v_now);

    UPDATE consoles SET status = 'rented_out', updated_at = v_now
    WHERE consoles.id = p_console_id;

    RETURN QUERY SELECT v_rental_id, v_total_days, v_total_amount,
        v_voucher_discount, v_free_days, v_final_amount, 'active'::VARCHAR;
END;
$$;

-- Backfill: set voucher_id for existing free_days rentals
UPDATE daily_rentals d
SET voucher_id = (SELECT id FROM vouchers WHERE code = 'FREE1HARI')
WHERE d.free_days_used = 1 AND d.voucher_id IS NULL;

UPDATE daily_rentals d
SET voucher_id = (SELECT id FROM vouchers WHERE code = 'FREE2HARI')
WHERE d.free_days_used = 2 AND d.voucher_id IS NULL;
