-- =============================================================================
-- 000001_init_schema.down.sql — Drop semua tabel, index, sequence, extension
-- =============================================================================

-- Drop semua tabel (urutan berdasarkan dependency)
DROP TABLE IF EXISTS tv_activity_logs;
DROP TABLE IF EXISTS tv_notifications;
DROP TABLE IF EXISTS food_order_items;
DROP TABLE IF EXISTS food_orders;
DROP TABLE IF EXISTS menu_items;
DROP TABLE IF EXISTS bookings;
DROP TABLE IF EXISTS daily_rentals;
DROP TABLE IF EXISTS console_pricing_tiers;
DROP TABLE IF EXISTS discount_rules;
DROP TABLE IF EXISTS vouchers;
DROP TABLE IF EXISTS app_settings;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS memberships;
DROP TABLE IF EXISTS shifts;
DROP TABLE IF EXISTS customers;
DROP TABLE IF EXISTS consoles;
DROP TABLE IF EXISTS users;

-- Drop sequence
DROP SEQUENCE IF EXISTS food_order_daily_seq;

-- Drop extension
DROP EXTENSION IF EXISTS "uuid-ossp";
