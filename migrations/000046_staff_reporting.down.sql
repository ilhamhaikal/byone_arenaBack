DROP FUNCTION IF EXISTS "__SP__StaffLoginSessions"(DATE, DATE, UUID);
DROP FUNCTION IF EXISTS "__SP__StaffDailyRevenue"(DATE, DATE, UUID);
DROP INDEX IF EXISTS idx_daily_rentals_created_by;
DROP INDEX IF EXISTS idx_food_orders_created_by;
ALTER TABLE daily_rentals DROP COLUMN IF EXISTS created_by;
ALTER TABLE food_orders DROP COLUMN IF EXISTS created_by;
DROP TABLE IF EXISTS auth_sessions;
