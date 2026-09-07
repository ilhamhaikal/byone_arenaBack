-- =============================================================================
-- 000004_batch_stock_expiry.up.sql
-- Batch stock + tanggal kadaluarsa + log penambahan stok.
--
-- Konsep:
--   - Setiap penambahan stok = 1 batch (quantity, remaining, expires_at, added_at).
--     added_at otomatis = tanggal penambahan (sekaligus menjadi log bertanggal).
--   - Usable stock = SUM(remaining) yang belum kadaluarsa (expires_at >= hari ini).
--   - Konsumsi FIFO: batch dengan expires_at paling dekat dikonsumsi lebih dulu.
--   - Berlaku untuk bahan (ingredients) DAN menu items.
-- =============================================================================

-- =============================================================================
-- 1. BAHAN (ingredients)
-- =============================================================================
CREATE TABLE IF NOT EXISTS ingredients (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name       VARCHAR(150) NOT NULL,
    unit       VARCHAR(20) NOT NULL DEFAULT 'pcs',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================================================
-- 2. Batch stok BAHAN (log penambahan + kadaluarsa)
-- =============================================================================
CREATE TABLE IF NOT EXISTS ingredient_stock_batches (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ingredient_id UUID NOT NULL REFERENCES ingredients(id) ON DELETE CASCADE,
    quantity      NUMERIC(10,3) NOT NULL CHECK (quantity > 0),
    remaining     NUMERIC(10,3) NOT NULL DEFAULT 0 CHECK (remaining >= 0),
    expires_at    DATE NOT NULL,
    added_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    notes         TEXT
);

CREATE INDEX IF NOT EXISTS idx_ingredient_batches_ingredient ON ingredient_stock_batches(ingredient_id);
CREATE INDEX IF NOT EXISTS idx_ingredient_batches_expires ON ingredient_stock_batches(expires_at);

-- =============================================================================
-- 3. Batch stok MENU ITEM (log penambahan + kadaluarsa)
-- =============================================================================
CREATE TABLE IF NOT EXISTS menu_item_stock_batches (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    menu_item_id UUID NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
    quantity     NUMERIC(10,3) NOT NULL CHECK (quantity > 0),
    remaining    NUMERIC(10,3) NOT NULL DEFAULT 0 CHECK (remaining >= 0),
    expires_at   DATE NOT NULL,
    added_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    notes        TEXT
);

CREATE INDEX IF NOT EXISTS idx_menu_batches_menu ON menu_item_stock_batches(menu_item_id);
CREATE INDEX IF NOT EXISTS idx_menu_batches_expires ON menu_item_stock_batches(expires_at);

-- =============================================================================
-- 4. Resep menu -> bahan (BOM langsung)
-- =============================================================================
CREATE TABLE IF NOT EXISTS menu_item_ingredients (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    menu_item_id  UUID NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
    ingredient_id UUID NOT NULL REFERENCES ingredients(id) ON DELETE CASCADE,
    quantity      NUMERIC(10,3) NOT NULL CHECK (quantity > 0),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (menu_item_id, ingredient_id)
);

CREATE INDEX IF NOT EXISTS idx_mii_menu ON menu_item_ingredients(menu_item_id);

-- =============================================================================
-- 5. Sub-resep bahan -> bahan (BOM bertingkat)
-- =============================================================================
CREATE TABLE IF NOT EXISTS ingredient_recipes (
    id                     UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ingredient_id          UUID NOT NULL REFERENCES ingredients(id) ON DELETE CASCADE,
    component_ingredient_id UUID NOT NULL REFERENCES ingredients(id) ON DELETE CASCADE,
    quantity               NUMERIC(10,3) NOT NULL CHECK (quantity > 0),
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (ingredient_id <> component_ingredient_id),
    UNIQUE (ingredient_id, component_ingredient_id)
);

CREATE INDEX IF NOT EXISTS idx_ir_ingredient ON ingredient_recipes(ingredient_id);

-- =============================================================================
-- 6. Migrasi stok menu lama -> batch (expiry default 1 tahun, bisa diubah admin)
-- =============================================================================
INSERT INTO menu_item_stock_batches (id, menu_item_id, quantity, remaining, expires_at, added_at, notes)
SELECT uuid_generate_v4(), id, stock, stock, CURRENT_DATE + 365, NOW(), 'migrasi stok lama'
FROM menu_items
WHERE stock > 0;

-- Hapus kolom stock lama (diganti batch)
ALTER TABLE menu_items DROP COLUMN IF EXISTS stock;

-- =============================================================================
-- 7. CreateFoodOrder — konsumsi stok menu & bahan (FIFO, anti-kadaluarsa)
-- =============================================================================
CREATE OR REPLACE FUNCTION "__SP__CreateFoodOrder"(p_session_id UUID, p_customer_id UUID, p_notes TEXT, p_items JSONB)
RETURNS TABLE(order_id UUID, order_number VARCHAR, total_amount NUMERIC)
LANGUAGE plpgsql AS $$
DECLARE
    v_order_id UUID;
    v_order_number VARCHAR(20);
    v_total NUMERIC(10,2):=0;
    v_item JSONB;
    v_menu_item_id UUID;
    v_quantity INT;
    v_item_notes TEXT;
    v_price NUMERIC(10,2);
    v_hpp NUMERIC(10,2);
    v_subtotal NUMERIC(10,2);
    v_available BOOLEAN;
    v_usable NUMERIC(10,3);
    v_need NUMERIC(10,3);
    v_ing_name VARCHAR;
    v_batch RECORD;
    v_req RECORD;
BEGIN
    IF p_items IS NULL OR jsonb_array_length(p_items)=0 THEN
        RAISE EXCEPTION 'ORDER_EMPTY';
    END IF;

    v_order_number:=generate_food_order_number();
    v_order_id:=uuid_generate_v4();

    INSERT INTO food_orders(id,order_number,session_id,customer_id,status,total_amount,notes,created_at,updated_at)
    VALUES(v_order_id,v_order_number,p_session_id,p_customer_id,'pending',0,p_notes,NOW(),NOW());

    FOR v_item IN SELECT * FROM jsonb_array_elements(p_items) LOOP
        v_menu_item_id:=(v_item->>'menu_item_id')::UUID;
        v_quantity:=(v_item->>'quantity')::INT;
        v_item_notes:=v_item->>'notes';

        IF v_quantity<=0 THEN
            RAISE EXCEPTION 'INVALID_QUANTITY';
        END IF;

        SELECT price, hpp, is_available
        INTO v_price, v_hpp, v_available
        FROM menu_items WHERE id=v_menu_item_id FOR UPDATE;

        IF NOT FOUND THEN
            RAISE EXCEPTION 'MENU_NOT_FOUND: %',v_menu_item_id;
        END IF;
        IF NOT v_available THEN
            RAISE EXCEPTION 'MENU_UNAVAILABLE';
        END IF;

        -- Cek stok menu (usable, belum kadaluarsa)
        SELECT COALESCE(SUM(remaining),0) INTO v_usable
        FROM menu_item_stock_batches
        WHERE menu_item_id = v_menu_item_id AND remaining > 0 AND expires_at >= CURRENT_DATE;

        IF v_usable < v_quantity THEN
            RAISE EXCEPTION 'INSUFFICIENT_STOCK: %', v_menu_item_id;
        END IF;

        v_subtotal:=v_price*v_quantity;
        v_total:=v_total+v_subtotal;

        INSERT INTO food_order_items(id,order_id,menu_item_id,quantity,unit_price,unit_hpp,subtotal,notes,created_at)
        VALUES(uuid_generate_v4(),v_order_id,v_menu_item_id,v_quantity,v_price,v_hpp,v_subtotal,v_item_notes,NOW());

        -- Kurangi stok menu FIFO
        v_need:=v_quantity;
        FOR v_batch IN
            SELECT id, remaining FROM menu_item_stock_batches
            WHERE menu_item_id = v_menu_item_id AND remaining > 0 AND expires_at >= CURRENT_DATE
            ORDER BY expires_at ASC, added_at ASC
            FOR UPDATE
        LOOP
            IF v_need <= 0 THEN EXIT; END IF;
            IF v_batch.remaining >= v_need THEN
                UPDATE menu_item_stock_batches SET remaining = remaining - v_need WHERE id = v_batch.id;
                v_need := 0;
            ELSE
                v_need := v_need - v_batch.remaining;
                UPDATE menu_item_stock_batches SET remaining = 0 WHERE id = v_batch.id;
            END IF;
        END LOOP;

        -- Bahan (resep bertingkat) untuk item ini
        FOR v_req IN
            WITH RECURSIVE req(ingredient_id, qty, depth) AS (
                SELECT mi.ingredient_id, v_quantity::NUMERIC * mi.quantity, 1
                FROM menu_item_ingredients mi
                WHERE mi.menu_item_id = v_menu_item_id
                UNION ALL
                SELECT ir.component_ingredient_id, r.qty * ir.quantity, r.depth + 1
                FROM req r
                JOIN ingredient_recipes ir ON ir.ingredient_id = r.ingredient_id
                WHERE r.depth < 20
            )
            SELECT ingredient_id, SUM(qty) AS total_qty
            FROM req GROUP BY ingredient_id
        LOOP
            SELECT COALESCE(SUM(remaining),0) INTO v_usable
            FROM ingredient_stock_batches
            WHERE ingredient_id = v_req.ingredient_id AND remaining > 0 AND expires_at >= CURRENT_DATE;

            IF v_usable < v_req.total_qty THEN
                SELECT name INTO v_ing_name FROM ingredients WHERE id = v_req.ingredient_id;
                RAISE EXCEPTION 'INSUFFICIENT_STOCK: bahan %', COALESCE(v_ing_name, v_req.ingredient_id::TEXT);
            END IF;

            v_need:=v_req.total_qty;
            FOR v_batch IN
                SELECT id, remaining FROM ingredient_stock_batches
                WHERE ingredient_id = v_req.ingredient_id AND remaining > 0 AND expires_at >= CURRENT_DATE
                ORDER BY expires_at ASC, added_at ASC
                FOR UPDATE
            LOOP
                IF v_need <= 0 THEN EXIT; END IF;
                IF v_batch.remaining >= v_need THEN
                    UPDATE ingredient_stock_batches SET remaining = remaining - v_need WHERE id = v_batch.id;
                    v_need := 0;
                ELSE
                    v_need := v_need - v_batch.remaining;
                    UPDATE ingredient_stock_batches SET remaining = 0 WHERE id = v_batch.id;
                END IF;
            END LOOP;
        END LOOP;
    END LOOP;

    UPDATE food_orders SET total_amount=v_total,updated_at=NOW() WHERE id=v_order_id;
    RETURN QUERY SELECT v_order_id,v_order_number,v_total;
END;
$$;

-- =============================================================================
-- 8. UpdateFoodOrderStatus — kembalikan stok menu & bahan saat dibatalkan
-- =============================================================================
CREATE OR REPLACE FUNCTION "__SP__UpdateFoodOrderStatus"(p_order_id UUID, p_status VARCHAR)
RETURNS VOID LANGUAGE plpgsql AS $$
DECLARE
    v_current_status VARCHAR;
    v_item RECORD;
    v_batch RECORD;
    v_need NUMERIC(10,3);
    v_req RECORD;
    v_menu_qty INT;
BEGIN
    SELECT status INTO v_current_status FROM food_orders WHERE id=p_order_id FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'ORDER_NOT_FOUND';
    END IF;
    IF v_current_status='cancelled' THEN
        RAISE EXCEPTION 'ORDER_CANCELLED';
    END IF;
    IF v_current_status='served' AND p_status!='cancelled' THEN
        RAISE EXCEPTION 'ORDER_SERVED';
    END IF;

    IF p_status='cancelled' AND v_current_status IN ('pending','preparing') THEN
        FOR v_item IN SELECT menu_item_id, quantity FROM food_order_items WHERE order_id=p_order_id LOOP
            v_menu_qty := v_item.quantity;

            -- Kembalikan stok menu ke batch terakhir (approx)
            SELECT id INTO v_batch FROM menu_item_stock_batches
            WHERE menu_item_id = v_item.menu_item_id
            ORDER BY expires_at DESC, added_at DESC
            LIMIT 1 FOR UPDATE;
            IF FOUND THEN
                UPDATE menu_item_stock_batches SET remaining = remaining + v_menu_qty WHERE id = v_batch.id;
            ELSE
                INSERT INTO menu_item_stock_batches(id, menu_item_id, quantity, remaining, expires_at, added_at, notes)
                VALUES(uuid_generate_v4(), v_item.menu_item_id, v_menu_qty, v_menu_qty, CURRENT_DATE + 365, NOW(), 'restore cancel');
            END IF;

            -- Kembalikan stok bahan
            FOR v_req IN
                WITH RECURSIVE req(ingredient_id, qty, depth) AS (
                    SELECT mi.ingredient_id, v_menu_qty::NUMERIC * mi.quantity, 1
                    FROM menu_item_ingredients mi
                    WHERE mi.menu_item_id = v_item.menu_item_id
                    UNION ALL
                    SELECT ir.component_ingredient_id, r.qty * ir.quantity, r.depth + 1
                    FROM req r
                    JOIN ingredient_recipes ir ON ir.ingredient_id = r.ingredient_id
                    WHERE r.depth < 20
                )
                SELECT ingredient_id, SUM(qty) AS total_qty
                FROM req GROUP BY ingredient_id
            LOOP
                v_need := v_req.total_qty;
                SELECT id INTO v_batch FROM ingredient_stock_batches
                WHERE ingredient_id = v_req.ingredient_id
                ORDER BY expires_at DESC, added_at DESC
                LIMIT 1 FOR UPDATE;
                IF FOUND THEN
                    UPDATE ingredient_stock_batches SET remaining = remaining + v_need WHERE id = v_batch.id;
                ELSE
                    INSERT INTO ingredient_stock_batches(id, ingredient_id, quantity, remaining, expires_at, added_at, notes)
                    VALUES(uuid_generate_v4(), v_req.ingredient_id, v_need, v_need, CURRENT_DATE + 365, NOW(), 'restore cancel');
                END IF;
            END LOOP;
        END LOOP;
    END IF;

    UPDATE food_orders SET status=p_status,updated_at=NOW() WHERE id=p_order_id;
END;
$$;
