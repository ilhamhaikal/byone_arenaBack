-- =============================================================================
-- 000023_ingredient_stock_procedures.down.sql
-- =============================================================================

DROP FUNCTION IF EXISTS "__SP__IngredientUsableStock"(TEXT);
DROP FUNCTION IF EXISTS "__SP__ExpiredIngredients"(INT);
DROP FUNCTION IF EXISTS "__SP__StockLog"(DATE, DATE);
DROP FUNCTION IF EXISTS "__SP__IngredientNames"();
