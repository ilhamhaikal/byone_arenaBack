-- =============================================================================
-- 000059_ingredient_category_recalc.up.sql
--
-- Recalculate HPP menu ketika kategori bahan berubah (mis. bahan diubah
-- menjadi 'spice' agar tidak dihitung ke HPP).
-- =============================================================================

DROP TRIGGER IF EXISTS trg_ingredient_cost_recalc ON ingredients;

CREATE TRIGGER trg_ingredient_cost_recalc
    AFTER UPDATE OF cost_per_unit, category ON ingredients
    FOR EACH STATEMENT EXECUTE FUNCTION "__SP__RecalculateMenuHPPTrigger"();