-- =============================================================================
-- 000023_ingredient_stock_procedures.up.sql
--
-- Memindahkan query mentah stok bahan dari repository ke STORED PROCEDURE,
-- agar logika data cukup diperbaiki di procedure tanpa build ulang aplikasi.
-- =============================================================================

-- 1. Stok usable + kadaluarsa terdekat per bahan.
--    p_ids = NULL / kosong  -> semua bahan
--    p_ids = 'uuid1,uuid2'  -> hanya bahan tertentu
CREATE OR REPLACE FUNCTION "__SP__IngredientUsableStock"(p_ids TEXT DEFAULT NULL)
RETURNS TABLE(ingredient_id UUID, usable NUMERIC, nearest_expiry DATE)
LANGUAGE plpgsql AS $$
BEGIN
    IF p_ids IS NULL OR btrim(p_ids) = '' THEN
        RETURN QUERY
        SELECT b.ingredient_id,
               COALESCE(SUM(b.remaining),0)::numeric AS usable,
               MIN(b.expires_at) AS nearest_expiry
          FROM ingredient_stock_batches b
         WHERE b.remaining > 0 AND b.expires_at >= CURRENT_DATE
         GROUP BY b.ingredient_id;
    ELSE
        RETURN QUERY
        SELECT b.ingredient_id,
               COALESCE(SUM(b.remaining),0)::numeric AS usable,
               MIN(b.expires_at) AS nearest_expiry
          FROM ingredient_stock_batches b
         WHERE b.remaining > 0 AND b.expires_at >= CURRENT_DATE
           AND b.ingredient_id = ANY (string_to_array(p_ids, ',')::uuid[])
         GROUP BY b.ingredient_id;
    END IF;
END;
$$;

-- 2. Bahan yang kadaluarsa / mendekati kadaluarsa dalam N hari.
CREATE OR REPLACE FUNCTION "__SP__ExpiredIngredients"(p_days INT DEFAULT 3)
RETURNS TABLE(id UUID, name VARCHAR, unit VARCHAR,
              expired_qty NUMERIC, usable_stock NUMERIC, nearest_expiry TEXT)
LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT i.id, i.name, i.unit,
           COALESCE(SUM(b.remaining) FILTER (WHERE b.expires_at < CURRENT_DATE),0)::numeric AS expired_qty,
           COALESCE(SUM(b.remaining) FILTER (WHERE b.expires_at >= CURRENT_DATE),0)::numeric AS usable_stock,
           MIN(b.expires_at)::text AS nearest_expiry
      FROM ingredients i
      JOIN ingredient_stock_batches b ON b.ingredient_id = i.id
     WHERE b.remaining > 0
     GROUP BY i.id, i.name, i.unit
    HAVING MIN(b.expires_at) < CURRENT_DATE + (p_days * INTERVAL '1 day')
     ORDER BY MIN(b.expires_at) ASC;
END;
$$;

-- 3. Log penambahan stok bahan pada rentang tanggal.
DROP FUNCTION IF EXISTS "__SP__StockLog"(DATE, DATE);
CREATE OR REPLACE FUNCTION "__SP__StockLog"(p_start_date DATE, p_end_date DATE)
RETURNS TABLE("type" TEXT, item_name TEXT, unit TEXT,
              quantity NUMERIC, remaining NUMERIC,
              expires_at TEXT, added_at TEXT, notes TEXT)
LANGUAGE plpgsql AS $$
BEGIN
    IF p_start_date IS NULL THEN p_start_date := CURRENT_DATE; END IF;
    IF p_end_date IS NULL THEN p_end_date := CURRENT_DATE; END IF;

    RETURN QUERY
    SELECT 'ingredient'::text AS "type",
           i.name::text AS item_name,
           i.unit::text AS unit,
           b.quantity,
           b.remaining,
           b.expires_at::text AS expires_at,
           b.added_at::date::text AS added_at,
           COALESCE(b.notes,'') AS notes
      FROM ingredient_stock_batches b
      JOIN ingredients i ON i.id = b.ingredient_id
     WHERE b.added_at::date BETWEEN p_start_date AND p_end_date
     ORDER BY b.added_at DESC, i.name ASC;
END;
$$;

-- 4. Daftar id + nama bahan (untuk label bahan pembatas stok menu).
CREATE OR REPLACE FUNCTION "__SP__IngredientNames"()
RETURNS TABLE(id UUID, name VARCHAR)
LANGUAGE sql AS $$
    SELECT id, name FROM ingredients;
$$;
