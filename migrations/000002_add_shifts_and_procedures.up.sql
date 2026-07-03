-- Migration: 000002_add_shifts_and_update_roles.up.sql
-- Penambahan tabel shifts, update role user, update skema payment (cash only)

-- =============================================
-- Update constraint role di tabel users
-- =============================================
ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_role_check;

ALTER TABLE users
    ADD CONSTRAINT users_role_check
    CHECK (role IN ('superadmin', 'admin', 'kasir'));

-- =============================================
-- Tabel: shifts (jadwal shift kasir)
-- =============================================
CREATE TABLE IF NOT EXISTS shifts (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        VARCHAR(100) NOT NULL,
    start_hour  SMALLINT NOT NULL CHECK (start_hour >= 0 AND start_hour <= 23),
    end_hour    SMALLINT NOT NULL CHECK (end_hour >= 0 AND end_hour <= 23),
    is_24_hour  BOOLEAN NOT NULL DEFAULT FALSE,
    status      VARCHAR(20) NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'inactive')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_shifts_user_id ON shifts(user_id);
CREATE INDEX IF NOT EXISTS idx_shifts_status  ON shifts(status);

-- =============================================
-- Update tabel payments: tambah kolom cash & kembalian
-- =============================================
ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS cash_received NUMERIC(10,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS change_amount  NUMERIC(10,2) NOT NULL DEFAULT 0;

-- Update constraint payment_method → hanya cash
ALTER TABLE payments
    DROP CONSTRAINT IF EXISTS payments_payment_method_check;

ALTER TABLE payments
    ADD CONSTRAINT payments_payment_method_check
    CHECK (payment_method IN ('cash'));

-- =============================================
-- STORED PROCEDURES
-- =============================================

-- --------------------------------------------
-- SP: sp_start_session
-- Memulai sesi rental dengan validasi dan update status konsol (atomic)
-- --------------------------------------------
CREATE OR REPLACE FUNCTION sp_start_session(
    p_console_id    UUID,
    p_customer_id   UUID,   -- nullable
    p_notes         TEXT
)
RETURNS TABLE (
    id               UUID,
    console_id       UUID,
    customer_id      UUID,
    start_time       TIMESTAMPTZ,
    end_time         TIMESTAMPTZ,
    duration_minutes INT,
    total_price      NUMERIC,
    status           VARCHAR,
    notes            TEXT,
    created_at       TIMESTAMPTZ,
    updated_at       TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_session_id UUID;
    v_status     VARCHAR;
BEGIN
    -- Cek status konsol
    SELECT c.status INTO v_status FROM consoles c WHERE c.id = p_console_id FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'CONSOLE_NOT_FOUND: Konsol tidak ditemukan';
    END IF;

    IF v_status != 'available' THEN
        RAISE EXCEPTION 'CONSOLE_NOT_AVAILABLE: Konsol tidak tersedia (status: %)', v_status;
    END IF;

    -- Cek sesi aktif yang mungkin masih ada
    IF EXISTS (SELECT 1 FROM sessions s WHERE s.console_id = p_console_id AND s.status = 'active') THEN
        RAISE EXCEPTION 'SESSION_ALREADY_ACTIVE: Konsol masih memiliki sesi aktif';
    END IF;

    -- Buat sesi baru
    v_session_id := uuid_generate_v4();

    INSERT INTO sessions (id, console_id, customer_id, start_time, status, notes, created_at, updated_at)
    VALUES (v_session_id, p_console_id, p_customer_id, NOW(), 'active', p_notes, NOW(), NOW());

    -- Update status konsol
    UPDATE consoles SET status = 'in_use', updated_at = NOW() WHERE id = p_console_id;

    -- Kembalikan data sesi yang baru dibuat
    RETURN QUERY
        SELECT s.id, s.console_id, s.customer_id, s.start_time, s.end_time,
               s.duration_minutes, s.total_price, s.status, s.notes, s.created_at, s.updated_at
        FROM sessions s WHERE s.id = v_session_id;
END;
$$;

-- --------------------------------------------
-- SP: sp_end_session
-- Mengakhiri sesi dan menghitung total harga (atomic)
-- --------------------------------------------
CREATE OR REPLACE FUNCTION sp_end_session(p_session_id UUID)
RETURNS TABLE (
    id               UUID,
    console_id       UUID,
    customer_id      UUID,
    start_time       TIMESTAMPTZ,
    end_time         TIMESTAMPTZ,
    duration_minutes INT,
    total_price      NUMERIC,
    status           VARCHAR,
    notes            TEXT,
    created_at       TIMESTAMPTZ,
    updated_at       TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_session        sessions%ROWTYPE;
    v_price_per_hour NUMERIC;
    v_duration_min   INT;
    v_total_price    NUMERIC;
BEGIN
    -- Ambil sesi dengan lock
    SELECT * INTO v_session FROM sessions WHERE sessions.id = p_session_id FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'SESSION_NOT_FOUND: Sesi tidak ditemukan';
    END IF;

    IF v_session.status != 'active' THEN
        RAISE EXCEPTION 'SESSION_NOT_ACTIVE: Sesi sudah tidak aktif (status: %)', v_session.status;
    END IF;

    -- Ambil harga per jam konsol
    SELECT price_per_hour INTO v_price_per_hour FROM consoles WHERE consoles.id = v_session.console_id;

    -- Hitung durasi dan harga
    v_duration_min := EXTRACT(EPOCH FROM (NOW() - v_session.start_time))::INT / 60;
    v_total_price  := ROUND((v_duration_min::NUMERIC / 60.0) * v_price_per_hour, 2);

    -- Minimal tagih 1 menit
    IF v_duration_min < 1 THEN v_duration_min := 1; END IF;
    IF v_total_price  < 0 THEN v_total_price  := 0; END IF;

    -- Update sesi
    UPDATE sessions
    SET end_time         = NOW(),
        duration_minutes = v_duration_min,
        total_price      = v_total_price,
        status           = 'completed',
        updated_at       = NOW()
    WHERE sessions.id = p_session_id;

    -- Kembalikan status konsol
    UPDATE consoles SET status = 'available', updated_at = NOW() WHERE consoles.id = v_session.console_id;

    RETURN QUERY
        SELECT sessions.id, sessions.console_id, sessions.customer_id, sessions.start_time, sessions.end_time,
               sessions.duration_minutes, sessions.total_price, sessions.status, sessions.notes, sessions.created_at, sessions.updated_at
        FROM sessions WHERE sessions.id = p_session_id;
END;
$$;

-- --------------------------------------------
-- SP: sp_cancel_session
-- Membatalkan sesi aktif dan mengembalikan status konsol
-- --------------------------------------------
CREATE OR REPLACE FUNCTION sp_cancel_session(p_session_id UUID)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    v_console_id UUID;
    v_status     VARCHAR;
BEGIN
    SELECT console_id, status INTO v_console_id, v_status
    FROM sessions WHERE id = p_session_id FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'SESSION_NOT_FOUND: Sesi tidak ditemukan';
    END IF;

    IF v_status != 'active' THEN
        RAISE EXCEPTION 'SESSION_NOT_ACTIVE: Hanya sesi aktif yang dapat dibatalkan (status: %)', v_status;
    END IF;

    UPDATE sessions SET status = 'cancelled', updated_at = NOW() WHERE id = p_session_id;
    UPDATE consoles SET status = 'available', updated_at = NOW() WHERE id = v_console_id;
END;
$$;

-- --------------------------------------------
-- SP: sp_create_payment
-- Membuat tagihan pembayaran tunai untuk sesi yang sudah selesai
-- --------------------------------------------
CREATE OR REPLACE FUNCTION sp_create_payment(
    p_session_id    UUID,
    p_cash_received NUMERIC,
    p_notes         TEXT
)
RETURNS TABLE (
    id              UUID,
    session_id      UUID,
    amount          NUMERIC,
    payment_method  VARCHAR,
    payment_status  VARCHAR,
    paid_at         TIMESTAMPTZ,
    cash_received   NUMERIC,
    change_amount   NUMERIC,
    notes           TEXT,
    created_at      TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_total_price  NUMERIC;
    v_session_status VARCHAR;
    v_payment_id   UUID;
    v_change       NUMERIC;
BEGIN
    -- Validasi sesi
    SELECT total_price, status INTO v_total_price, v_session_status
    FROM sessions WHERE id = p_session_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'SESSION_NOT_FOUND: Sesi tidak ditemukan';
    END IF;

    IF v_session_status != 'completed' THEN
        RAISE EXCEPTION 'SESSION_NOT_COMPLETED: Sesi harus diselesaikan terlebih dahulu';
    END IF;

    -- Cek apakah sudah ada pembayaran
    IF EXISTS (SELECT 1 FROM payments WHERE session_id = p_session_id) THEN
        RAISE EXCEPTION 'PAYMENT_EXISTS: Tagihan untuk sesi ini sudah dibuat';
    END IF;

    -- Validasi uang yang diterima
    IF p_cash_received < v_total_price THEN
        RAISE EXCEPTION 'INSUFFICIENT_CASH: Uang yang diterima (%) kurang dari tagihan (%)',
            p_cash_received, v_total_price;
    END IF;

    v_change     := p_cash_received - v_total_price;
    v_payment_id := uuid_generate_v4();

    INSERT INTO payments (id, session_id, amount, payment_method, payment_status,
                          cash_received, change_amount, notes, created_at, updated_at)
    VALUES (v_payment_id, p_session_id, v_total_price, 'cash', 'paid',
            p_cash_received, v_change, p_notes, NOW(), NOW());

    -- Langsung set paid_at karena tunai = lunas saat itu juga
    UPDATE payments SET paid_at = NOW() WHERE id = v_payment_id;

    RETURN QUERY
        SELECT p.id, p.session_id, p.amount, p.payment_method, p.payment_status,
               p.paid_at, p.cash_received, p.change_amount, p.notes, p.created_at, p.updated_at
        FROM payments p WHERE p.id = v_payment_id;
END;
$$;

-- --------------------------------------------
-- SP: sp_refund_payment
-- Proses refund pembayaran tunai yang sudah lunas
-- --------------------------------------------
CREATE OR REPLACE FUNCTION sp_refund_payment(p_payment_id UUID)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    v_status VARCHAR;
BEGIN
    SELECT payment_status INTO v_status FROM payments WHERE id = p_payment_id FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'PAYMENT_NOT_FOUND: Data pembayaran tidak ditemukan';
    END IF;

    IF v_status != 'paid' THEN
        RAISE EXCEPTION 'PAYMENT_NOT_PAID: Hanya pembayaran lunas yang dapat direfund (status: %)', v_status;
    END IF;

    UPDATE payments SET payment_status = 'refunded', updated_at = NOW() WHERE id = p_payment_id;
END;
$$;

-- --------------------------------------------
-- SP: sp_validate_kasir_shift
-- Validasi apakah kasir dapat login sesuai jadwal shift-nya
-- Mengembalikan TRUE jika diizinkan, FALSE jika tidak
-- --------------------------------------------
CREATE OR REPLACE FUNCTION sp_validate_kasir_shift(p_user_id UUID)
RETURNS BOOLEAN
LANGUAGE plpgsql
AS $$
DECLARE
    v_role       VARCHAR;
    v_hour       SMALLINT;
    v_has_access BOOLEAN := FALSE;
    v_shift      RECORD;
BEGIN
    -- Ambil role user
    SELECT role INTO v_role FROM users WHERE id = p_user_id AND is_active = TRUE;

    IF NOT FOUND THEN
        RETURN FALSE;
    END IF;

    -- Superadmin dan Admin tidak dibatasi shift
    IF v_role IN ('superadmin', 'admin') THEN
        RETURN TRUE;
    END IF;

    -- Untuk kasir, cek jadwal shift
    v_hour := EXTRACT(HOUR FROM NOW() AT TIME ZONE 'Asia/Jakarta')::SMALLINT;

    FOR v_shift IN
        SELECT * FROM shifts
        WHERE user_id = p_user_id AND status = 'active'
    LOOP
        IF v_shift.is_24_hour THEN
            RETURN TRUE;
        END IF;

        -- Shift melewati tengah malam (misal 22:00 - 06:00)
        IF v_shift.start_hour > v_shift.end_hour THEN
            IF v_hour >= v_shift.start_hour OR v_hour < v_shift.end_hour THEN
                RETURN TRUE;
            END IF;
        ELSE
            -- Shift normal (misal 08:00 - 16:00)
            IF v_hour >= v_shift.start_hour AND v_hour < v_shift.end_hour THEN
                RETURN TRUE;
            END IF;
        END IF;
    END LOOP;

    RETURN FALSE;
END;
$$;
