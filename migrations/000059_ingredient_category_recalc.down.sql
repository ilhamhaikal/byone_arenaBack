-- Rollback 000059: trigger hanya memantau cost_per_unit lagi.

DROP TRIGGER IF EXISTS trg_ingredient_cost_recalc ON ingredients;

CREATE TRIGGER trg_ingredient_cost_recalc
    AFTER UPDATE OF cost_per_unit ON ingredients
    FOR EACH STATEMENT EXECUTE FUNCTION "__SP__RecalculateMenuHPPTrigger"();