-- Migration: 000004_add_discount_rules.up.sql
-- Menambahkan sistem diskon otomatis (happy hour, member, hari tertentu)
-- Dapat digunakan bersamaan dengan voucher dalam satu transaksi

-- =============================================
-- Update tabel customers: tambah flag member
-- =============================================
ALTER TABLE customers
    ADD COLUMN IF NOT EXISTS is_member BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_customers_is_member ON customers(is_member);

-- =============================================
-- Tabel: discount_rules
-- Aturan diskon otomatis yang dievaluasi saat pembayaran
-- =============================================
CREATE TABLE IF NOT EXISTS discount_rules (
    id             UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    name           VARCHAR(150) NOT NULL,
    rule_type      VARCHAR(20)  NOT NULL CHECK (rule_type IN ('always', 'happy_hour', 'member', 'day_of_week')),
    discount_type  VARCHAR(20)  NOT NULL CHECK (discount_type IN ('percentage', 'fixed_amount')),
    discount_value NUMERIC(10,2) NOT NULL CHECK (discount_value > 0),
    max_discount   NUMERIC(10,2) NOT NULL DEFAULT 0,   -- batas maks diskon persen (0 = tidak terbatas)
    min_purchase   NUMERIC(10,2) NOT NULL DEFAULT 0,   -- minimal total sebelum rule berlaku
    -- Happy hour: jam mulai dan jam selesai (0-23), bisa lintas tengah malam
    start_hour     SMALLINT     NULL CHECK (start_hour >= 0 AND start_hour <= 23),
    end_hour       SMALLINT     NULL CHECK (end_hour >= 0 AND end_hour <= 23),
    -- Day of week: "0,1,2" (0=Minggu, 1=Senin, ..., 6=Sabtu)
    days_of_week   VARCHAR(20)  NULL,
    priority       INT          NOT NULL DEFAULT 0,    -- lebih tinggi = dievaluasi lebih dulu
    is_active      BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_discount_rules_rule_type  ON discount_rules(rule_type);
CREATE INDEX IF NOT EXISTS idx_discount_rules_is_active  ON discount_rules(is_active);
CREATE INDEX IF NOT EXISTS idx_discount_rules_priority   ON discount_rules(priority DESC);

-- =============================================
-- Update tabel payments: kolom diskon otomatis
-- =============================================
ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS auto_discount_amount NUMERIC(10,2) NOT NULL DEFAULT 0;

-- =============================================
-- FUNCTION: byoneEvaluateDiscountRules
-- Mengevaluasi semua aturan diskon aktif untuk satu sesi
-- Mengembalikan total nominal diskon otomatis
-- =============================================
CREATE OR REPLACE FUNCTION byoneEvaluateDiscountRules(
    p_total_price        NUMERIC,
    p_session_start_time TIMESTAMPTZ,
    p_is_member          BOOLEAN DEFAULT FALSE
)
RETURNS NUMERIC
LANGUAGE plpgsql
AS $$
DECLARE
    v_dow        INT;              -- 0=Minggu, 1=Senin ... 6=Sabtu
    v_hour       INT;              -- jam dalam sehari (0-23) zona WIB
    v_total_disc NUMERIC(10,2) := 0;
    v_disc       NUMERIC(10,2);
    r            RECORD;
BEGIN
    v_dow  := EXTRACT(DOW  FROM p_session_start_time AT TIME ZONE 'Asia/Jakarta')::INT;
    v_hour := EXTRACT(HOUR FROM p_session_start_time AT TIME ZONE 'Asia/Jakarta')::INT;

    FOR r IN
        SELECT * FROM discount_rules
        WHERE is_active = TRUE
        ORDER BY priority DESC, created_at ASC
    LOOP
        -- Evaluasi kondisi rule
        CASE r.rule_type
            WHEN 'always' THEN
                NULL; -- selalu berlaku, tidak ada kondisi tambahan

            WHEN 'happy_hour' THEN
                IF r.start_hour IS NULL OR r.end_hour IS NULL THEN
                    CONTINUE;
                END IF;
                -- Tangani shift melewati tengah malam (misal 22:00 - 02:00)
                IF r.start_hour > r.end_hour THEN
                    IF NOT (v_hour >= r.start_hour OR v_hour < r.end_hour) THEN
                        CONTINUE;
                    END IF;
                ELSE
                    IF NOT (v_hour >= r.start_hour AND v_hour < r.end_hour) THEN
                        CONTINUE;
                    END IF;
                END IF;

            WHEN 'member' THEN
                IF NOT p_is_member THEN
                    CONTINUE;
                END IF;

            WHEN 'day_of_week' THEN
                IF r.days_of_week IS NULL OR TRIM(r.days_of_week) = '' THEN
                    CONTINUE;
                END IF;
                IF NOT (v_dow = ANY(string_to_array(r.days_of_week, ',')::INT[])) THEN
                    CONTINUE;
                END IF;

            ELSE
                CONTINUE;
        END CASE;

        -- Cek minimal pembelian
        IF p_total_price < r.min_purchase THEN
            CONTINUE;
        END IF;

        -- Hitung nilai diskon
        IF r.discount_type = 'percentage' THEN
            v_disc := ROUND((p_total_price * r.discount_value / 100)::NUMERIC, 2);
            IF r.max_discount > 0 AND v_disc > r.max_discount THEN
                v_disc := r.max_discount;
            END IF;
        ELSE
            -- fixed_amount
            v_disc := r.discount_value;
            IF v_disc > p_total_price THEN
                v_disc := p_total_price;
            END IF;
        END IF;

        v_total_disc := v_total_disc + v_disc;
    END LOOP;

    -- Total diskon tidak boleh melebihi total harga
    IF v_total_disc > p_total_price THEN
        v_total_disc := p_total_price;
    END IF;

    RETURN v_total_disc;
END;
$$;

-- =============================================
-- UPDATE STORED PROCEDURE: byoneCreatePayment
-- Menambahkan dukungan diskon otomatis (auto_discount_amount)
-- Diskon otomatis + diskon voucher dapat digabungkan
-- =============================================
-- DROP dulu karena return type berubah (tambah kolom auto_discount_amount)
DROP FUNCTION IF EXISTS byoneCreatePayment(UUID, NUMERIC, TEXT, VARCHAR);

CREATE OR REPLACE FUNCTION byoneCreatePayment(
    p_session_id    UUID,
    p_cash_received NUMERIC,
    p_notes         TEXT    DEFAULT NULL,
    p_voucher_code  VARCHAR DEFAULT NULL
)
RETURNS TABLE (
    payment_id          UUID,
    amount              NUMERIC,
    discount_amount     NUMERIC,
    auto_discount_amount NUMERIC,
    cash_received       NUMERIC,
    change_amount       NUMERIC,
    voucher_id          UUID,
    paid_at             TIMESTAMPTZ
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
    -- Ambil data sesi
    SELECT * INTO v_session FROM sessions WHERE id = p_session_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'SESSION_NOT_FOUND: Sesi tidak ditemukan';
    END IF;

    IF v_session.status != 'completed' THEN
        RAISE EXCEPTION 'SESSION_NOT_COMPLETED: Sesi belum selesai, tidak bisa dibayar';
    END IF;

    -- Cek apakah sudah ada pembayaran aktif
    IF EXISTS (SELECT 1 FROM payments WHERE session_id = p_session_id AND payment_status != 'refunded') THEN
        RAISE EXCEPTION 'PAYMENT_EXISTS: Pembayaran untuk sesi ini sudah ada';
    END IF;

    v_amount := v_session.total_price;

    -- Cek status member pelanggan (jika sesi terkait pelanggan terdaftar)
    IF v_session.customer_id IS NOT NULL THEN
        SELECT COALESCE(is_member, FALSE)
        INTO v_is_member
        FROM customers
        WHERE id = v_session.customer_id;
    END IF;

    -- Evaluasi diskon otomatis berdasarkan aturan aktif
    v_auto_discount := byoneEvaluateDiscountRules(v_amount, v_session.start_time, v_is_member);

    -- Proses voucher jika diberikan (dihitung dari total sebelum auto discount)
    IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code) != '' THEN
        SELECT va.voucher_id, va.discount_amount
        INTO v_voucher_id, v_voucher_discount
        FROM byoneApplyVoucher(p_voucher_code, v_amount) va;

        -- Tambah usage count voucher
        UPDATE vouchers
        SET usage_count = usage_count + 1,
            updated_at  = v_now
        WHERE id = v_voucher_id;
    END IF;

    -- Total diskon gabungan, tidak boleh melebihi total harga
    v_total_discount := v_auto_discount + v_voucher_discount;
    IF v_total_discount > v_amount THEN
        v_total_discount := v_amount;
        -- Proporsikan ulang agar auto_discount tidak negatif
        IF v_auto_discount > v_amount THEN
            v_auto_discount    := v_amount;
            v_voucher_discount := 0;
        ELSE
            v_voucher_discount := v_amount - v_auto_discount;
        END IF;
    END IF;

    -- Hitung final amount setelah semua diskon
    v_final_amount := v_amount - v_total_discount;
    IF v_final_amount < 0 THEN v_final_amount := 0; END IF;

    -- Validasi uang yang diterima
    IF p_cash_received < v_final_amount THEN
        RAISE EXCEPTION 'INSUFFICIENT_CASH: Uang yang diterima kurang. Dibutuhkan Rp %.0f, diterima Rp %.0f',
            v_final_amount, p_cash_received;
    END IF;

    v_change := p_cash_received - v_final_amount;

    -- Insert payment
    INSERT INTO payments (
        session_id, amount, discount_amount, auto_discount_amount, payment_method,
        payment_status, cash_received, change_amount, voucher_id, notes,
        paid_at, created_at, updated_at
    )
    VALUES (
        p_session_id, v_amount, v_voucher_discount, v_auto_discount, 'cash', 'paid',
        p_cash_received, v_change, v_voucher_id, p_notes,
        v_now, v_now, v_now
    )
    RETURNING id INTO v_payment_id;

    RETURN QUERY SELECT
        v_payment_id,
        v_amount,
        v_voucher_discount,
        v_auto_discount,
        p_cash_received,
        v_change,
        v_voucher_id,
        v_now;
END;
$$;
