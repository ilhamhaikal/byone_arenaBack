-- Migration: 000001_init_schema.up.sql
-- Inisialisasi skema database untuk sistem rental PS Byone Arena

-- Extension untuk generate UUID
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- =============================================
-- Tabel: users (admin & operator)
-- =============================================
CREATE TABLE IF NOT EXISTS users (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username    VARCHAR(50) NOT NULL UNIQUE,
    password    VARCHAR(255) NOT NULL,
    full_name   VARCHAR(100) NOT NULL,
    role        VARCHAR(20) NOT NULL CHECK (role IN ('admin', 'operator')),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================
-- Tabel: consoles (unit PlayStation)
-- =============================================
CREATE TABLE IF NOT EXISTS consoles (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(100) NOT NULL,
    console_type    VARCHAR(10) NOT NULL CHECK (console_type IN ('PS3', 'PS4', 'PS5')),
    status          VARCHAR(20) NOT NULL DEFAULT 'available'
                        CHECK (status IN ('available', 'in_use', 'maintenance')),
    price_per_hour  NUMERIC(10, 2) NOT NULL CHECK (price_per_hour > 0),
    description     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================
-- Tabel: customers (pelanggan terdaftar)
-- =============================================
CREATE TABLE IF NOT EXISTS customers (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(100) NOT NULL,
    phone       VARCHAR(20) NOT NULL UNIQUE,
    email       VARCHAR(150),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================
-- Tabel: sessions (sesi rental)
-- =============================================
CREATE TABLE IF NOT EXISTS sessions (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    console_id          UUID NOT NULL REFERENCES consoles(id),
    customer_id         UUID REFERENCES customers(id),  -- nullable untuk walk-in
    start_time          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    end_time            TIMESTAMPTZ,
    duration_minutes    INTEGER NOT NULL DEFAULT 0,
    total_price         NUMERIC(10, 2) NOT NULL DEFAULT 0,
    status              VARCHAR(20) NOT NULL DEFAULT 'active'
                            CHECK (status IN ('active', 'completed', 'cancelled')),
    notes               TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================
-- Tabel: payments (pembayaran)
-- =============================================
CREATE TABLE IF NOT EXISTS payments (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id      UUID NOT NULL REFERENCES sessions(id) UNIQUE,
    amount          NUMERIC(10, 2) NOT NULL CHECK (amount >= 0),
    payment_method  VARCHAR(20) NOT NULL CHECK (payment_method IN ('cash', 'transfer', 'qris')),
    payment_status  VARCHAR(20) NOT NULL DEFAULT 'pending'
                        CHECK (payment_status IN ('pending', 'paid', 'refunded')),
    paid_at         TIMESTAMPTZ,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================
-- Index untuk performa query
-- =============================================
CREATE INDEX IF NOT EXISTS idx_consoles_status ON consoles(status);
CREATE INDEX IF NOT EXISTS idx_sessions_console_id ON sessions(console_id);
CREATE INDEX IF NOT EXISTS idx_sessions_customer_id ON sessions(customer_id);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_start_time ON sessions(start_time DESC);
CREATE INDEX IF NOT EXISTS idx_payments_session_id ON payments(session_id);
CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(payment_status);

-- =============================================
-- Data awal: Admin default
-- Password: Admin@123 (bcrypt hash)
-- =============================================
INSERT INTO users (id, username, password, full_name, role, is_active)
VALUES (
    uuid_generate_v4(),
    'admin',
    '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', -- password: password
    'Administrator',
    'admin',
    TRUE
) ON CONFLICT (username) DO NOTHING;
