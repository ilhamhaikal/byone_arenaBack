-- =============================================================================
-- 000022_revenue_chart_split.down.sql
-- Kembalikan RevenueChart ke versi 3 kolom (satu seri total) dan hapus SP
-- ringkasan metode pembayaran makanan.
-- =============================================================================

DROP FUNCTION IF EXISTS "__SP__FoodPaymentMethodSummary"(DATE, DATE);

DROP FUNCTION IF EXISTS "__SP__RevenueChart"(INT);
CREATE OR REPLACE FUNCTION "__SP__RevenueChart"(p_days INT DEFAULT 7)
RETURNS TABLE(date TEXT, revenue NUMERIC, sessions INT)
LANGUAGE plpgsql AS $$
BEGIN
IF p_days<1 THEN p_days:=7; END IF; IF p_days>30 THEN p_days:=30; END IF;
RETURN QUERY
SELECT dd::DATE::TEXT,
       COALESCE(sr.rev,0)+COALESCE(dr.rev,0)+COALESCE(mr.rev,0)+COALESCE(fr.rev,0) AS revenue,
       (COALESCE(sr.sessions,0)+COALESCE(dr.rentals,0)+COALESCE(mr.memberships,0)+COALESCE(fr.orders,0))::INT AS sessions
FROM generate_series(CURRENT_DATE-(p_days-1),CURRENT_DATE,'1 day'::INTERVAL) dd
LEFT JOIN LATERAL (SELECT COALESCE(SUM(p.total_payment),0) AS rev, COUNT(*)::INT AS sessions FROM payments p WHERE p.payment_status='paid' AND p.session_id IS NOT NULL AND p.paid_at::DATE=dd::DATE) sr ON TRUE
LEFT JOIN LATERAL (SELECT COALESCE(SUM(d.final_amount),0) AS rev, COUNT(*)::INT AS rentals FROM daily_rentals d WHERE d.created_at::DATE=dd::DATE) dr ON TRUE
LEFT JOIN LATERAL (SELECT COALESCE(SUM(p2.total_payment),0) AS rev, COUNT(*)::INT AS memberships FROM payments p2 WHERE p2.payment_status='paid' AND p2.session_id IS NULL AND p2.paid_at::DATE=dd::DATE) mr ON TRUE
LEFT JOIN LATERAL (SELECT COALESCE(SUM(fo.total_amount - COALESCE(fo.discount_amount,0)),0) AS rev, COUNT(*)::INT AS orders FROM food_orders fo WHERE fo.status='served' AND fo.updated_at::DATE=dd::DATE) fr ON TRUE
ORDER BY dd::DATE;
END;
$$;
