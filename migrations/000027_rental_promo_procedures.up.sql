-- =============================================================================
-- 000027_rental_promo_procedures.up.sql
--
-- Memindahkan query mentah sisa (daily_price konsol, set metode rental harian,
-- diskon pesanan makanan, laporan bonus promo) ke STORED PROCEDURE.
-- =============================================================================

-- ── CONSOLE / DAILY RENTAL ──────────────────────────────────────────────────

CREATE OR REPLACE FUNCTION "__SP__ConsoleDailyPrice"(p_console_id UUID)
RETURNS TABLE(daily_price NUMERIC)
LANGUAGE sql AS $$
    SELECT daily_price FROM consoles WHERE id = p_console_id;
$$;

CREATE OR REPLACE FUNCTION "__SP__SetDailyRentalPaymentMethod"(p_rental_id UUID, p_method VARCHAR)
RETURNS VOID
LANGUAGE sql AS $$
    UPDATE daily_rentals SET payment_method = p_method WHERE id = p_rental_id;
$$;

-- ── FOOD ORDER ──────────────────────────────────────────────────────────────

CREATE OR REPLACE FUNCTION "__SP__SetFoodOrderDiscount"(p_order_id UUID, p_discount NUMERIC)
RETURNS VOID
LANGUAGE sql AS $$
    UPDATE food_orders SET discount_amount = p_discount WHERE id = p_order_id;
$$;

-- ── PROMO BONUS REPORT ──────────────────────────────────────────────────────

CREATE OR REPLACE FUNCTION "__SP__PromoBonusReport"(p_start_date DATE, p_end_date DATE)
RETURNS TABLE(date TEXT, sessions_with_bonus INT, total_bonus_minutes INT)
LANGUAGE plpgsql AS $$
BEGIN
    IF p_start_date IS NULL THEN p_start_date := CURRENT_DATE - 7; END IF;
    IF p_end_date IS NULL THEN p_end_date := CURRENT_DATE; END IF;

    RETURN QUERY
    SELECT TO_CHAR(s.start_time AT TIME ZONE 'Asia/Jakarta', 'YYYY-MM-DD') AS date,
           COUNT(*) FILTER (WHERE s.promo_bonus_minutes > 0)::INT AS sessions_with_bonus,
           COALESCE(SUM(s.promo_bonus_minutes), 0)::INT AS total_bonus_minutes
      FROM sessions s
     WHERE (s.start_time AT TIME ZONE 'Asia/Jakarta')::DATE BETWEEN p_start_date AND p_end_date
       AND s.status <> 'cancelled'
     GROUP BY 1
     ORDER BY 1;
END;
$$;
