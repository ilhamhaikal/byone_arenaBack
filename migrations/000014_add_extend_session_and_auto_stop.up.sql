-- Migration: 000014_add_extend_session_and_auto_stop.up.sql
-- 1. Allow multiple payments per session (remove unique constraint)
-- 2. Add byoneExtendSession SP for extending session time
-- 3. Session auto-stop handled by backend goroutine

-- =============================================
-- 1. Hapus unique constraint session_id di payments
--    agar satu sesi bisa punya banyak payment (initial + extend)
-- =============================================
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_session_id_key;
DROP INDEX IF EXISTS idx_payments_session_id;
CREATE INDEX IF NOT EXISTS idx_payments_session_id ON payments(session_id);

-- =============================================
-- 2. SP: byoneExtendSession — tambah waktu sewa
--    Membuat payment PENDING, admin harus konfirmasi nanti
-- =============================================
CREATE OR REPLACE FUNCTION "byoneExtendSession"(
    p_session_id              UUID,
    p_additional_minutes      INTEGER,
    p_cash_received           NUMERIC,
    p_voucher_code            VARCHAR DEFAULT NULL,
    p_notes                   TEXT DEFAULT NULL
)
RETURNS TABLE (
    -- updated session
    session_id                  UUID,
    session_booked_minutes      INT,
    session_end_scheduled       TIMESTAMPTZ,
    -- new pending payment
    payment_id                  UUID,
    payment_amount              NUMERIC,
    payment_discount            NUMERIC,
    payment_total               NUMERIC,
    payment_cash_received       NUMERIC,
    payment_change              NUMERIC,
    payment_voucher_id          UUID,
    payment_status              VARCHAR,
    payment_paid_at             TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_session           sessions%ROWTYPE;
    v_payment_id        UUID;
    v_price_per_hour    NUMERIC(10,2);
    v_amount            NUMERIC(10,2);
    v_voucher_discount  NUMERIC(10,2) := 0;
    v_voucher_id        UUID := NULL;
    v_final_amount      NUMERIC(10,2);
    v_change            NUMERIC(10,2);
    v_new_booked        INT;
    v_new_end           TIMESTAMPTZ;
    v_now               TIMESTAMPTZ := NOW();
BEGIN
    -- Validasi sesi
    SELECT * INTO v_session FROM sessions WHERE sessions.id = p_session_id FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'SESSION_NOT_FOUND: Sesi tidak ditemukan';
    END IF;
    IF v_session.status != 'active' THEN
        RAISE EXCEPTION 'SESSION_NOT_ACTIVE: Sesi sudah tidak aktif';
    END IF;

    -- Validasi additional minutes
    IF p_additional_minutes < 30 THEN
        RAISE EXCEPTION 'INVALID_DURATION: Minimal tambah waktu 30 menit';
    END IF;

    -- Ambil harga per jam
    SELECT price_per_hour INTO v_price_per_hour FROM consoles WHERE consoles.id = v_session.console_id;

    -- Hitung harga tambahan
    v_amount := ROUND((p_additional_minutes::NUMERIC / 60.0) * v_price_per_hour, 2);

    -- Voucher opsional
    IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code) != '' THEN
        BEGIN
            SELECT va.voucher_id, va.discount_amount
            INTO v_voucher_id, v_voucher_discount
            FROM "byoneApplyVoucher"(p_voucher_code, v_amount) va;
        EXCEPTION WHEN OTHERS THEN
            -- Voucher invalid, ignore
            v_voucher_discount := 0;
            v_voucher_id := NULL;
        END;
        IF v_voucher_id IS NOT NULL THEN
            UPDATE vouchers SET usage_count = usage_count + 1, updated_at = v_now WHERE id = v_voucher_id;
        END IF;
    END IF;

    IF v_voucher_discount > v_amount THEN v_voucher_discount := v_amount; END IF;
    v_final_amount := v_amount - v_voucher_discount;
    IF v_final_amount < 0 THEN v_final_amount := 0; END IF;

    v_change := GREATEST(p_cash_received - v_final_amount, 0);

    -- Hitung waktu baru
    v_new_booked := v_session.booked_duration_minutes + p_additional_minutes;
    v_new_end := v_now + (v_new_booked * INTERVAL '1 minute');

    -- Update sesi
    UPDATE sessions
    SET booked_duration_minutes = v_new_booked,
        end_scheduled_at       = v_new_end,
        updated_at             = v_now
    WHERE sessions.id = p_session_id;

    -- Buat payment PENDING (admin konfirmasi nanti)
    v_payment_id := uuid_generate_v4();
    INSERT INTO payments (
        id, session_id, amount, discount_amount, auto_discount_amount,
        total_payment, payment_method, payment_status,
        cash_received, change_amount, voucher_id, notes,
        created_at, updated_at
    ) VALUES (
        v_payment_id, p_session_id,
        v_amount, v_voucher_discount, 0,
        v_final_amount, 'cash', 'pending',
        p_cash_received, v_change, v_voucher_id, p_notes,
        v_now, v_now
    );

    RETURN QUERY SELECT
        p_session_id, v_new_booked, v_new_end,
        v_payment_id, v_amount, v_voucher_discount, v_final_amount,
        p_cash_received, v_change, v_voucher_id,
        'pending'::VARCHAR, NULL::TIMESTAMPTZ;
END;
$$;

-- =============================================
-- 3. SP: byoneConfirmExtendPayment — admin konfirmasi pembayaran tambahan
-- =============================================
CREATE OR REPLACE FUNCTION "byoneConfirmExtendPayment"(p_payment_id UUID)
RETURNS TABLE (
    payment_id          UUID,
    payment_status      VARCHAR,
    paid_at             TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_payment payments%ROWTYPE;
    v_now     TIMESTAMPTZ := NOW();
BEGIN
    SELECT * INTO v_payment FROM payments WHERE payments.id = p_payment_id FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'PAYMENT_NOT_FOUND: Pembayaran tidak ditemukan';
    END IF;
    IF v_payment.payment_status != 'pending' THEN
        RAISE EXCEPTION 'PAYMENT_NOT_PENDING: Hanya pembayaran pending yang bisa dikonfirmasi';
    END IF;

    UPDATE payments
    SET payment_status = 'paid', paid_at = v_now, updated_at = v_now
    WHERE payments.id = p_payment_id;

    RETURN QUERY SELECT p_payment_id, 'paid'::VARCHAR, v_now;
END;
$$;
