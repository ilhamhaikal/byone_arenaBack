-- Kembalikan CreateFoodOrder versi sebelum diskon dihitung di server.
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

        -- Menu WAJIB punya resep (bahan)
        IF NOT EXISTS (SELECT 1 FROM menu_item_ingredients WHERE menu_item_id = v_menu_item_id) THEN
            RAISE EXCEPTION 'MENU_NO_RECIPE: %',v_menu_item_id;
        END IF;

        v_subtotal:=v_price*v_quantity;
        v_total:=v_total+v_subtotal;

        INSERT INTO food_order_items(id,order_id,menu_item_id,quantity,unit_price,unit_hpp,subtotal,notes,created_at)
        VALUES(uuid_generate_v4(),v_order_id,v_menu_item_id,v_quantity,v_price,v_hpp,v_subtotal,v_item_notes,NOW());

        -- Konsumsi stok BAHAN (resep bertingkat)
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
