-- Rollback 000058: hapus scope diskon, kembalikan evaluator dan CreateFoodOrder.

DROP FUNCTION IF EXISTS "__SP__EvaluateDiscountRules"(NUMERIC, TIMESTAMPTZ, BOOLEAN, VARCHAR);

CREATE OR REPLACE FUNCTION "__SP__EvaluateDiscountRules"(p_total_price NUMERIC, p_session_start_time TIMESTAMPTZ, p_is_member BOOLEAN DEFAULT FALSE)
RETURNS NUMERIC LANGUAGE plpgsql AS $$
DECLARE v_dow INT; v_hour INT; v_total_disc NUMERIC(10,2):=0; v_disc NUMERIC(10,2); r RECORD;
BEGIN
    v_dow:=EXTRACT(DOW FROM p_session_start_time AT TIME ZONE 'Asia/Jakarta')::INT;
    v_hour:=EXTRACT(HOUR FROM p_session_start_time AT TIME ZONE 'Asia/Jakarta')::INT;
    FOR r IN SELECT * FROM discount_rules WHERE is_active=TRUE ORDER BY priority DESC, created_at ASC LOOP
        CASE r.rule_type
            WHEN 'always' THEN NULL;
            WHEN 'happy_hour' THEN
                IF r.start_hour IS NULL OR r.end_hour IS NULL THEN CONTINUE; END IF;
                IF r.start_hour>r.end_hour THEN
                    IF NOT (v_hour>=r.start_hour OR v_hour<r.end_hour) THEN CONTINUE; END IF;
                ELSE
                    IF NOT (v_hour>=r.start_hour AND v_hour<r.end_hour) THEN CONTINUE; END IF;
                END IF;
            WHEN 'member' THEN IF NOT p_is_member THEN CONTINUE; END IF;
            WHEN 'day_of_week' THEN
                IF r.days_of_week IS NULL OR TRIM(r.days_of_week)='' THEN CONTINUE; END IF;
                IF NOT (v_dow=ANY(string_to_array(r.days_of_week,',')::INT[])) THEN CONTINUE; END IF;
            ELSE CONTINUE;
        END CASE;
        IF p_total_price<r.min_purchase THEN CONTINUE; END IF;
        IF r.discount_type='percentage' THEN
            v_disc:=ROUND((p_total_price*r.discount_value/100)::NUMERIC,2);
            IF r.max_discount>0 AND v_disc>r.max_discount THEN v_disc:=r.max_discount; END IF;
        ELSE
            v_disc:=r.discount_value;
            IF v_disc>p_total_price THEN v_disc:=p_total_price; END IF;
        END IF;
        v_total_disc:=v_total_disc+v_disc;
    END LOOP;
    IF v_total_disc>p_total_price THEN v_total_disc:=p_total_price; END IF;
    RETURN v_total_disc;
END;
$$;

CREATE OR REPLACE FUNCTION "__SP__CreateFoodOrder"(p_session_id UUID, p_customer_id UUID, p_notes TEXT, p_items JSONB)
RETURNS TABLE(order_id UUID, order_number VARCHAR, total_amount NUMERIC)
LANGUAGE plpgsql AS $$
DECLARE
    v_order_id UUID;
    v_order_number VARCHAR(20);
    v_total NUMERIC(10,2):=0;
    v_discount NUMERIC(10,2):=0;
    v_customer_id UUID;
    v_is_member BOOLEAN:=FALSE;
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

    v_customer_id := p_customer_id;
    IF v_customer_id IS NULL AND p_session_id IS NOT NULL THEN
        SELECT customer_id INTO v_customer_id FROM sessions WHERE id=p_session_id;
    END IF;
    IF v_customer_id IS NOT NULL THEN
        SELECT COALESCE(is_member,FALSE) INTO v_is_member
        FROM customers WHERE id=v_customer_id;
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

        IF NOT EXISTS (SELECT 1 FROM menu_item_ingredients WHERE menu_item_id = v_menu_item_id) THEN
            RAISE EXCEPTION 'MENU_NO_RECIPE: %',v_menu_item_id;
        END IF;

        v_subtotal:=v_price*v_quantity;
        v_total:=v_total+v_subtotal;

        INSERT INTO food_order_items(id,order_id,menu_item_id,quantity,unit_price,unit_hpp,subtotal,notes,created_at)
        VALUES(uuid_generate_v4(),v_order_id,v_menu_item_id,v_quantity,v_price,v_hpp,v_subtotal,v_item_notes,NOW());

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

    v_discount := COALESCE("__SP__EvaluateDiscountRules"(v_total, NOW(), v_is_member), 0);
    IF v_discount < 0 THEN
        v_discount := 0;
    END IF;
    IF v_discount > v_total THEN
        v_discount := v_total;
    END IF;

    UPDATE food_orders
    SET total_amount=v_total, discount_amount=v_discount, updated_at=NOW()
    WHERE id=v_order_id;

    RETURN QUERY SELECT v_order_id,v_order_number,v_total;
END;
$$;
