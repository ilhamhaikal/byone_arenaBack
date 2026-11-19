-- =============================================================================
-- 000034_menu_action_permissions.down.sql
-- =============================================================================

-- Hapus seluruh key aksi (<menu>.write / <menu>.delete).
UPDATE users
SET permissions = (
    SELECT COALESCE(jsonb_agg(p), '[]'::jsonb)
    FROM jsonb_array_elements_text(permissions) AS t(p)
    WHERE p NOT LIKE '%.write'
      AND p NOT LIKE '%.delete'
)
WHERE jsonb_typeof(permissions) = 'array';

-- Kembalikan menu Voucher untuk kasir (perilaku sebelum migrasi ini).
UPDATE users
SET permissions = (
    SELECT COALESCE(jsonb_agg(DISTINCT p), '[]'::jsonb)
    FROM (
        SELECT jsonb_array_elements_text(permissions) AS p
        UNION
        SELECT 'voucher'::text
    ) q
)
WHERE role = 'kasir'
  AND jsonb_typeof(permissions) = 'array';
