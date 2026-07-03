-- Migration: 000008_add_total_payment.up.sql
-- Tambah kolom total_payment untuk menyimpan jumlah final yang dibayar customer
-- total_payment = amount - discount_amount - auto_discount_amount
-- Digunakan untuk keperluan laporan tanpa harus menghitung ulang

-- =============================================
-- 1. Tambah kolom ke tabel payments
-- =============================================
ALTER TABLE payments ADD COLUMN IF NOT EXISTS total_payment NUMERIC(10,2) NOT NULL DEFAULT 0;

-- Isi data yang sudah ada (backfill)
UPDATE payments
SET total_payment = amount - COALESCE(discount_amount, 0) - COALESCE(auto_discount_amount, 0)
WHERE total_payment = 0;

-- =============================================
-- 2. Update sp_create_payment — tambah total_payment ke RETURN dan INSERT
-- =============================================
DROP FUNCTION IF EXISTS sp_create_payment(UUID, NUMERIC, TEXT, VARCHAR);

CREATE OR REPLACE FUNCTION sp_create_payment(
    p_session_id    UUID,
    p_cash_received NUMERIC,
    p_notes         TEXT    DEFAULT NULL,
    p_voucher_code  VARCHAR DEFAULT NULL
)
RETURNS TABLE (
    payment_id           UUID,
    amount               NUMERIC,
    discount_amount      NUMERIC,
    auto_discount_amount NUMERIC,
    total_payment        NUMERIC,
    cash_received        NUMERIC,
    change_amount        NUMERIC,
    voucher_id           UUID,
    paid_at              TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_session          sessions%ROWTYPE;
    v_payment_id       UUID;
    v_amount           NUMERIC(10,2);
    v_voucher_discount NUMERIC(10,2) := 0;
    v_auto_discount    NUMERIC(10,2) := 0;
    v_total_discount   NUMERIC(10,2);
    v_final_amount     NUMERIC(10,2);
    v_change           NUMERIC(10,2);
    v_voucher_id       UUID := NULL;
    v_is_member        BOOLEAN := FALSE;
    v_now              TIMESTAMPTZ := NOW();
BEGIN
    SELECT * INTO v_session FROM sessions WHERE id = p_session_id FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'SESSION_NOT_FOUND: Sesi tidak ditemukan';
    END IF;

    IF v_session.status != 'completed' THEN
        RAISE EXCEPTION 'SESSION_NOT_COMPLETED: Sesi belum selesai, tidak bisa dibayar';
    END IF;

    IF EXISTS (SELECT 1 FROM payments WHERE session_id = p_session_id AND payment_status != 'refunded') THEN
        RAISE EXCEPTION 'PAYMENT_EXISTS: Pembayaran untuk sesi ini sudah ada';
    END IF;

    v_amount := v_session.total_price;

    IF v_session.customer_id IS NOT NULL THEN
        SELECT COALESCE(is_member, FALSE)
        INTO v_is_member
        FROM customers
        WHERE id = v_session.customer_id;
    END IF;

    v_auto_discount := sp_evaluate_discount_rules(v_amount, v_session.start_time, v_is_member);

    IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code) != '' THEN
        SELECT va.voucher_id, va.discount_amount
        INTO v_voucher_id, v_voucher_discount
        FROM sp_apply_voucher(p_voucher_code, v_amount) va;

        UPDATE vouchers
        SET usage_count = usage_count + 1, updated_at = v_now
        WHERE id = v_voucher_id;
    END IF;

    v_total_discount := v_auto_discount + v_voucher_discount;
    IF v_total_discount > v_amount THEN
        v_total_discount := v_amount;
        IF v_auto_discount > v_amount THEN
            v_auto_discount    := v_amount;
            v_voucher_discount := 0;
        ELSE
            v_voucher_discount := v_amount - v_auto_discount;
        END IF;
    END IF;

    v_final_amount := GREATEST(v_amount - v_total_discount, 0);

    IF p_cash_received < v_final_amount THEN
        RAISE EXCEPTION 'INSUFFICIENT_CASH: Uang yang diterima kurang. Dibutuhkan Rp %.0f, diterima Rp %.0f',
            v_final_amount, p_cash_received;
    END IF;

    v_change := p_cash_received - v_final_amount;

    INSERT INTO payments (
        session_id, amount, discount_amount, auto_discount_amount,
        total_payment, payment_method, payment_status,
        cash_received, change_amount, voucher_id, notes,
        paid_at, created_at, updated_at
    )
    VALUES (
        p_session_id, v_amount, v_voucher_discount, v_auto_discount,
        v_final_amount, 'cash', 'paid',
        p_cash_received, v_change, v_voucher_id, p_notes,
        v_now, v_now, v_now
    )
    RETURNING id INTO v_payment_id;

    RETURN QUERY SELECT
        v_payment_id, v_amount, v_voucher_discount, v_auto_discount,
        v_final_amount,
        p_cash_received, v_change, v_voucher_id, v_now;
END;
$$;

-- =============================================
-- 3. Update sp_start_session_with_payment — tambah total_payment
-- =============================================
DROP FUNCTION IF EXISTS sp_start_session_with_payment(UUID, UUID, TEXT, INTEGER, NUMERIC, VARCHAR);

CREATE OR REPLACE FUNCTION sp_start_session_with_payment(
    p_console_id              UUID,
    p_customer_id             UUID,
    p_notes                   TEXT,
    p_booked_duration_minutes INTEGER,
    p_cash_received           NUMERIC,
    p_voucher_code            VARCHAR DEFAULT NULL
)
RETURNS TABLE (
    session_id              UUID,
    session_status          VARCHAR,
    session_start_time      TIMESTAMPTZ,
    session_booked_minutes  INT,
    session_end_scheduled   TIMESTAMPTZ,
    payment_id              UUID,
    base_amount             NUMERIC,
    discount_amount         NUMERIC,
    auto_discount_amount    NUMERIC,
    total_payment           NUMERIC,
    cash_received           NUMERIC,
    change_amount           NUMERIC,
    voucher_id              UUID,
    paid_at                 TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_session_id       UUID;
    v_payment_id       UUID;
    v_console_status   VARCHAR;
    v_price_per_hour   NUMERIC(10,2);
    v_end_scheduled    TIMESTAMPTZ;
    v_amount           NUMERIC(10,2);
    v_voucher_discount NUMERIC(10,2) := 0;
    v_auto_discount    NUMERIC(10,2) := 0;
    v_total_discount   NUMERIC(10,2);
    v_final_amount     NUMERIC(10,2);
    v_change           NUMERIC(10,2);
    v_voucher_id       UUID := NULL;
    v_is_member        BOOLEAN := FALSE;
    v_now              TIMESTAMPTZ := NOW();
BEGIN
    IF p_booked_duration_minutes < 60 THEN
        RAISE EXCEPTION 'INVALID_DURATION: Durasi minimal 60 menit (1 jam)';
    END IF;

    SELECT c.status, c.price_per_hour
    INTO v_console_status, v_price_per_hour
    FROM consoles c WHERE c.id = p_console_id FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'CONSOLE_NOT_FOUND: Konsol tidak ditemukan';
    END IF;

    IF v_console_status != 'available' THEN
        RAISE EXCEPTION 'CONSOLE_NOT_AVAILABLE: Konsol tidak tersedia (status: %)', v_console_status;
    END IF;

    IF EXISTS (
        SELECT 1 FROM sessions s2
        WHERE s2.console_id = p_console_id AND s2.status = 'active'
    ) THEN
        RAISE EXCEPTION 'SESSION_ALREADY_ACTIVE: Konsol masih memiliki sesi aktif';
    END IF;

    v_amount        := ROUND((p_booked_duration_minutes::NUMERIC / 60.0) * v_price_per_hour, 2);
    v_end_scheduled := v_now + (p_booked_duration_minutes * INTERVAL '1 minute');

    IF p_customer_id IS NOT NULL THEN
        SELECT COALESCE(is_member, FALSE)
        INTO v_is_member
        FROM customers WHERE id = p_customer_id;
    END IF;

    v_auto_discount := sp_evaluate_discount_rules(v_amount, v_now, v_is_member);

    IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code) != '' THEN
        SELECT va.voucher_id, va.discount_amount
        INTO v_voucher_id, v_voucher_discount
        FROM sp_apply_voucher(p_voucher_code, v_amount) va;

        UPDATE vouchers
        SET usage_count = usage_count + 1, updated_at = v_now
        WHERE id = v_voucher_id;
    END IF;

    v_total_discount := v_auto_discount + v_voucher_discount;
    IF v_total_discount > v_amount THEN
        v_total_discount := v_amount;
        IF v_auto_discount > v_amount THEN
            v_auto_discount    := v_amount;
            v_voucher_discount := 0;
        ELSE
            v_voucher_discount := v_amount - v_auto_discount;
        END IF;
    END IF;

    v_final_amount := GREATEST(v_amount - v_total_discount, 0);

    IF p_cash_received < v_final_amount THEN
        RAISE EXCEPTION 'INSUFFICIENT_CASH: Uang yang diterima kurang. Dibutuhkan Rp %.0f, diterima Rp %.0f',
            v_final_amount, p_cash_received;
    END IF;

    v_change := p_cash_received - v_final_amount;

    v_session_id := uuid_generate_v4();
    INSERT INTO sessions (
        id, console_id, customer_id,
        start_time, booked_duration_minutes, end_scheduled_at,
        total_price, status, notes, created_at, updated_at
    )
    VALUES (
        v_session_id, p_console_id, p_customer_id,
        v_now, p_booked_duration_minutes, v_end_scheduled,
        v_amount, 'active', p_notes, v_now, v_now
    );

    UPDATE consoles SET status = 'in_use', updated_at = v_now WHERE id = p_console_id;

    v_payment_id := uuid_generate_v4();
    INSERT INTO payments (
        id, session_id, amount, discount_amount, auto_discount_amount,
        total_payment, payment_method, payment_status,
        cash_received, change_amount, voucher_id, notes,
        paid_at, created_at, updated_at
    )
    VALUES (
        v_payment_id, v_session_id,
        v_amount, v_voucher_discount, v_auto_discount,
        v_final_amount, 'cash', 'paid',
        p_cash_received, v_change,
        v_voucher_id, p_notes,
        v_now, v_now, v_now
    );

    RETURN QUERY SELECT
        v_session_id,
        'active'::VARCHAR,
        v_now,
        p_booked_duration_minutes,
        v_end_scheduled,
        v_payment_id,
        v_amount,
        v_voucher_discount,
        v_auto_discount,
        v_final_amount,
        p_cash_received,
        v_change,
        v_voucher_id,
        v_now;
END;
$$;
