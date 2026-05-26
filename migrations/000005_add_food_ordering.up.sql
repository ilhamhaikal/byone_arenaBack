-- Migration: 000005_add_food_ordering.up.sql
-- Sistem pemesanan makanan & minuman
-- Admin membuat menu, lalu mencatat pesanan pelanggan (bisa terhubung ke sesi PS)

-- =============================================
-- Tabel: menu_items
-- Daftar makanan/minuman yang tersedia
-- =============================================
CREATE TABLE IF NOT EXISTS menu_items (
    id           UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),
    name         VARCHAR(150)  NOT NULL,
    category     VARCHAR(30)   NOT NULL DEFAULT 'food'
                     CHECK (category IN ('food', 'drink', 'snack', 'other')),
    price        NUMERIC(10,2) NOT NULL CHECK (price >= 0),
    description  TEXT          NULL,
    is_available BOOLEAN       NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_menu_items_category     ON menu_items(category);
CREATE INDEX IF NOT EXISTS idx_menu_items_is_available ON menu_items(is_available);

-- =============================================
-- Tabel: food_orders
-- Header pesanan makanan (1 pesanan bisa berisi banyak item)
-- =============================================
CREATE TABLE IF NOT EXISTS food_orders (
    id           UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_number VARCHAR(20)   NOT NULL UNIQUE,         -- nomor pesanan, contoh: F-20240526-001
    session_id   UUID          NULL REFERENCES sessions(id)  ON DELETE SET NULL,   -- opsional, jika pelanggan sedang main
    customer_id  UUID          NULL REFERENCES customers(id) ON DELETE SET NULL,   -- opsional, bisa walk-in
    status       VARCHAR(20)   NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending', 'preparing', 'served', 'cancelled')),
    total_amount NUMERIC(10,2) NOT NULL DEFAULT 0,
    notes        TEXT          NULL,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_food_orders_session_id  ON food_orders(session_id);
CREATE INDEX IF NOT EXISTS idx_food_orders_customer_id ON food_orders(customer_id);
CREATE INDEX IF NOT EXISTS idx_food_orders_status      ON food_orders(status);
CREATE INDEX IF NOT EXISTS idx_food_orders_created_at  ON food_orders(created_at DESC);

-- =============================================
-- Tabel: food_order_items
-- Detail item dalam satu pesanan makanan
-- =============================================
CREATE TABLE IF NOT EXISTS food_order_items (
    id           UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id     UUID          NOT NULL REFERENCES food_orders(id) ON DELETE CASCADE,
    menu_item_id UUID          NOT NULL REFERENCES menu_items(id)  ON DELETE RESTRICT,
    quantity     INT           NOT NULL CHECK (quantity > 0),
    unit_price   NUMERIC(10,2) NOT NULL CHECK (unit_price >= 0),  -- harga saat dipesan (snapshot)
    subtotal     NUMERIC(10,2) NOT NULL CHECK (subtotal >= 0),    -- quantity × unit_price
    notes        TEXT          NULL,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_food_order_items_order_id     ON food_order_items(order_id);
CREATE INDEX IF NOT EXISTS idx_food_order_items_menu_item_id ON food_order_items(menu_item_id);

-- =============================================
-- SEQUENCE: nomor urut pesanan harian
-- =============================================
CREATE SEQUENCE IF NOT EXISTS food_order_daily_seq START 1;

-- =============================================
-- FUNCTION: generate_order_number
-- Menghasilkan nomor pesanan format: F-YYYYMMDD-NNN
-- =============================================
CREATE OR REPLACE FUNCTION generate_food_order_number()
RETURNS VARCHAR
LANGUAGE plpgsql
AS $$
DECLARE
    v_date   VARCHAR(8);
    v_seq    INT;
    v_number VARCHAR(20);
BEGIN
    v_date := TO_CHAR(NOW() AT TIME ZONE 'Asia/Jakarta', 'YYYYMMDD');
    -- Reset sequence jika hari berganti (perbandingan sederhana via tanggal di nomor)
    SELECT COALESCE(MAX(CAST(SPLIT_PART(order_number, '-', 3) AS INT)), 0) + 1
    INTO v_seq
    FROM food_orders
    WHERE order_number LIKE 'F-' || v_date || '-%';

    v_number := 'F-' || v_date || '-' || LPAD(v_seq::TEXT, 3, '0');
    RETURN v_number;
END;
$$;

-- =============================================
-- FUNCTION: sp_create_food_order
-- Membuat pesanan makanan secara atomik
-- Input items dalam format JSON array: [{"menu_item_id":"uuid","quantity":2,"notes":"..."}]
-- =============================================
CREATE OR REPLACE FUNCTION sp_create_food_order(
    p_session_id  UUID,
    p_customer_id UUID,
    p_notes       TEXT,
    p_items       JSONB  -- array: [{"menu_item_id":"uuid","quantity":int,"notes":"text"}]
)
RETURNS TABLE (
    order_id     UUID,
    order_number VARCHAR,
    total_amount NUMERIC
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_order_id     UUID;
    v_order_number VARCHAR(20);
    v_total        NUMERIC(10,2) := 0;
    v_item         JSONB;
    v_menu_item_id UUID;
    v_quantity     INT;
    v_item_notes   TEXT;
    v_price        NUMERIC(10,2);
    v_subtotal     NUMERIC(10,2);
    v_available    BOOLEAN;
BEGIN
    -- Validasi array item tidak kosong
    IF p_items IS NULL OR jsonb_array_length(p_items) = 0 THEN
        RAISE EXCEPTION 'ORDER_EMPTY: Pesanan harus memiliki minimal 1 item';
    END IF;

    -- Generate nomor pesanan
    v_order_number := generate_food_order_number();
    v_order_id     := uuid_generate_v4();

    -- Buat header pesanan
    INSERT INTO food_orders (id, order_number, session_id, customer_id, status, total_amount, notes, created_at, updated_at)
    VALUES (v_order_id, v_order_number, p_session_id, p_customer_id, 'pending', 0, p_notes, NOW(), NOW());

    -- Proses setiap item
    FOR v_item IN SELECT * FROM jsonb_array_elements(p_items)
    LOOP
        v_menu_item_id := (v_item->>'menu_item_id')::UUID;
        v_quantity     := (v_item->>'quantity')::INT;
        v_item_notes   := v_item->>'notes';

        IF v_quantity <= 0 THEN
            RAISE EXCEPTION 'INVALID_QUANTITY: Jumlah item harus lebih dari 0';
        END IF;

        -- Ambil harga dan status menu item (snapshot harga saat pesan)
        SELECT price, is_available INTO v_price, v_available
        FROM menu_items WHERE id = v_menu_item_id;

        IF NOT FOUND THEN
            RAISE EXCEPTION 'MENU_NOT_FOUND: Menu item tidak ditemukan (id: %)', v_menu_item_id;
        END IF;

        IF NOT v_available THEN
            RAISE EXCEPTION 'MENU_UNAVAILABLE: Menu item tidak tersedia saat ini';
        END IF;

        v_subtotal := v_price * v_quantity;
        v_total    := v_total + v_subtotal;

        INSERT INTO food_order_items (id, order_id, menu_item_id, quantity, unit_price, subtotal, notes, created_at)
        VALUES (uuid_generate_v4(), v_order_id, v_menu_item_id, v_quantity, v_price, v_subtotal, v_item_notes, NOW());
    END LOOP;

    -- Update total pesanan
    UPDATE food_orders SET total_amount = v_total, updated_at = NOW() WHERE id = v_order_id;

    RETURN QUERY SELECT v_order_id, v_order_number, v_total;
END;
$$;

-- =============================================
-- FUNCTION: sp_update_food_order_status
-- Update status pesanan dengan validasi transisi
-- =============================================
CREATE OR REPLACE FUNCTION sp_update_food_order_status(
    p_order_id  UUID,
    p_status    VARCHAR
)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    v_current_status VARCHAR;
BEGIN
    SELECT status INTO v_current_status FROM food_orders WHERE id = p_order_id FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'ORDER_NOT_FOUND: Pesanan tidak ditemukan';
    END IF;

    IF v_current_status = 'cancelled' THEN
        RAISE EXCEPTION 'ORDER_CANCELLED: Pesanan yang sudah dibatalkan tidak bisa diubah statusnya';
    END IF;

    IF v_current_status = 'served' AND p_status != 'cancelled' THEN
        RAISE EXCEPTION 'ORDER_SERVED: Pesanan yang sudah selesai tidak bisa diubah statusnya';
    END IF;

    UPDATE food_orders
    SET status = p_status, updated_at = NOW()
    WHERE id = p_order_id;
END;
$$;
