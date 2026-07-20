-- Migration: 000018_add_daily_rentals_bookings_membership.up.sql
-- 1. Daily Rental — sewa harian (console dibawa pulang)
-- 2. Booking — reservasi konsol untuk waktu tertentu
-- 3. Membership — data keanggotaan + harga

-- =============================================
-- 1. Update consoles: tambah status 'rented_out'
-- =============================================
ALTER TABLE consoles DROP CONSTRAINT IF EXISTS consoles_status_check;
ALTER TABLE consoles ADD CONSTRAINT consoles_status_check
    CHECK (status IN ('available', 'in_use', 'maintenance', 'rented_out'));

-- =============================================
-- 2. Daily Rentals
-- =============================================
CREATE TABLE IF NOT EXISTS daily_rentals (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    console_id      UUID NOT NULL REFERENCES consoles(id),
    customer_id     UUID NOT NULL REFERENCES customers(id),
    start_date      DATE NOT NULL,
    end_date        DATE NOT NULL,                     -- tanggal kembali
    daily_price     NUMERIC(10,2) NOT NULL CHECK (daily_price > 0),
    total_days      INT NOT NULL DEFAULT 1,
    free_days_used  INT NOT NULL DEFAULT 0,              -- jumlah hari gratis dari voucher
    total_amount    NUMERIC(10,2) NOT NULL,
    discount_amount NUMERIC(10,2) NOT NULL DEFAULT 0,    -- diskon dari voucher
    final_amount    NUMERIC(10,2) NOT NULL DEFAULT 0,    -- harga setelah diskon
    status          VARCHAR(20) NOT NULL DEFAULT 'active'
                        CHECK (status IN ('active', 'returned', 'overdue')),
    notes           TEXT,
    returned_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_daily_rentals_console ON daily_rentals(console_id);
CREATE INDEX IF NOT EXISTS idx_daily_rentals_customer ON daily_rentals(customer_id);
CREATE INDEX IF NOT EXISTS idx_daily_rentals_status ON daily_rentals(status);

-- SP: byoneCreateDailyRental — buat rental harian + voucher
CREATE OR REPLACE FUNCTION "byoneCreateDailyRental"(
    p_console_id    UUID,
    p_customer_id   UUID,
    p_start_date    DATE,
    p_end_date      DATE,
    p_daily_price   NUMERIC,
    p_voucher_code  VARCHAR DEFAULT NULL,
    p_notes         TEXT DEFAULT NULL
)
RETURNS TABLE (
    rental_id       UUID,
    total_days      INT,
    total_amount    NUMERIC,
    status          VARCHAR,
    payment_id      UUID
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_rental_id         UUID;
    v_payment_id        UUID;
    v_console_stat      VARCHAR;
    v_total_days        INT;
    v_total_amount      NUMERIC(10,2);
    v_voucher_discount  NUMERIC(10,2) := 0;
    v_final_amount      NUMERIC(10,2);
    v_now               TIMESTAMPTZ := NOW();
BEGIN
    -- Validasi konsol
    SELECT c.status INTO v_console_stat FROM consoles c WHERE c.id = p_console_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'CONSOLE_NOT_FOUND: Konsol tidak ditemukan'; END IF;
    IF v_console_stat != 'available' THEN
        RAISE EXCEPTION 'CONSOLE_NOT_AVAILABLE: Konsol tidak tersedia (status: %)', v_console_stat;
    END IF;

    -- Validasi tanggal
    IF p_end_date <= p_start_date THEN
        RAISE EXCEPTION 'INVALID_DATE: Tanggal kembali harus setelah tanggal pinjam';
    END IF;

    -- 20 ke 23 = 3 hari (20, 21, 22)
    v_total_days := p_end_date - p_start_date;
    IF v_total_days < 1 THEN v_total_days := 1; END IF;
    v_total_amount := v_total_days * p_daily_price;

    -- Apply voucher jika ada (support free_days, percentage, fixed_amount)
    IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code) != '' THEN
        BEGIN
            SELECT va.discount_amount INTO v_voucher_discount
            FROM "byoneApplyVoucher"(p_voucher_code, v_total_amount, p_daily_price, v_total_days) va;
            UPDATE vouchers SET usage_count = usage_count + 1, updated_at = v_now
            WHERE code = UPPER(p_voucher_code);
        EXCEPTION WHEN OTHERS THEN
            v_voucher_discount := 0;
        END;
    END IF;

    IF v_voucher_discount > v_total_amount THEN v_voucher_discount := v_total_amount; END IF;
    v_final_amount := v_total_amount - v_voucher_discount;

    -- Buat rental
    v_rental_id := uuid_generate_v4();
    INSERT INTO daily_rentals (
        id, console_id, customer_id,
        start_date, end_date, daily_price,
        total_days, total_amount,
        status, notes, created_at, updated_at
    ) VALUES (
        v_rental_id, p_console_id, p_customer_id,
        p_start_date, p_end_date, p_daily_price,
        v_total_days, v_total_amount,
        'active', p_notes, v_now, v_now
    );

    -- Tandai konsol rented_out
    UPDATE consoles SET status = 'rented_out', updated_at = v_now WHERE consoles.id = p_console_id;

    -- Buat pembayaran
    v_payment_id := uuid_generate_v4();
    INSERT INTO payments (
        id, session_id, amount, discount_amount, auto_discount_amount,
        total_payment, payment_method, payment_status,
        cash_received, change_amount, notes,
        paid_at, created_at, updated_at
    ) VALUES (
        v_payment_id, '00000000-0000-0000-0000-000000000000',  -- no session
        v_final_amount, v_voucher_discount, 0,
        v_final_amount, 'cash', 'paid',
        v_final_amount, 0, p_notes,
        v_now, v_now, v_now
    );

    RETURN QUERY SELECT v_rental_id, v_total_days, v_total_amount, 'active'::VARCHAR, v_payment_id;
END;
$$;

-- SP: byoneReturnDailyRental — kembalikan rental harian
CREATE OR REPLACE FUNCTION "byoneReturnDailyRental"(p_rental_id UUID)
RETURNS TABLE (
    rental_id     UUID,
    status        VARCHAR,
    returned_at   TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_rental daily_rentals%ROWTYPE;
    v_now    TIMESTAMPTZ := NOW();
BEGIN
    SELECT * INTO v_rental FROM daily_rentals WHERE daily_rentals.id = p_rental_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'RENTAL_NOT_FOUND: Rental tidak ditemukan'; END IF;
    IF v_rental.status != 'active' AND v_rental.status != 'overdue' THEN
        RAISE EXCEPTION 'RENTAL_NOT_ACTIVE: Rental sudah dikembalikan';
    END IF;

    UPDATE daily_rentals
    SET status = 'returned', returned_at = v_now, updated_at = v_now
    WHERE daily_rentals.id = p_rental_id;

    UPDATE consoles SET status = 'available', updated_at = v_now WHERE consoles.id = v_rental.console_id;

    RETURN QUERY SELECT p_rental_id, 'returned'::VARCHAR, v_now;
END;
$$;

-- =============================================
-- 3. Bookings — reservasi konsol
-- =============================================
CREATE TABLE IF NOT EXISTS bookings (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    console_id       UUID NOT NULL REFERENCES consoles(id),
    customer_id      UUID NOT NULL REFERENCES customers(id),
    booking_date     DATE NOT NULL,                      -- tanggal booking
    start_hour       INT NOT NULL CHECK (start_hour >= 0 AND start_hour <= 23),
    start_minute     INT NOT NULL DEFAULT 0 CHECK (start_minute >= 0 AND start_minute <= 59),
    duration_minutes INT NOT NULL CHECK (duration_minutes >= 30),
    status           VARCHAR(20) NOT NULL DEFAULT 'pending'
                         CHECK (status IN ('pending', 'confirmed', 'cancelled', 'completed')),
    notes            TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_bookings_console ON bookings(console_id);
CREATE INDEX IF NOT EXISTS idx_bookings_date ON bookings(booking_date);
CREATE INDEX IF NOT EXISTS idx_bookings_status ON bookings(status);

-- SP: byoneCreateBooking — buat booking dengan validasi overlap
CREATE OR REPLACE FUNCTION "byoneCreateBooking"(
    p_console_id       UUID,
    p_customer_id      UUID,
    p_booking_date     DATE,
    p_start_hour       INT,
    p_start_minute     INT,
    p_duration_minutes INT,
    p_notes            TEXT DEFAULT NULL
)
RETURNS TABLE (
    booking_id UUID,
    status     VARCHAR
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_id               UUID;
    v_booking_start    TIMESTAMPTZ;
    v_booking_end      TIMESTAMPTZ;
    v_conflict_count   INT;
    v_customer_exists  BOOLEAN;
    v_console_exists   BOOLEAN;
BEGIN
    -- Validasi input
    IF p_duration_minutes < 30 THEN
        RAISE EXCEPTION 'INVALID_DURATION: Durasi minimal 30 menit';
    END IF;

    -- Cek konsol & customer exist
    SELECT EXISTS(SELECT 1 FROM consoles WHERE id = p_console_id) INTO v_console_exists;
    IF NOT v_console_exists THEN RAISE EXCEPTION 'CONSOLE_NOT_FOUND'; END IF;

    SELECT EXISTS(SELECT 1 FROM customers WHERE id = p_customer_id) INTO v_customer_exists;
    IF NOT v_customer_exists THEN RAISE EXCEPTION 'CUSTOMER_NOT_FOUND'; END IF;

    -- Hitung waktu booking
    v_booking_start := p_booking_date + make_time(p_start_hour, p_start_minute, 0);
    v_booking_end   := v_booking_start + (p_duration_minutes * INTERVAL '1 minute');

    -- Cek overlap: tidak boleh ada sesi aktif atau booking confirmed di jam yang sama
    SELECT COUNT(*) INTO v_conflict_count
    FROM (
        -- Sesi aktif
        SELECT s.console_id FROM sessions s
        WHERE s.console_id = p_console_id
          AND s.status = 'active'
          AND s.start_time < v_booking_end
          AND COALESCE(s.end_scheduled_at, s.start_time + (s.booked_duration_minutes * INTERVAL '1 minute')) > v_booking_start
        UNION ALL
        -- Booking confirmed
        SELECT b.console_id FROM bookings b
        WHERE b.console_id = p_console_id
          AND b.status IN ('pending', 'confirmed')
          AND b.booking_date = p_booking_date
          AND make_time(b.start_hour, b.start_minute, 0) < v_booking_end::TIME
          AND make_time(b.start_hour, b.start_minute, 0) + (b.duration_minutes * INTERVAL '1 minute') > v_booking_start::TIME
    ) conflicts;

    IF v_conflict_count > 0 THEN
        RAISE EXCEPTION 'BOOKING_CONFLICT: Sudah ada booking atau sesi aktif di jam tersebut';
    END IF;

    v_id := uuid_generate_v4();
    INSERT INTO bookings (id, console_id, customer_id, booking_date, start_hour, start_minute, duration_minutes, status, notes, created_at, updated_at)
    VALUES (v_id, p_console_id, p_customer_id, p_booking_date, p_start_hour, p_start_minute, p_duration_minutes, 'pending', p_notes, NOW(), NOW());

    RETURN QUERY SELECT v_id, 'pending'::VARCHAR;
END;
$$;

-- =============================================
-- 4. Membership — update tabel customers
-- =============================================
ALTER TABLE customers ADD COLUMN IF NOT EXISTS membership_type VARCHAR(20)
    CHECK (membership_type IS NULL OR membership_type IN ('monthly', 'yearly', 'lifetime'));
ALTER TABLE customers ADD COLUMN IF NOT EXISTS membership_start DATE;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS membership_expiry DATE;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS membership_price NUMERIC(10,2) DEFAULT 0;
