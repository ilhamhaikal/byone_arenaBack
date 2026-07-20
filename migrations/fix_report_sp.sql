CREATE OR REPLACE FUNCTION "byoneReportSummary"(p_start_date DATE, p_end_date DATE)
RETURNS TABLE(report JSONB)
LANGUAGE plpgsql
AS $$
DECLARE v_total_days INT; v_result JSONB;
BEGIN
    IF p_start_date IS NULL THEN p_start_date := CURRENT_DATE - 7; END IF;
    IF p_end_date IS NULL THEN p_end_date := CURRENT_DATE; END IF;
    IF p_start_date > p_end_date THEN RAISE EXCEPTION 'INVALID_DATE_RANGE'; END IF;
    v_total_days := p_end_date - p_start_date + 1;

    WITH
    session_rev AS (
        SELECT
            COALESCE(SUM(p.total_payment), 0)   AS revenue,
            COALESCE(SUM(p.amount), 0)           AS base_amount,
            COALESCE(SUM(p.discount_amount), 0)  AS discount,
            COALESCE(SUM(p.auto_discount_amount), 0) AS auto_discount,
            COALESCE(SUM(p.cash_received), 0)    AS cash_received,
            COALESCE(SUM(p.change_amount), 0)    AS change_amount,
            COUNT(*)::INT                        AS trx_count,
            COUNT(p.voucher_id)::INT             AS voucher_trx
        FROM payments p
        WHERE p.payment_status = 'paid' AND p.session_id IS NOT NULL
          AND p.paid_at::DATE BETWEEN p_start_date AND p_end_date
    ),
    rental_rev AS (
        SELECT
            COALESCE(SUM(d.final_amount), 0)   AS revenue,
            COALESCE(SUM(d.total_amount), 0)    AS base_amount,
            COALESCE(SUM(d.discount_amount), 0) AS discount,
            0::NUMERIC                          AS auto_discount,
            COALESCE(SUM(d.final_amount), 0)    AS cash_received,
            0::NUMERIC                          AS change_amount,
            COUNT(*)::INT                       AS trx_count,
            0::INT                              AS voucher_trx
        FROM daily_rentals d
        WHERE d.created_at::DATE BETWEEN p_start_date AND p_end_date
    ),
    member_rev AS (
        SELECT
            COALESCE(SUM(p.total_payment), 0)   AS revenue,
            COALESCE(SUM(p.amount), 0)           AS base_amount,
            COALESCE(SUM(p.discount_amount), 0)  AS discount,
            0::NUMERIC                          AS auto_discount,
            COALESCE(SUM(p.cash_received), 0)    AS cash_received,
            COALESCE(SUM(p.change_amount), 0)    AS change_amount,
            COUNT(*)::INT                       AS trx_count,
            0::INT                              AS voucher_trx
        FROM payments p
        WHERE p.payment_status = 'paid' AND p.session_id IS NULL
          AND p.paid_at::DATE BETWEEN p_start_date AND p_end_date
    ),
    combined AS (
        SELECT
            sr.revenue + rr.revenue + mr.revenue           AS total_revenue,
            sr.base_amount + rr.base_amount + mr.base_amount AS total_base,
            sr.discount + rr.discount + mr.discount         AS total_discount,
            sr.auto_discount + rr.auto_discount + mr.auto_discount AS total_auto,
            sr.cash_received + rr.cash_received + mr.cash_received AS total_cash,
            sr.change_amount + rr.change_amount + mr.change_amount AS total_change,
            sr.trx_count + rr.trx_count + mr.trx_count     AS total_trx,
            sr.voucher_trx + rr.voucher_trx + mr.voucher_trx AS voucher_trx,
            rr.revenue AS rental_rev, rr.trx_count AS rental_count,
            mr.revenue AS member_rev, mr.trx_count AS member_count
        FROM session_rev sr
        CROSS JOIN rental_rev rr
        CROSS JOIN member_rev mr
    ),
    session_agg AS (
        SELECT COUNT(*)::INT AS total_sessions,
            COALESCE(SUM(s.duration_minutes), 0)::INT AS total_play_minutes,
            COALESCE(AVG(s.duration_minutes), 0)::INT AS avg_duration_minutes
        FROM sessions s WHERE s.status = 'completed'
          AND s.created_at::DATE BETWEEN p_start_date AND p_end_date
    ),
    voucher_usage AS (
        SELECT COALESCE(jsonb_agg(row_to_json(v)), '[]'::JSONB) AS list FROM (
            SELECT v.name AS "voucherName", v.code AS "voucherCode",
                v.discount_type AS "discountType", COUNT(*)::INT AS "usageCount",
                SUM(p.discount_amount)::NUMERIC(10,2) AS "totalDiscount"
            FROM payments p JOIN vouchers v ON v.id = p.voucher_id
            WHERE p.payment_status = 'paid' AND p.voucher_id IS NOT NULL
              AND p.paid_at::DATE BETWEEN p_start_date AND p_end_date
            GROUP BY v.id, v.name, v.code, v.discount_type ORDER BY "usageCount" DESC
        ) v
    ),
    console_usage AS (
        SELECT COALESCE(jsonb_agg(row_to_json(c)), '[]'::JSONB) AS list FROM (
            SELECT c.name AS "consoleName", c.console_type AS "consoleType",
                COUNT(*)::INT AS "totalSessions",
                COALESCE(SUM(s.duration_minutes), 0)::INT AS "totalMinutes"
            FROM sessions s JOIN consoles c ON c.id = s.console_id
            WHERE s.status = 'completed'
              AND s.created_at::DATE BETWEEN p_start_date AND p_end_date
            GROUP BY c.id, c.name, c.console_type ORDER BY "totalMinutes" DESC
        ) c
    ),
    daily AS (
        SELECT COALESCE(jsonb_agg(row_to_json(d) ORDER BY d."date"), '[]'::JSONB) AS list FROM (
            SELECT dd::DATE AS "date",
                COALESCE(pd.rev, 0) + COALESCE(drd.rev, 0) + COALESCE(md.rev, 0) AS "revenue",
                COALESCE(pd.trx, 0) + COALESCE(drd.trx, 0) + COALESCE(md.trx, 0) AS "transactions",
                COALESCE(sd.sessions, 0) AS "sessions",
                COALESCE(sd.playMinutes, 0) AS "playMinutes",
                COALESCE(drd.rental_count, 0) AS "dailyRentals",
                COALESCE(md.member_count, 0) AS "memberships"
            FROM generate_series(p_start_date, p_end_date, '1 day'::INTERVAL) dd
            LEFT JOIN LATERAL (
                SELECT COALESCE(SUM(p.total_payment), 0) AS rev, COUNT(*)::INT AS trx
                FROM payments p WHERE p.payment_status = 'paid' AND p.session_id IS NOT NULL AND p.paid_at::DATE = dd
            ) pd ON TRUE
            LEFT JOIN LATERAL (
                SELECT COALESCE(SUM(d2.final_amount), 0) AS rev, COUNT(*)::INT AS trx,
                       COUNT(*)::INT AS rental_count
                FROM daily_rentals d2 WHERE d2.created_at::DATE = dd
            ) drd ON TRUE
            LEFT JOIN LATERAL (
                SELECT COALESCE(SUM(p2.total_payment), 0) AS rev, COUNT(*)::INT AS trx,
                       COUNT(*)::INT AS member_count
                FROM payments p2 WHERE p2.payment_status = 'paid' AND p2.session_id IS NULL AND p2.paid_at::DATE = dd
            ) md ON TRUE
            LEFT JOIN LATERAL (
                SELECT COUNT(*)::INT AS sessions, COALESCE(SUM(s2.duration_minutes), 0)::INT AS playMinutes
                FROM sessions s2 WHERE s2.status = 'completed' AND s2.created_at::DATE = dd
            ) sd ON TRUE
        ) d
    ),
    discount_rules_summary AS (
        SELECT COALESCE(jsonb_agg(row_to_json(dr)), '[]'::JSONB) AS list FROM (
            SELECT dr.name AS "ruleName", dr.rule_type AS "ruleType",
                dr.discount_type AS "discountType", dr.discount_value AS "discountValue", dr.is_active AS "isActive"
            FROM discount_rules dr WHERE dr.is_active = TRUE ORDER BY dr.rule_type, dr.name
        ) dr
    )
    SELECT jsonb_build_object(
        'period', jsonb_build_object('startDate', p_start_date, 'endDate', p_end_date, 'totalDays', v_total_days),
        'revenue', jsonb_build_object(
            'totalRevenue', cb.total_revenue,
            'totalBaseAmount', cb.total_base,
            'voucherDiscount', cb.total_discount,
            'autoDiscount', cb.total_auto,
            'totalDiscount', cb.total_discount + cb.total_auto,
            'totalCashReceived', cb.total_cash,
            'totalChange', cb.total_change,
            'dailyRentalRevenue', cb.rental_rev,
            'dailyRentalCount', cb.rental_count,
            'membershipRevenue', cb.member_rev,
            'membershipCount', cb.member_count
        ),
        'transactions', jsonb_build_object(
            'totalTransactions', cb.total_trx,
            'voucherTransactions', cb.voucher_trx,
            'averagePerDay', ROUND(cb.total_trx::NUMERIC / GREATEST(v_total_days, 1), 1)
        ),
        'sessions', jsonb_build_object(
            'totalSessions', sa.total_sessions,
            'totalPlayMinutes', sa.total_play_minutes,
            'totalPlayHours', ROUND(sa.total_play_minutes::NUMERIC / 60.0, 1),
            'averageMinutes', sa.avg_duration_minutes
        ),
        'vouchers', vu.list,
        'consoles', cu.list,
        'dailyBreakdown', d.list,
        'activeDiscountRules', dr.list,
        'generatedAt', NOW()
    ) INTO v_result
    FROM combined cb
    CROSS JOIN session_agg sa
    CROSS JOIN voucher_usage vu
    CROSS JOIN console_usage cu
    CROSS JOIN daily d
    CROSS JOIN discount_rules_summary dr;

    RETURN QUERY SELECT v_result;
END;
$$;
