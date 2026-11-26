-- =============================================================================
-- 000057_ingredient_category.up.sql
--
-- Kategori bahan:
--   food  = bahan makanan (pcs/gram/kg)
--   drink = bahan minuman (pcs/gram/ml)
--   spice = bumbu (dibeli per gram/ons, dipakai lintas menu)
--
-- Bumbu tetap punya stok dan tetap dikonsumsi saat order, tetapi TIDAK
-- dihitung ke HPP menu. HPP menu hanya dari bahan food/drink.
-- =============================================================================

ALTER TABLE ingredients
    ADD COLUMN IF NOT EXISTS category VARCHAR(20) NOT NULL DEFAULT 'food';

ALTER TABLE ingredients
    DROP CONSTRAINT IF EXISTS ingredients_category_check;

ALTER TABLE ingredients
    ADD CONSTRAINT ingredients_category_check
    CHECK (category IN ('food', 'drink', 'spice'));

CREATE OR REPLACE FUNCTION "__SP__IngredientHPP"(
    p_ingredient_id UUID,
    p_depth INT DEFAULT 0
)
RETURNS NUMERIC
LANGUAGE plpgsql AS $$
DECLARE
    v_cost      NUMERIC(12,4) := 0;
    v_category  VARCHAR(20) := 'food';
    v_sub       RECORD;
    v_total     NUMERIC(12,4) := 0;
    v_has_recipe BOOLEAN := FALSE;
BEGIN
    SELECT COALESCE(cost_per_unit, 0), COALESCE(category, 'food')
      INTO v_cost, v_category
      FROM ingredients WHERE id = p_ingredient_id;

    -- Bumbu tidak dihitung sebagai HPP menu.
    IF v_category = 'spice' THEN
        RETURN 0;
    END IF;

    IF p_depth > 20 THEN
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

    RETURN COALESCE(v_cost, 0);
END;
$$;