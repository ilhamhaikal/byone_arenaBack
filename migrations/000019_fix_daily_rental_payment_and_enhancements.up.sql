-- Migration: 000019_fix_daily_rental_payment_and_enhancements.up.sql
-- 1. Fix: make payments.session_id nullable (daily rental tidak punya sesi)
-- 2. Add consoles.daily_price — harga sewa harian
-- 3. Add consoles.last_seen_at — heartbeat tracking
-- 4. Fix byoneCreateDailyRental — hapus payment INSERT
-- 5. Add overdue auto-set

-- =============================================
-- 1. Fix payments.session_id → nullable
-- =============================================
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_session_id_fkey;
ALTER TABLE payments ALTER COLUMN session_id DROP NOT NULL;
ALTER TABLE payments ADD CONSTRAINT payments_session_id_fkey
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE SET NULL;
DROP INDEX IF EXISTS payments_session_id_key;

-- =============================================
-- 2. Tambah consoles.daily_price
-- =============================================
ALTER TABLE consoles ADD COLUMN IF NOT EXISTS daily_price NUMERIC(10,2) NOT NULL DEFAULT 0;

-- =============================================
-- 3. Tambah consoles.last_seen_at
-- =============================================
ALTER TABLE consoles ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ;

-- =============================================
-- 4. Fix byoneCreateDailyRental — jangan insert payment
-- =============================================
DROP FUNCTION IF EXISTS "byoneCreateDailyRental"(UUID, UUID, DATE, DATE, NUMERIC, NUMERIC, TEXT);
CREATE OR REPLACE FUNCTION "byoneCreateDailyRental"(
    p_console_id    UUID,
    p_customer_id   UUID,
    p_start_date    DATE,
    p_end_date      DATE,
    p_daily_price   NUMERIC,
    p_deposit       NUMERIC DEFAULT 0,
    p_notes         TEXT DEFAULT NULL
)
RETURNS TABLE (
    rental_id       UUID,
    total_days      INT,
    total_amount    NUMERIC,
    status          VARCHAR
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_rental_id     UUID;
    v_console_stat  VARCHAR;
    v_total_days    INT;
    v_total_amount  NUMERIC(10,2);
    v_now           TIMESTAMPTZ := NOW();
BEGIN
    SELECT c.status INTO v_console_stat FROM consoles c WHERE c.id = p_console_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'CONSOLE_NOT_FOUND: Konsol tidak ditemukan'; END IF;
    IF v_console_stat != 'available' THEN
        RAISE EXCEPTION 'CONSOLE_NOT_AVAILABLE: Konsol tidak tersedia (status: %)', v_console_stat;
    END IF;
    IF p_end_date < p_start_date THEN
        RAISE EXCEPTION 'INVALID_DATE: Tanggal kembali tidak boleh sebelum tanggal pinjam';
    END IF;

    v_total_days := p_end_date - p_start_date + 1;
    v_total_amount := v_total_days * p_daily_price;

    v_rental_id := uuid_generate_v4();
    INSERT INTO daily_rentals (
        id, console_id, customer_id,
        start_date, end_date, daily_price,
        total_days, total_amount, deposit_amount,
        status, notes, created_at, updated_at
    ) VALUES (
        v_rental_id, p_console_id, p_customer_id,
        p_start_date, p_end_date, p_daily_price,
        v_total_days, v_total_amount, p_deposit,
        'active', p_notes, v_now, v_now
    );

    UPDATE consoles SET status = 'rented_out', updated_at = v_now WHERE consoles.id = p_console_id;

    RETURN QUERY SELECT v_rental_id, v_total_days, v_total_amount, 'active'::VARCHAR;
END;
$$;

-- =============================================
-- 5. Overdue auto-set: update auto-stop safety net
--    (done via Go goroutine, no SQL change needed)
-- =============================================
