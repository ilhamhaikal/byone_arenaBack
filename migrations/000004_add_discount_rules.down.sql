-- Migration: 000004_add_discount_rules.down.sql
-- Rollback sistem diskon otomatis

-- Kembalikan sp_create_payment ke versi sebelumnya (dari 000003)
CREATE OR REPLACE FUNCTION sp_create_payment(
    p_session_id    UUID,
    p_cash_received NUMERIC,
    p_notes         TEXT    DEFAULT NULL,
    p_voucher_code  VARCHAR DEFAULT NULL
)
RETURNS TABLE (
    payment_id      UUID,
    amount          NUMERIC,
    discount_amount NUMERIC,
    cash_received   NUMERIC,
    change_amount   NUMERIC,
    voucher_id      UUID,
    paid_at         TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_session       sessions%ROWTYPE;
    v_payment_id    UUID;
    v_amount        NUMERIC(10,2);
    v_discount      NUMERIC(10,2) := 0;
    v_final_amount  NUMERIC(10,2);
    v_change        NUMERIC(10,2);
    v_voucher_id    UUID := NULL;
    v_now           TIMESTAMPTZ := NOW();
BEGIN
    SELECT * INTO v_session FROM sessions WHERE id = p_session_id;
    IF NOT FOUND THEN RAISE EXCEPTION 'SESSION_NOT_FOUND: Sesi tidak ditemukan'; END IF;
    IF v_session.status != 'completed' THEN RAISE EXCEPTION 'SESSION_NOT_COMPLETED: Sesi belum selesai'; END IF;
    IF EXISTS (SELECT 1 FROM payments WHERE session_id = p_session_id AND payment_status != 'refunded') THEN
        RAISE EXCEPTION 'PAYMENT_EXISTS: Pembayaran untuk sesi ini sudah ada';
    END IF;

    v_amount := v_session.total_price;

    IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code) != '' THEN
        SELECT va.voucher_id, va.discount_amount INTO v_voucher_id, v_discount
        FROM sp_apply_voucher(p_voucher_code, v_amount) va;
        UPDATE vouchers SET usage_count = usage_count + 1, updated_at = v_now WHERE id = v_voucher_id;
    END IF;

    v_final_amount := v_amount - v_discount;
    IF v_final_amount < 0 THEN v_final_amount := 0; END IF;

    IF p_cash_received < v_final_amount THEN
        RAISE EXCEPTION 'INSUFFICIENT_CASH: Uang yang diterima kurang. Dibutuhkan Rp %.0f, diterima Rp %.0f',
            v_final_amount, p_cash_received;
    END IF;

    v_change := p_cash_received - v_final_amount;

    INSERT INTO payments (session_id, amount, discount_amount, payment_method, payment_status,
                          cash_received, change_amount, voucher_id, notes, paid_at, created_at, updated_at)
    VALUES (p_session_id, v_amount, v_discount, 'cash', 'paid',
            p_cash_received, v_change, v_voucher_id, p_notes, v_now, v_now, v_now)
    RETURNING id INTO v_payment_id;

    RETURN QUERY SELECT v_payment_id, v_amount, v_discount, p_cash_received, v_change, v_voucher_id, v_now;
END;
$$;

DROP FUNCTION IF EXISTS sp_evaluate_discount_rules(NUMERIC, TIMESTAMPTZ, BOOLEAN);

ALTER TABLE payments DROP COLUMN IF EXISTS auto_discount_amount;

DROP TABLE IF EXISTS discount_rules;

DROP INDEX IF EXISTS idx_customers_is_member;
ALTER TABLE customers DROP COLUMN IF EXISTS is_member;
