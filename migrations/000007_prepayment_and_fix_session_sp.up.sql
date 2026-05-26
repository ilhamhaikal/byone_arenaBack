-- Migration: 000007_prepayment_and_fix_session_sp.up.sql
-- 1. Fix bug "column reference id is ambiguous" di sp_start_session
-- 2. Tambah sp_start_session_with_payment (buat sesi + bayar di depan, satu transaksi atomik)

-- =============================================
-- 1. Fix sp_start_session — gunakan table name penuh agar tidak ambigu
-- =============================================
DROP FUNCTION IF EXISTS sp_start_session(UUID, UUID, TEXT, INTEGER);

CREATE OR REPLACE FUNCTION sp_start_session(
    p_console_id              UUID,
    p_customer_id             UUID,   -- nullable (walk-in)
    p_notes                   TEXT,
    p_booked_duration_minutes INTEGER DEFAULT 0
)
RETURNS TABLE (
    id                       UUID,
    console_id               UUID,
    customer_id              UUID,
    start_time               TIMESTAMPTZ,
    end_time                 TIMESTAMPTZ,
    end_scheduled_at         TIMESTAMPTZ,
    booked_duration_minutes  INT,
    duration_minutes         INT,
    total_price              NUMERIC,
    status                   VARCHAR,
    notes                    TEXT,
    created_at               TIMESTAMPTZ,
    updated_at               TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_session_id    UUID;
    v_status        VARCHAR;
    v_end_scheduled TIMESTAMPTZ;
BEGIN
    SELECT c.status INTO v_status
    FROM consoles c WHERE c.id = p_console_id FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'CONSOLE_NOT_FOUND: Konsol tidak ditemukan';
    END IF;

    IF v_status != 'available' THEN
        RAISE EXCEPTION 'CONSOLE_NOT_AVAILABLE: Konsol tidak tersedia (status: %)', v_status;
    END IF;

    IF EXISTS (
        SELECT 1 FROM sessions s2
        WHERE s2.console_id = p_console_id AND s2.status = 'active'
    ) THEN
        RAISE EXCEPTION 'SESSION_ALREADY_ACTIVE: Konsol masih memiliki sesi aktif';
    END IF;

    IF p_booked_duration_minutes > 0 THEN
        v_end_scheduled := NOW() + (p_booked_duration_minutes * INTERVAL '1 minute');
    END IF;

    v_session_id := uuid_generate_v4();

    INSERT INTO sessions (
        id, console_id, customer_id,
        start_time, booked_duration_minutes, end_scheduled_at,
        status, notes, created_at, updated_at
    )
    VALUES (
        v_session_id, p_console_id, p_customer_id,
        NOW(), p_booked_duration_minutes, v_end_scheduled,
        'active', p_notes, NOW(), NOW()
    );

    UPDATE consoles SET status = 'in_use', updated_at = NOW() WHERE id = p_console_id;

    -- Gunakan nama tabel penuh (bukan alias) untuk hindari ambiguitas dengan output column
    RETURN QUERY
        SELECT
            sessions.id,
            sessions.console_id,
            sessions.customer_id,
            sessions.start_time,
            sessions.end_time,
            sessions.end_scheduled_at,
            sessions.booked_duration_minutes,
            sessions.duration_minutes,
            sessions.total_price,
            sessions.status,
            sessions.notes,
            sessions.created_at,
            sessions.updated_at
        FROM sessions
        WHERE sessions.id = v_session_id;
END;
$$;


-- =============================================
-- 2. sp_start_session_with_payment
-- Membuat sesi + pembayaran di depan dalam satu transaksi atomik.
-- Harga dihitung dari booked_duration_minutes × price_per_hour.
-- Diskon otomatis (happy hour, member, dll) + voucher opsional diterapkan.
-- =============================================
CREATE OR REPLACE FUNCTION sp_start_session_with_payment(
    p_console_id              UUID,
    p_customer_id             UUID,   -- nullable (walk-in)
    p_notes                   TEXT,
    p_booked_duration_minutes INTEGER,
    p_cash_received           NUMERIC,
    p_voucher_code            VARCHAR DEFAULT NULL
)
RETURNS TABLE (
    -- sesi (nama kolom beda agar tidak ambigu)
    session_id              UUID,
    session_status          VARCHAR,
    session_start_time      TIMESTAMPTZ,
    session_booked_minutes  INT,
    session_end_scheduled   TIMESTAMPTZ,
    -- pembayaran
    payment_id              UUID,
    base_amount             NUMERIC,    -- harga sebelum diskon
    discount_amount         NUMERIC,    -- diskon voucher
    auto_discount_amount    NUMERIC,    -- diskon otomatis
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
    -- Validasi durasi
    IF p_booked_duration_minutes < 60 THEN
        RAISE EXCEPTION 'INVALID_DURATION: Durasi minimal 60 menit (1 jam)';
    END IF;

    -- Ambil dan kunci konsol
    SELECT c.status, c.price_per_hour
    INTO v_console_status, v_price_per_hour
    FROM consoles c
    WHERE c.id = p_console_id
    FOR UPDATE;

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

    -- Hitung harga berdasarkan durasi yang dipesan
    v_amount        := ROUND((p_booked_duration_minutes::NUMERIC / 60.0) * v_price_per_hour, 2);
    v_end_scheduled := v_now + (p_booked_duration_minutes * INTERVAL '1 minute');

    -- Cek status member pelanggan
    IF p_customer_id IS NOT NULL THEN
        SELECT COALESCE(is_member, FALSE)
        INTO v_is_member
        FROM customers
        WHERE id = p_customer_id;
    END IF;

    -- Evaluasi diskon otomatis (happy hour, member, day_of_week, dll)
    v_auto_discount := sp_evaluate_discount_rules(v_amount, v_now, v_is_member);

    -- Proses voucher jika ada
    IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code) != '' THEN
        SELECT va.voucher_id, va.discount_amount
        INTO v_voucher_id, v_voucher_discount
        FROM sp_apply_voucher(p_voucher_code, v_amount) va;

        UPDATE vouchers
        SET usage_count = usage_count + 1,
            updated_at  = v_now
        WHERE id = v_voucher_id;
    END IF;

    -- Gabungkan diskon, tidak boleh melebihi total harga
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

    -- Validasi uang yang diterima
    IF p_cash_received < v_final_amount THEN
        RAISE EXCEPTION 'INSUFFICIENT_CASH: Uang yang diterima kurang. Dibutuhkan Rp %.0f, diterima Rp %.0f',
            v_final_amount, p_cash_received;
    END IF;

    v_change := p_cash_received - v_final_amount;

    -- Buat sesi
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

    -- Tandai konsol sebagai sedang digunakan
    UPDATE consoles SET status = 'in_use', updated_at = v_now WHERE id = p_console_id;

    -- Buat pembayaran (sudah lunas di depan)
    v_payment_id := uuid_generate_v4();
    INSERT INTO payments (
        id, session_id, amount, discount_amount, auto_discount_amount,
        payment_method, payment_status, cash_received, change_amount,
        voucher_id, notes, paid_at, created_at, updated_at
    )
    VALUES (
        v_payment_id, v_session_id,
        v_amount, v_voucher_discount, v_auto_discount,
        'cash', 'paid',
        p_cash_received, v_change,
        v_voucher_id, p_notes,
        v_now, v_now, v_now
    );

    -- Kembalikan hasil
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
        p_cash_received,
        v_change,
        v_voucher_id,
        v_now;
END;
$$;
