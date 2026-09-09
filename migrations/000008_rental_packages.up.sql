-- ============================================
-- 000008: Paket rental (begadang dll) + bundling item
-- ============================================
CREATE TABLE IF NOT EXISTS rental_packages (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(150) NOT NULL,
    start_time  TIME NOT NULL,
    end_time    TIME NOT NULL,
    price       NUMERIC(10,2) NOT NULL CHECK (price >= 0),
    description TEXT,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS rental_package_items (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    package_id   UUID NOT NULL REFERENCES rental_packages(id) ON DELETE CASCADE,
    menu_item_id UUID NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
    quantity     INT NOT NULL CHECK (quantity > 0),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (package_id, menu_item_id)
);

CREATE INDEX IF NOT EXISTS idx_rental_packages_active ON rental_packages(is_active);
CREATE INDEX IF NOT EXISTS idx_rental_package_items_package ON rental_package_items(package_id);

-- Durasi paket dalam menit (jam mulai->selesai, lintas tengah malam didukung)
CREATE OR REPLACE FUNCTION "__SP__PackageDurationMinutes"(p_start TIME, p_end TIME)
RETURNS INT LANGUAGE plpgsql AS $$
BEGIN
    IF p_end > p_start THEN
        RETURN EXTRACT(EPOCH FROM (p_end - p_start))::INT / 60;
    ELSE
        RETURN EXTRACT(EPOCH FROM (p_end + INTERVAL '24 hours' - p_start))::INT / 60;
    END IF;
END;
$$;

-- 1. Daftar paket + item bundling (jsonb)
CREATE OR REPLACE FUNCTION "__SP__ListRentalPackages"(p_include_inactive BOOLEAN DEFAULT FALSE)
RETURNS TABLE(id UUID, name VARCHAR, start_time TIME, end_time TIME, price NUMERIC,
              duration_minutes INT, description TEXT, is_active BOOLEAN,
              created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ, items JSONB)
LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT rp.id, rp.name, rp.start_time, rp.end_time, rp.price,
           "__SP__PackageDurationMinutes"(rp.start_time, rp.end_time) AS duration_minutes,
           rp.description, rp.is_active, rp.created_at, rp.updated_at,
           COALESCE((
               SELECT jsonb_agg(jsonb_build_object(
                          'menuItemId', rpi.menu_item_id,
                          'menuItemName', mi.name,
                          'quantity', rpi.quantity)
                      ORDER BY mi.name)
               FROM rental_package_items rpi
               JOIN menu_items mi ON mi.id = rpi.menu_item_id
               WHERE rpi.package_id = rp.id
           ), '[]'::JSONB) AS items
    FROM rental_packages rp
    WHERE p_include_inactive OR rp.is_active = TRUE
    ORDER BY rp.name ASC;
END;
$$;

-- 2. Simpan paket (upsert) + ganti item bundling-nya
CREATE OR REPLACE FUNCTION "__SP__SaveRentalPackage"(
    p_id UUID, p_name VARCHAR, p_start TIME, p_end TIME, p_price NUMERIC,
    p_description TEXT, p_is_active BOOLEAN, p_items JSONB
)
RETURNS UUID LANGUAGE plpgsql AS $$
DECLARE v_id UUID;
BEGIN
    IF p_id IS NULL OR NOT EXISTS (SELECT 1 FROM rental_packages WHERE id = p_id) THEN
        v_id := uuid_generate_v4();
        INSERT INTO rental_packages(id, name, start_time, end_time, price, description, is_active, created_at, updated_at)
        VALUES (v_id, p_name, p_start, p_end, p_price, p_description, COALESCE(p_is_active, TRUE), NOW(), NOW());
    ELSE
        v_id := p_id;
        UPDATE rental_packages
        SET name = p_name, start_time = p_start, end_time = p_end, price = p_price,
            description = p_description, is_active = COALESCE(p_is_active, TRUE), updated_at = NOW()
        WHERE id = v_id;
    END IF;

    DELETE FROM rental_package_items WHERE package_id = v_id;
    INSERT INTO rental_package_items(id, package_id, menu_item_id, quantity)
    SELECT uuid_generate_v4(), v_id, (it->>'menuItemId')::UUID, (it->>'quantity')::INT
    FROM jsonb_array_elements(COALESCE(p_items, '[]'::JSONB)) it
    WHERE it->>'menuItemId' IS NOT NULL AND (it->>'quantity')::INT > 0;

    RETURN v_id;
END;
$$;

-- 3. Hapus paket
CREATE OR REPLACE FUNCTION "__SP__DeleteRentalPackage"(p_id UUID)
RETURNS VOID LANGUAGE plpgsql AS $$
BEGIN
    DELETE FROM rental_packages WHERE id = p_id;
    IF NOT FOUND THEN RAISE EXCEPTION 'PACKAGE_NOT_FOUND'; END IF;
END;
$$;

-- 4. Mulai sesi dengan paket (harga flat paket + konsumsi stok item bundling)
CREATE OR REPLACE FUNCTION "__SP__StartSessionWithPackage"(
    p_console_id UUID, p_customer_id UUID, p_package_id UUID,
    p_cash_received NUMERIC, p_notes TEXT
)
RETURNS TABLE(
    session_id UUID, session_status VARCHAR, session_start_time TIMESTAMPTZ,
    session_booked_minutes INT, session_end_scheduled TIMESTAMPTZ,
    payment_id UUID, total_payment NUMERIC, cash_received NUMERIC, change_amount NUMERIC,
    paid_at TIMESTAMPTZ, package_name VARCHAR
)
LANGUAGE plpgsql AS $$
DECLARE
    v_session_id UUID;
    v_payment_id UUID;
    v_console_status VARCHAR;
    v_pkg rental_packages%ROWTYPE;
    v_duration INT;
    v_now TIMESTAMPTZ := NOW();
    v_end_scheduled TIMESTAMPTZ;
    v_change NUMERIC(10,2);
    v_item RECORD;
    v_req RECORD;
    v_usable NUMERIC(10,3);
    v_need NUMERIC(10,3);
    v_ing_name VARCHAR;
    v_batch RECORD;
BEGIN
    SELECT * INTO v_pkg FROM rental_packages WHERE id = p_package_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'PACKAGE_NOT_FOUND'; END IF;
    IF NOT v_pkg.is_active THEN RAISE EXCEPTION 'PACKAGE_INACTIVE'; END IF;

    SELECT c.status INTO v_console_status FROM consoles c WHERE c.id = p_console_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'CONSOLE_NOT_FOUND'; END IF;
    IF v_console_status != 'available' THEN RAISE EXCEPTION 'CONSOLE_NOT_AVAILABLE: %', v_console_status; END IF;
    IF EXISTS (SELECT 1 FROM sessions s2 WHERE s2.console_id = p_console_id AND s2.status = 'active') THEN
        RAISE EXCEPTION 'SESSION_ALREADY_ACTIVE';
    END IF;

    IF p_cash_received < v_pkg.price THEN
        RAISE EXCEPTION 'INSUFFICIENT_CASH: butuh Rp %.0f', v_pkg.price;
    END IF;

    v_duration := "__SP__PackageDurationMinutes"(v_pkg.start_time, v_pkg.end_time);
    v_end_scheduled := v_now + (v_duration * INTERVAL '1 minute');
    v_change := GREATEST(p_cash_received - v_pkg.price, 0);

    -- Konsumsi stok item bundling (resep bahan, FIFO) SEBELUM membuat sesi,
    -- supaya kalau stok kurang, seluruh transaksi batal (atomik).
    FOR v_item IN
        SELECT rpi.menu_item_id, rpi.quantity
        FROM rental_package_items rpi
        WHERE rpi.package_id = p_package_id
    LOOP
        IF NOT EXISTS (SELECT 1 FROM menu_item_ingredients mii WHERE mii.menu_item_id = v_item.menu_item_id) THEN
            RAISE EXCEPTION 'MENU_NO_RECIPE: item paket %', v_item.menu_item_id;
        END IF;

        FOR v_req IN
            WITH RECURSIVE req(ingredient_id, qty, depth) AS (
                SELECT mii.ingredient_id, v_item.quantity::NUMERIC * mii.quantity, 1
                FROM menu_item_ingredients mii
                WHERE mii.menu_item_id = v_item.menu_item_id
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

            v_need := v_req.total_qty;
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

    v_session_id := uuid_generate_v4();
    INSERT INTO sessions(id, console_id, customer_id, start_time, booked_duration_minutes,
                         end_scheduled_at, total_price, status, notes,
                         paid_duration_minutes, created_at, updated_at)
    VALUES (v_session_id, p_console_id, p_customer_id, v_now, v_duration,
            v_end_scheduled, v_pkg.price, 'active', COALESCE(p_notes, v_pkg.name),
            v_duration, v_now, v_now);
    UPDATE consoles SET status = 'in_use', updated_at = v_now WHERE consoles.id = p_console_id;
    INSERT INTO tv_activity_logs(id, console_id, event, session_id, is_authorized, created_at)
    VALUES (uuid_generate_v4(), p_console_id, 'on', v_session_id, TRUE, v_now);

    v_payment_id := uuid_generate_v4();
    INSERT INTO payments(id, session_id, amount, discount_amount, auto_discount_amount,
                         total_payment, payment_method, payment_status, cash_received,
                         change_amount, notes, paid_at, created_at, updated_at)
    VALUES (v_payment_id, v_session_id, v_pkg.price, 0, 0, v_pkg.price, 'cash', 'paid',
            p_cash_received, v_change, p_notes, v_now, v_now, v_now);

    RETURN QUERY SELECT v_session_id, 'active'::VARCHAR, v_now, v_duration, v_end_scheduled,
        v_payment_id, v_pkg.price, p_cash_received, v_change, v_now, v_pkg.name;
END;
$$;
