-- =============================================================================
-- 000054_allow_free_membership.up.sql
--
-- Membership boleh dijual dengan harga 0 (gratis). Sebelumnya SP
-- SellMembership menolak harga 0 dengan PRICE_NOT_SET, sehingga pembuatan
-- customer member / penjualan membership gagal saat harga diset 0.
--
-- Perubahan hanya pada validasi harga; data customer/payment lama tidak
-- disentuh.
-- =============================================================================

DROP FUNCTION IF EXISTS "__SP__SellMembership"(UUID, NUMERIC);

CREATE OR REPLACE FUNCTION "__SP__SellMembership"(
    p_customer_id UUID,
    p_cash_received NUMERIC
)
RETURNS TABLE(
    customer_id UUID,
    membership_price NUMERIC,
    payment_id UUID,
    change_amount NUMERIC,
    message VARCHAR
)
LANGUAGE plpgsql AS $$
DECLARE
    v_customer   customers%ROWTYPE;
    v_price      NUMERIC(10,2);
    v_payment_id UUID;
    v_change     NUMERIC(10,2);
    v_now        TIMESTAMPTZ := NOW();
BEGIN
    SELECT * INTO v_customer
      FROM customers
     WHERE customers.id = p_customer_id
       FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'CUSTOMER_NOT_FOUND';
    END IF;

    IF v_customer.is_member THEN
        RAISE EXCEPTION 'ALREADY_MEMBER';
    END IF;

    SELECT COALESCE(value::NUMERIC, 0)
      INTO v_price
      FROM app_settings
     WHERE key = 'membership_price';

    -- Harga 0 = membership gratis dan tetap valid.
    v_price := COALESCE(v_price, 0);

    IF p_cash_received IS NULL THEN
        p_cash_received := v_price;
    END IF;

    IF p_cash_received < 0 THEN
        RAISE EXCEPTION 'INVALID_CASH';
    END IF;

    IF p_cash_received < v_price THEN
        RAISE EXCEPTION 'INSUFFICIENT_CASH';
    END IF;

    v_change := p_cash_received - v_price;

    UPDATE customers
       SET is_member = TRUE,
           membership_type = 'member',
           membership_start = CURRENT_DATE,
           membership_expiry = NULL,
           membership_price = v_price,
           updated_at = v_now
     WHERE customers.id = p_customer_id;

    v_payment_id := uuid_generate_v4();

    INSERT INTO payments(
        id, session_id, amount, total_payment, payment_method,
        payment_status, cash_received, change_amount, notes,
        paid_at, created_at, updated_at
    ) VALUES (
        v_payment_id, NULL, v_price, v_price, 'cash',
        'paid', p_cash_received, v_change, 'Membership lifetime',
        v_now, v_now, v_now
    );

    RETURN QUERY SELECT
        p_customer_id,
        v_price,
        v_payment_id,
        v_change,
        ('Member aktif. Kembalian: Rp ' || v_change::TEXT)::VARCHAR;
END;
$$;