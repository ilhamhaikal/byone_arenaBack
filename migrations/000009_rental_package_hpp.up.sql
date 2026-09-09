-- ============================================
-- 000009: Bundling paket dicatat sebagai HPP (food_orders, tanpa pendapatan)
-- ============================================
-- Saat sesi paket dimulai, item bundling (kopi/risol/mie gratis) dicatat ke
-- food_orders dengan status 'served' dan total_amount=0, tetapi unit_hpp diisi
-- dari harga pokok menu. Dengan begitu laporan mencatat HPP bundling
-- (food_sales_hpp) TANPA menambah pendapatan makanan.
DROP FUNCTION IF EXISTS "__SP__StartSessionWithPackage"(UUID, UUID, UUID, NUMERIC, TEXT);

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
    v_order_id UUID;
    v_order_number VARCHAR(20);
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
    v_hpp NUMERIC(10,2);
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

    -- Konsumsi stok item bundling (resep bahan, FIFO) SEBELUM membuat sesi.
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

    -- Catat bundling sebagai HPP (tanpa pendapatan): food_order served, total 0,
    -- unit_hpp diisi harga pokok menu agar masuk food_sales_hpp di laporan.
    IF EXISTS (SELECT 1 FROM rental_package_items rpi WHERE rpi.package_id = p_package_id) THEN
        v_order_id := uuid_generate_v4();
        v_order_number := generate_food_order_number();
        INSERT INTO food_orders(id, order_number, session_id, customer_id, status,
                                total_amount, discount_amount, notes, created_at, updated_at)
        VALUES (v_order_id, v_order_number, v_session_id, p_customer_id, 'served',
                0, 0, 'Bundling paket ' || v_pkg.name || ' (gratis)', v_now, v_now);

        FOR v_item IN
            SELECT rpi.menu_item_id, rpi.quantity
            FROM rental_package_items rpi
            WHERE rpi.package_id = p_package_id
        LOOP
            SELECT COALESCE(hpp, 0) INTO v_hpp FROM menu_items WHERE id = v_item.menu_item_id;
            INSERT INTO food_order_items(id, order_id, menu_item_id, quantity,
                                         unit_price, unit_hpp, subtotal, notes, created_at)
            VALUES (uuid_generate_v4(), v_order_id, v_item.menu_item_id, v_item.quantity,
                    0, v_hpp, 0, 'Bundling paket ' || v_pkg.name, v_now);
        END LOOP;
    END IF;

    RETURN QUERY SELECT v_session_id, 'active'::VARCHAR, v_now, v_duration, v_end_scheduled,
        v_payment_id, v_pkg.price, p_cash_received, v_change, v_now, v_pkg.name;
END;
$$;
