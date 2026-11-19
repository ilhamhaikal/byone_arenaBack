-- =============================================================================
-- 000036_console_price_permission.down.sql
-- =============================================================================

UPDATE users
SET permissions = permissions - 'console.price'
WHERE jsonb_typeof(permissions) = 'array';
