DROP FUNCTION IF EXISTS "__SP__PaymentMethodSummary"(DATE, DATE);

DROP INDEX IF EXISTS idx_daily_rentals_payment_method;
DROP INDEX IF EXISTS idx_food_orders_payment_method;

ALTER TABLE daily_rentals DROP CONSTRAINT IF EXISTS daily_rentals_payment_method_check;
ALTER TABLE food_orders DROP CONSTRAINT IF EXISTS food_orders_payment_method_check;

ALTER TABLE daily_rentals DROP COLUMN IF EXISTS payment_method;
ALTER TABLE food_orders DROP COLUMN IF EXISTS payment_method;