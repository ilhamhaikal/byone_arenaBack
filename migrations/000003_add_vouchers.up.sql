-- Migration: 000003_add_vouchers.up.sql
-- Menambahkan sistem voucher diskon untuk pembayaran rental

-- =============================================
-- Tabel: vouchers
-- =============================================
CREATE TABLE IF NOT EXISTS vouchers (
    id             UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    code           VARCHAR(50)  NOT NULL UNIQUE,
    name           VARCHAR(150) NOT NULL,
    discount_type  VARCHAR(20)  NOT NULL CHECK (discount_type IN ('percentage', 'fixed_amount')),
    discount_value NUMERIC(10,2) NOT NULL CHECK (discount_value > 0),
    min_purchase   NUMERIC(10,2) NOT NULL DEFAULT 0,   -- minimal total sebelum voucher berlaku
    max_discount   NUMERIC(10,2) NOT NULL DEFAULT 0,   -- maksimal diskon (0 = tidak terbatas), hanya untuk percentage
    max_usage      INT          NOT NULL DEFAULT 0,    -- maksimal total penggunaan (0 = tidak terbatas)
    usage_count    INT          NOT NULL DEFAULT 0,
    is_active      BOOLEAN      NOT NULL DEFAULT TRUE,
    expires_at     TIMESTAMPTZ  NULL,                  -- NULL = tidak ada batas waktu
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_vouchers_code      ON vouchers(code);
CREATE INDEX IF NOT EXISTS idx_vouchers_is_active ON vouchers(is_active);

-- =============================================
-- Update tabel payments: kolom voucher & diskon
-- =============================================
ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS voucher_id      UUID         NULL REFERENCES vouchers(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS discount_amount NUMERIC(10,2) NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_payments_voucher_id ON payments(voucher_id);

-- =============================================
-- STORED PROCEDURE: sp_apply_voucher
-- Memvalidasi dan menghitung diskon dari kode voucher
-- Returns: discount_amount yang harus dikurangi dari total
-- =============================================
CREATE OR REPLACE FUNCTION sp_apply_voucher(
    p_code        VARCHAR,
    p_total_price NUMERIC
)
RETURNS TABLE (
    voucher_id      UUID,
    discount_amount NUMERIC
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_voucher       vouchers%ROWTYPE;
    v_discount      NUMERIC(10,2) := 0;
BEGIN
    -- Ambil voucher berdasarkan kode
    SELECT * INTO v_voucher
    FROM vouchers
    WHERE code = UPPER(p_code);

    IF NOT FOUND THEN
        RAISE EXCEPTION 'VOUCHER_NOT_FOUND: Kode voucher tidak ditemukan';
    END IF;

    -- Cek status aktif
    IF NOT v_voucher.is_active THEN
        RAISE EXCEPTION 'VOUCHER_INACTIVE: Voucher sudah tidak aktif';
    END IF;

    -- Cek batas waktu
    IF v_voucher.expires_at IS NOT NULL AND v_voucher.expires_at < NOW() THEN
        RAISE EXCEPTION 'VOUCHER_EXPIRED: Voucher sudah kadaluarsa';
    END IF;

    -- Cek batas penggunaan
    IF v_voucher.max_usage > 0 AND v_voucher.usage_count >= v_voucher.max_usage THEN
        RAISE EXCEPTION 'VOUCHER_LIMIT_REACHED: Voucher sudah mencapai batas penggunaan';
    END IF;

    -- Cek minimal pembelian
    IF p_total_price < v_voucher.min_purchase THEN
        RAISE EXCEPTION 'VOUCHER_MIN_PURCHASE: Minimal pembelian Rp %.0f untuk menggunakan voucher ini', v_voucher.min_purchase;
    END IF;

    -- Hitung diskon
    IF v_voucher.discount_type = 'percentage' THEN
        v_discount := ROUND((p_total_price * v_voucher.discount_value / 100)::NUMERIC, 2);
        -- Terapkan batas maksimal diskon jika ada
        IF v_voucher.max_discount > 0 AND v_discount > v_voucher.max_discount THEN
            v_discount := v_voucher.max_discount;
        END IF;
    ELSE
        -- fixed_amount
        v_discount := v_voucher.discount_value;
        -- Diskon tidak boleh melebihi total harga
        IF v_discount > p_total_price THEN
            v_discount := p_total_price;
        END IF;
    END IF;

    RETURN QUERY SELECT v_voucher.id, v_discount;
END;
$$;

-- =============================================
-- UPDATE STORED PROCEDURE: sp_create_payment
-- Menambahkan dukungan voucher diskon
-- =============================================
CREATE OR REPLACE FUNCTION sp_create_payment(
    p_session_id    UUID,
    p_cash_received NUMERIC,
    p_notes         TEXT DEFAULT NULL,
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
    -- Ambil data sesi
    SELECT * INTO v_session FROM sessions WHERE id = p_session_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'SESSION_NOT_FOUND: Sesi tidak ditemukan';
    END IF;

    IF v_session.status != 'completed' THEN
        RAISE EXCEPTION 'SESSION_NOT_COMPLETED: Sesi belum selesai, tidak bisa dibayar';
    END IF;

    -- Cek apakah sudah ada pembayaran
    IF EXISTS (SELECT 1 FROM payments WHERE session_id = p_session_id AND payment_status != 'refunded') THEN
        RAISE EXCEPTION 'PAYMENT_EXISTS: Pembayaran untuk sesi ini sudah ada';
    END IF;

    v_amount := v_session.total_price;

    -- Proses voucher jika diberikan
    IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code) != '' THEN
        SELECT va.voucher_id, va.discount_amount
        INTO v_voucher_id, v_discount
        FROM sp_apply_voucher(p_voucher_code, v_amount) va;

        -- Tambah usage count voucher
        UPDATE vouchers
        SET usage_count = usage_count + 1,
            updated_at  = v_now
        WHERE id = v_voucher_id;
    END IF;

    -- Hitung final amount setelah diskon
    v_final_amount := v_amount - v_discount;
    IF v_final_amount < 0 THEN v_final_amount := 0; END IF;

    -- Validasi uang yang diterima
    IF p_cash_received < v_final_amount THEN
        RAISE EXCEPTION 'INSUFFICIENT_CASH: Uang yang diterima kurang. Dibutuhkan Rp %.0f, diterima Rp %.0f', v_final_amount, p_cash_received;
    END IF;

    v_change := p_cash_received - v_final_amount;

    -- Insert payment
    INSERT INTO payments (
        session_id, amount, discount_amount, payment_method, payment_status,
        cash_received, change_amount, voucher_id, notes, paid_at, created_at, updated_at
    )
    VALUES (
        p_session_id, v_amount, v_discount, 'cash', 'paid',
        p_cash_received, v_change, v_voucher_id, p_notes, v_now, v_now, v_now
    )
    RETURNING id INTO v_payment_id;

    RETURN QUERY SELECT v_payment_id, v_amount, v_discount, p_cash_received, v_change, v_voucher_id, v_now;
END;
$$;
