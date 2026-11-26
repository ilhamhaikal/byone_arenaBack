-- Rollback 000057: hapus kategori bahan dan kembalikan IngredientHPP lama.

ALTER TABLE ingredients
    DROP CONSTRAINT IF EXISTS ingredients_category_check;
ALTER TABLE ingredients
    DROP COLUMN IF EXISTS category;

CREATE OR REPLACE FUNCTION "__SP__IngredientHPP"(
    p_ingredient_id UUID,
    p_depth INT DEFAULT 0
)
RETURNS NUMERIC
LANGUAGE plpgsql AS $$
DECLARE
    v_cost  NUMERIC(12,4) := 0;
    v_sub   RECORD;
    v_total NUMERIC(12,4) := 0;
    v_has_recipe BOOLEAN := FALSE;
BEGIN
    IF p_depth > 20 THEN
        SELECT COALESCE(cost_per_unit, 0) INTO v_cost
          FROM ingredients WHERE id = p_ingredient_id;
        RETURN COALESCE(v_cost, 0);
    END IF;

    SELECT EXISTS (
        SELECT 1 FROM ingredient_recipes WHERE ingredient_id = p_ingredient_id
    ) INTO v_has_recipe;

    IF v_has_recipe THEN
        FOR v_sub IN
            SELECT component_ingredient_id, quantity
              FROM ingredient_recipes
             WHERE ingredient_id = p_ingredient_id
        LOOP
            v_total := v_total + (v_sub.quantity * "__SP__IngredientHPP"(v_sub.component_ingredient_id, p_depth + 1));
        END LOOP;
        RETURN v_total;
    END IF;

    SELECT COALESCE(cost_per_unit, 0) INTO v_cost
      FROM ingredients WHERE id = p_ingredient_id;
    RETURN COALESCE(v_cost, 0);
END;
$$;