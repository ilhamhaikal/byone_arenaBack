-- =============================================================================
-- 000022_revenue_chart_split.up.sql
--
-- Grafik pendapatan dipisah menjadi DUA seri (rental/harian vs makanan)
-- dan ringkasan metode pembayaran khusus makanan.
--
-- Semua logika data ditaruh di STORED PROCEDURE (bukan query di aplikasi),
-- sehingga bila ada bug/penyesuaian cukup perbaiki procedure-nya tanpa
-- perlu build ulang aplikasi.
-- =============================================================================

-- 1. RevenueChart — tambah kolom rental_revenue & food_revenue.
--    rental_revenue = sewa konsol (sesi) + rental harian + membership
--    food_revenue   = makanan/minuman
DROP FUNCTION IF EXISTS "__SP__RevenueChart"(INT);
CREATE OR REPLACE FUNCTION "__SP__RevenueChart"(p_days INT DEFAULT 7)
RETURNS TABLE(date TEXT, rental_revenue NUMERIC, food_revenue NUMERIC, revenue NUMERIC, sessions INT)
LANGUAGE plpgsql AS $$
BEGIN
IF p_days < 1 THEN p_days := 7; END IF;
IF p_days > 30 THEN p_days := 30; END IF;

RETURN QUERY
SELECT dd::DATE::TEXT,
       (COALESCE(sr.rev,0) + COALESCE(dr.rev,0) + COALESCE(mr.rev,0)) AS rental_revenue,
       COALESCE(fr.rev,0) AS food_revenue,
       (COALESCE(sr.rev,0) + COALESCE(dr.rev,0) + COALESCE(mr.rev,0) + COALESCE(fr.rev,0)) AS revenue,
       (COALESCE(sr.sessions,0) + COALESCE(dr.rentals,0) + COALESCE(mr.memberships,0) + COALESCE(fr.orders,0))::INT AS sessions
FROM generate_series(CURRENT_DATE-(p_days-1), CURRENT_DATE, '1 day'::INTERVAL) dd
LEFT JOIN LATERAL (
    SELECT COALESCE(SUM(p.total_payment),0) AS rev, COUNT(*)::INT AS sessions
    FROM payments p
    WHERE p.payment_status='paid' AND p.session_id IS NOT NULL AND p.paid_at::DATE = dd::DATE
) sr ON TRUE
LEFT JOIN LATERAL (
    SELECT COALESCE(SUM(d.final_amount),0) AS rev, COUNT(*)::INT AS rentals
    FROM daily_rentals d
    WHERE d.created_at::DATE = dd::DATE
) dr ON TRUE
LEFT JOIN LATERAL (
    SELECT COALESCE(SUM(p2.total_payment),0) AS rev, COUNT(*)::INT AS memberships
    FROM payments p2
    WHERE p2.payment_status='paid' AND p2.session_id IS NULL AND p2.paid_at::DATE = dd::DATE
) mr ON TRUE
LEFT JOIN LATERAL (
    SELECT COALESCE(SUM(fo.total_amount - COALESCE(fo.discount_amount,0)),0) AS rev, COUNT(*)::INT AS orders
    FROM food_orders fo
    WHERE fo.status='served' AND fo.updated_at::DATE = dd::DATE
) fr ON TRUE
ORDER BY dd::DATE;
END;
$$;

-- 2. FoodPaymentMethodSummary — ringkasan metode pembayaran KHUSUS makanan.
--    Dipakai kartu "Makanan" (terpisah dari kartu Pendapatan).
CREATE OR REPLACE FUNCTION "__SP__FoodPaymentMethodSummary"(p_start_date DATE, p_end_date DATE)
RETURNS TABLE(payment_method VARCHAR, total NUMERIC, trx INT)
LANGUAGE plpgsql AS $$
BEGIN
    IF p_start_date IS NULL THEN p_start_date := CURRENT_DATE; END IF;
    IF p_end_date IS NULL THEN p_end_date := CURRENT_DATE; END IF;

    RETURN QUERY
    SELECT fo.payment_method::VARCHAR,
           COALESCE(SUM(fo.total_amount),0) AS total,
           COUNT(*)::INT AS trx
      FROM food_orders fo
     WHERE fo.status='served'
       AND fo.updated_at::DATE BETWEEN p_start_date AND p_end_date
     GROUP BY fo.payment_method
     ORDER BY fo.payment_method;
END;
$$;
