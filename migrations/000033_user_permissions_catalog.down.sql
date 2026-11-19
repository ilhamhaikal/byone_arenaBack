-- =============================================================================
-- 000033_user_permissions_catalog.down.sql
-- =============================================================================

-- Kembalikan SetAppSetting ke perilaku lama (UPDATE saja).
CREATE OR REPLACE FUNCTION "__SP__SetAppSetting"(p_key TEXT, p_value TEXT)
RETURNS VOID
LANGUAGE sql AS $$
    UPDATE app_settings SET value = p_value, updated_at = NOW() WHERE key = p_key;
$$;

-- Hapus key yang ditambahkan migrasi ini. Izin custom lain tetap dipertahankan.
UPDATE users
SET permissions = permissions
    - 'promo_bonus'
    - 'controller'
    - 'member_price'
    - 'daily_price'
WHERE jsonb_typeof(permissions) = 'array';
