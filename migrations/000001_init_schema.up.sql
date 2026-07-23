-- ============================================
-- CONSOLIDATED SCHEMA: Byone Arena PS Rental
-- Final version — 2026-07-23
-- ============================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================
-- 1. USERS
-- ============================================
CREATE TABLE IF NOT EXISTS users (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username    VARCHAR(50) NOT NULL UNIQUE,
    password    VARCHAR(255) NOT NULL,
    full_name   VARCHAR(100) NOT NULL,
    role        VARCHAR(20) NOT NULL CHECK (role IN ('admin', 'superadmin', 'kasir')),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================
-- 2. CONSOLES
-- ============================================
CREATE TABLE IF NOT EXISTS consoles (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(100) NOT NULL,
    console_type    VARCHAR(15) NOT NULL CHECK (console_type IN ('PS3', 'PS4', 'PS5', 'AndroidTV')),
    ip_address      VARCHAR(50),
    adb_port        INT NOT NULL DEFAULT 5555,
    mac_address     VARCHAR(20),
    status          VARCHAR(20) NOT NULL DEFAULT 'available'
                        CHECK (status IN ('available', 'in_use', 'maintenance', 'rented_out')),
    screen_status   VARCHAR(20) NOT NULL DEFAULT 'off'
                        CHECK (screen_status IN ('on', 'off')),
    price_per_hour  NUMERIC(10, 2) NOT NULL CHECK (price_per_hour > 0),
    daily_price     NUMERIC(10, 2) NOT NULL DEFAULT 0,
    pricing_tiers   JSONB DEFAULT '[]',
    description     TEXT,
    last_seen_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_consoles_status ON consoles(status);
CREATE INDEX IF NOT EXISTS idx_consoles_console_type ON consoles(console_type);

-- ============================================
-- 3. CUSTOMERS
-- ============================================
CREATE TABLE IF NOT EXISTS customers (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name             VARCHAR(100) NOT NULL,
    phone            VARCHAR(20) NOT NULL UNIQUE,
    email            VARCHAR(150),
    is_member        BOOLEAN NOT NULL DEFAULT FALSE,
    membership_type  VARCHAR(20),
    membership_start DATE,
    membership_expiry DATE,
    membership_price NUMERIC(10, 2) NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_customers_phone ON customers(phone);
CREATE INDEX IF NOT EXISTS idx_customers_is_member ON customers(is_member);

-- ============================================
-- 4. MEMBERSHIP
-- ============================================
CREATE TABLE IF NOT EXISTS memberships (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    customer_id     UUID NOT NULL UNIQUE REFERENCES customers(id),
    membership_type VARCHAR(20) NOT NULL DEFAULT 'regular'
                        CHECK (membership_type IN ('regular', 'silver', 'gold', 'platinum')),
    discount_percent NUMERIC(5,2) NOT NULL DEFAULT 0,
    total_spent      NUMERIC(12,2) NOT NULL DEFAULT 0,
    points           INT NOT NULL DEFAULT 0,
    started_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at       TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_memberships_customer_id ON memberships(customer_id);

-- ============================================
-- 5. SHIFTS
-- ============================================
CREATE TABLE IF NOT EXISTS shifts (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id),
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
CREATE INDEX IF NOT EXISTS idx_shifts_status ON shifts(status);
CREATE INDEX IF NOT EXISTS idx_shifts_active ON shifts(is_active);

-- ============================================
-- 6. SESSIONS
-- ============================================
CREATE TABLE IF NOT EXISTS sessions (
    id                       UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    console_id               UUID NOT NULL REFERENCES consoles(id),
    customer_id              UUID REFERENCES customers(id),
    start_time               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    end_time                 TIMESTAMPTZ,
    booked_duration_minutes  INT NOT NULL DEFAULT 0,
    end_scheduled_at         TIMESTAMPTZ,
    duration_minutes         INT NOT NULL DEFAULT 0,
    total_price              NUMERIC(10, 2) NOT NULL DEFAULT 0,
    status                   VARCHAR(20) NOT NULL DEFAULT 'active'
                                CHECK (status IN ('active', 'completed', 'cancelled')),
    notes                    TEXT,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_console_id ON sessions(console_id);
CREATE INDEX IF NOT EXISTS idx_sessions_customer_id ON sessions(customer_id);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_start_time ON sessions(start_time DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_end_scheduled_at ON sessions(end_scheduled_at);

-- ============================================
-- 7. PAYMENTS
-- ============================================
CREATE TABLE IF NOT EXISTS payments (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id          UUID NOT NULL REFERENCES sessions(id),
    voucher_id          UUID REFERENCES vouchers(id),
    amount              NUMERIC(10, 2) NOT NULL CHECK (amount >= 0),
    discount_amount     NUMERIC(10, 2) NOT NULL DEFAULT 0,
    auto_discount_amount NUMERIC(10, 2) NOT NULL DEFAULT 0,
    total_payment       NUMERIC(10, 2) NOT NULL DEFAULT 0,
    payment_method      VARCHAR(20) NOT NULL DEFAULT 'cash'
                            CHECK (payment_method IN ('cash', 'transfer', 'qris')),
    payment_status      VARCHAR(20) NOT NULL DEFAULT 'pending'
                            CHECK (payment_status IN ('pending', 'paid', 'refunded')),
    cash_received       NUMERIC(10, 2) NOT NULL DEFAULT 0,
    change_amount       NUMERIC(10, 2) NOT NULL DEFAULT 0,
    paid_at             TIMESTAMPTZ,
    notes               TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payments_session_id ON payments(session_id);
CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(payment_status);
CREATE INDEX IF NOT EXISTS idx_payments_voucher_id ON payments(voucher_id);

-- ============================================
-- 8. VOUCHERS
-- ============================================
CREATE TABLE IF NOT EXISTS vouchers (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code            VARCHAR(50) NOT NULL UNIQUE,
    name            VARCHAR(150) NOT NULL,
    discount_type   VARCHAR(20) NOT NULL CHECK (discount_type IN ('percentage', 'fixed_amount', 'free_days')),
    discount_value  NUMERIC(10, 2) NOT NULL CHECK (discount_value >= 0),
    min_purchase    NUMERIC(10, 2) NOT NULL DEFAULT 0,
    max_discount    NUMERIC(10, 2) NOT NULL DEFAULT 0,
    max_usage       INT NOT NULL DEFAULT 0,
    usage_count     INT NOT NULL DEFAULT 0,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_vouchers_code ON vouchers(code);
CREATE INDEX IF NOT EXISTS idx_vouchers_active ON vouchers(is_active, valid_from, valid_until);

-- ============================================
-- 9. DISCOUNT RULES (auto-discount: happy hour, member, etc.)
-- ============================================
CREATE TABLE IF NOT EXISTS discount_rules (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(150) NOT NULL,
    rule_type       VARCHAR(20) NOT NULL CHECK (rule_type IN ('happy_hour', 'member', 'bulk', 'promo', 'always', 'day_of_week')),
    discount_type   VARCHAR(20) NOT NULL CHECK (discount_type IN ('percentage', 'fixed_amount')),
    discount_value  NUMERIC(10, 2) NOT NULL CHECK (discount_value >= 0),
    max_discount    NUMERIC(10, 2) NOT NULL DEFAULT 0,
    min_purchase    NUMERIC(10, 2) NOT NULL DEFAULT 0,
    start_hour      SMALLINT CHECK (start_hour >= 0 AND start_hour <= 23),
    end_hour        SMALLINT CHECK (end_hour >= 0 AND end_hour <= 23),
    days_of_week    VARCHAR(20),
    priority        INT NOT NULL DEFAULT 0,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_discount_rules_active ON discount_rules(is_active);
CREATE INDEX IF NOT EXISTS idx_discount_rules_type ON discount_rules(rule_type);

-- ============================================
-- 10. PRICING TIERS (console-specific)
-- ============================================
CREATE TABLE IF NOT EXISTS console_pricing_tiers (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    console_id      UUID NOT NULL REFERENCES consoles(id) ON DELETE CASCADE,
    start_minute    INT NOT NULL DEFAULT 0,
    end_minute      INT,   -- NULL = unlimited
    price_per_hour  NUMERIC(10, 2) NOT NULL CHECK (price_per_hour > 0),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pricing_tiers_console ON console_pricing_tiers(console_id);

-- ============================================
-- 11. MENU ITEMS (food & drinks)
-- ============================================
CREATE TABLE IF NOT EXISTS menu_items (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name         VARCHAR(150) NOT NULL,
    category     VARCHAR(30) NOT NULL DEFAULT 'food'
                     CHECK (category IN ('food', 'drink', 'snack', 'other')),
    price        NUMERIC(10,2) NOT NULL CHECK (price >= 0),
    description  TEXT,
    is_available BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_menu_items_category ON menu_items(category);
CREATE INDEX IF NOT EXISTS idx_menu_items_available ON menu_items(is_available);

-- ============================================
-- 12. FOOD ORDERS
-- ============================================
CREATE TABLE IF NOT EXISTS food_orders (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_number VARCHAR(20) NOT NULL UNIQUE,
    session_id   UUID REFERENCES sessions(id) ON DELETE SET NULL,
    customer_id  UUID REFERENCES customers(id) ON DELETE SET NULL,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending', 'preparing', 'served', 'cancelled')),
    total_amount NUMERIC(10,2) NOT NULL DEFAULT 0,
    notes        TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_food_orders_session_id ON food_orders(session_id);
CREATE INDEX IF NOT EXISTS idx_food_orders_customer_id ON food_orders(customer_id);
CREATE INDEX IF NOT EXISTS idx_food_orders_status ON food_orders(status);

-- ============================================
-- 13. FOOD ORDER ITEMS
-- ============================================
CREATE TABLE IF NOT EXISTS food_order_items (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id     UUID NOT NULL REFERENCES food_orders(id) ON DELETE CASCADE,
    menu_item_id UUID NOT NULL REFERENCES menu_items(id) ON DELETE RESTRICT,
    quantity     INT NOT NULL CHECK (quantity > 0),
    unit_price   NUMERIC(10,2) NOT NULL CHECK (unit_price >= 0),
    subtotal     NUMERIC(10,2) NOT NULL CHECK (subtotal >= 0),
    notes        TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_food_order_items_order_id ON food_order_items(order_id);
CREATE INDEX IF NOT EXISTS idx_food_order_items_menu_item_id ON food_order_items(menu_item_id);

-- ============================================
-- 14. TV NOTIFICATIONS
-- ============================================
CREATE TABLE IF NOT EXISTS tv_notifications (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title               VARCHAR(100) NOT NULL,
    message             TEXT NOT NULL,
    target_all          BOOLEAN NOT NULL DEFAULT FALSE,
    target_console_ids  JSONB NOT NULL DEFAULT '[]',
    active_sessions_only BOOLEAN NOT NULL DEFAULT FALSE,
    loop_enabled        BOOLEAN NOT NULL DEFAULT FALSE,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    priority            VARCHAR(10) NOT NULL DEFAULT 'normal'
                            CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tv_notifications_active ON tv_notifications(is_active);
CREATE INDEX IF NOT EXISTS idx_tv_notifications_loop ON tv_notifications(loop_enabled, is_active);

-- ============================================
-- 15. TV ACTIVITY LOGS
-- ============================================
CREATE TABLE IF NOT EXISTS tv_activity_logs (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    console_id       UUID NOT NULL REFERENCES consoles(id),
    event            VARCHAR(20) NOT NULL CHECK (event IN ('on', 'off', 'sleep', 'screensaver')),
    session_id       UUID REFERENCES sessions(id),
    is_authorized    BOOLEAN NOT NULL DEFAULT FALSE,
    duration_minutes INT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tv_logs_console ON tv_activity_logs(console_id, created_at);
CREATE INDEX IF NOT EXISTS idx_tv_logs_session ON tv_activity_logs(session_id);

-- ============================================
-- 16. DAILY RENTALS
-- ============================================
CREATE TABLE IF NOT EXISTS daily_rentals (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    console_id      UUID NOT NULL REFERENCES consoles(id),
    customer_id     UUID NOT NULL REFERENCES customers(id),
    start_date      DATE NOT NULL,
    end_date        DATE NOT NULL,
    daily_price     NUMERIC(10, 2) NOT NULL DEFAULT 0,
    total_days      INT NOT NULL DEFAULT 1,
    free_days_used  INT NOT NULL DEFAULT 0,
    total_amount    NUMERIC(10, 2) NOT NULL DEFAULT 0,
    discount_amount NUMERIC(10, 2) NOT NULL DEFAULT 0,
    final_amount    NUMERIC(10, 2) NOT NULL DEFAULT 0,
    voucher_id      UUID REFERENCES vouchers(id),
    status          VARCHAR(20) NOT NULL DEFAULT 'active'
                        CHECK (status IN ('active', 'returned', 'completed', 'cancelled')),
    returned_at     TIMESTAMPTZ,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_daily_rentals_console ON daily_rentals(console_id);
CREATE INDEX IF NOT EXISTS idx_daily_rentals_date ON daily_rentals(start_date);
CREATE INDEX IF NOT EXISTS idx_daily_rentals_status ON daily_rentals(status);

-- ============================================
-- 17. BOOKINGS (reservasi)
-- ============================================
CREATE TABLE IF NOT EXISTS bookings (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    console_id       UUID NOT NULL REFERENCES consoles(id),
    customer_id      UUID NOT NULL REFERENCES customers(id),
    booking_date     DATE NOT NULL,
    start_hour       INT NOT NULL,
    start_minute     INT NOT NULL DEFAULT 0,
    duration_minutes INT NOT NULL,
    status           VARCHAR(20) NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending', 'confirmed', 'active', 'completed', 'cancelled', 'no_show')),
    notes            TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_bookings_console_date ON bookings(console_id, booking_date);
CREATE INDEX IF NOT EXISTS idx_bookings_status ON bookings(status);

-- ============================================
-- 18. APP SETTINGS
-- ============================================
CREATE TABLE IF NOT EXISTS app_settings (
    key         VARCHAR(100) PRIMARY KEY,
    value       TEXT NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO app_settings (key, value) VALUES ('membership_price', '50000') ON CONFLICT (key) DO NOTHING;
INSERT INTO app_settings (key, value) VALUES ('daily_price', '75000') ON CONFLICT (key) DO NOTHING;
