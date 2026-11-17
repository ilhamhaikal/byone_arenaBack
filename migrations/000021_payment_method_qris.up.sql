-- Metode pembayaran QRIS (dan transfer) untuk seluruh alur pembayaran.
--
-- payments.payment_method sudah mengizinkan 'cash','transfer','qris'.
-- Di sini ditambahkan kolom yang sama untuk pesanan makanan & rental harian,
-- plus stored procedure ringkasan per metode untuk laporan.

ALTER TABLE food_orders ADD COLUMN IF NOT EXISTS payment_method VARCHAR(20) NOT NULL DEFAULT 'cash';
ALTER TABLE daily_rentals ADD COLUMN IF NOT EXISTS payment_method VARCHAR(20) NOT NULL DEFAULT 'cash';

ALTER TABLE food_orders DROP CONSTRAINT IF EXISTS food_orders_payment_method_check;
ALTER TABLE food_orders ADD CONSTRAINT food_orders_payment_method_check
    CHECK (payment_method IN ('cash', 'qris', 'transfer'));

ALTER TABLE daily_rentals DROP CONSTRAINT IF EXISTS daily_rentals_payment_method_check;
ALTER TABLE daily_rentals ADD CONSTRAINT daily_rentals_payment_method_check
    CHECK (payment_method IN ('cash', 'qris', 'transfer'));

CREATE INDEX IF NOT EXISTS idx_food_orders_payment_method ON food_orders (payment_method);
CREATE INDEX IF NOT EXISTS idx_daily_rentals_payment_method ON daily_rentals (payment_method);

-- Ringkasan pendapatan per metode pembayaran (rental + makanan + rental harian).
CREATE OR REPLACE FUNCTION "__SP__PaymentMethodSummary"(p_start_date DATE, p_end_date DATE)
RETURNS TABLE(payment_method VARCHAR, total NUMERIC, trx INT)
LANGUAGE plpgsql AS $$
BEGIN
    IF p_start_date IS NULL THEN
        p_start_date := CURRENT_DATE - 7;
    END IF;
    IF p_end_date IS NULL THEN
        p_end_date := CURRENT_DATE;
    END IF;

    RETURN QUERY
    WITH src AS (
        SELECT p.payment_method::VARCHAR AS m, p.total_payment AS amount
          FROM payments p
         WHERE p.payment_status = 'paid'
           AND p.paid_at::DATE BETWEEN p_start_date AND p_end_date
        UNION ALL
        SELECT f.payment_method::VARCHAR, f.total_amount
          FROM food_orders f
         WHERE f.status = 'served'
           AND f.updated_at::DATE BETWEEN p_start_date AND p_end_date
        UNION ALL
        SELECT d.payment_method::VARCHAR, d.final_amount
          FROM daily_rentals d
         WHERE d.created_at::DATE BETWEEN p_start_date AND p_end_date
    )
    SELECT m, COALESCE(SUM(amount), 0), COUNT(*)::INT
      FROM src
     GROUP BY m
     ORDER BY m;
END;
$$;