-- Migration: 000011_add_report_summary_sp.up.sql
-- Stored procedure untuk laporan komprehensif dengan rentang tanggal

CREATE OR REPLACE FUNCTION "byoneReportSummary"(p_start_date DATE, p_end_date DATE)
RETURNS TABLE (
    report JSONB
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_total_days INT;
    v_result     JSONB;
BEGIN
    -- Validasi rentang
    IF p_start_date IS NULL THEN p_start_date := CURRENT_DATE - 7; END IF;
    IF p_end_date IS NULL THEN p_end_date := CURRENT_DATE; END IF;
    IF p_start_date > p_end_date THEN
        RAISE EXCEPTION 'INVALID_DATE_RANGE: Tanggal mulai tidak boleh lebih besar dari tanggal akhir';
    END IF;

    v_total_days := p_end_date - p_start_date + 1;

    -- Bangun report lengkap dalam satu JSON
    WITH
    -- Ringkasan revenue dari payments
    revenue_agg AS (
        SELECT
            COALESCE(SUM(p.total_payment), 0)        AS total_revenue,
            COALESCE(SUM(p.amount), 0)                AS total_base_amount,
            COALESCE(SUM(p.discount_amount), 0)       AS voucher_discount,
            COALESCE(SUM(p.auto_discount_amount), 0)  AS auto_discount,
            COALESCE(SUM(p.cash_received), 0)         AS total_cash_received,
            COALESCE(SUM(p.change_amount), 0)         AS total_change,
            COUNT(*)::INT                             AS total_transactions,
            COUNT(p.voucher_id)::INT                  AS voucher_transactions
        FROM payments p
        WHERE p.payment_status = 'paid'
          AND p.paid_at::DATE BETWEEN p_start_date AND p_end_date
    ),
    -- Ringkasan sesi
    session_agg AS (
        SELECT
            COUNT(*)::INT                              AS total_sessions,
            COALESCE(SUM(s.duration_minutes), 0)::INT  AS total_play_minutes,
            COALESCE(AVG(s.duration_minutes), 0)::INT  AS avg_duration_minutes
        FROM sessions s
        WHERE s.status = 'completed'
          AND s.created_at::DATE BETWEEN p_start_date AND p_end_date
    ),
    -- Penggunaan voucher
    voucher_usage AS (
        SELECT jsonb_agg(row_to_json(v)) AS list
        FROM (
            SELECT
                v.name       AS "voucherName",
                v.code       AS "voucherCode",
                v.discount_type AS "discountType",
                COUNT(*)::INT AS "usageCount",
                SUM(p.discount_amount)::NUMERIC(10,2) AS "totalDiscount"
            FROM payments p
            JOIN vouchers v ON v.id = p.voucher_id
            WHERE p.payment_status = 'paid'
              AND p.paid_at::DATE BETWEEN p_start_date AND p_end_date
              AND p.voucher_id IS NOT NULL
            GROUP BY v.id, v.name, v.code, v.discount_type
            ORDER BY "usageCount" DESC
        ) v
    ),
    -- Penggunaan konsol
    console_usage AS (
        SELECT jsonb_agg(row_to_json(c)) AS list
        FROM (
            SELECT
                c.name       AS "consoleName",
                c.console_type AS "consoleType",
                COUNT(*)::INT AS "totalSessions",
                COALESCE(SUM(s.duration_minutes), 0)::INT AS "totalMinutes"
            FROM sessions s
            JOIN consoles c ON c.id = s.console_id
            WHERE s.status = 'completed'
              AND s.created_at::DATE BETWEEN p_start_date AND p_end_date
            GROUP BY c.id, c.name, c.console_type
            ORDER BY "totalMinutes" DESC
        ) c
    ),
    -- Rincian per hari
    daily AS (
        SELECT jsonb_agg(row_to_json(d)) AS list
        FROM (
            SELECT
                dd::DATE                    AS "date",
                COALESCE(pd.revenue, 0)     AS "revenue",
                COALESCE(pd.transactions, 0) AS "transactions",
                COALESCE(sd.sessions, 0)    AS "sessions",
                COALESCE(sd.playMinutes, 0) AS "playMinutes"
            FROM generate_series(p_start_date, p_end_date, '1 day'::INTERVAL) dd
            LEFT JOIN LATERAL (
                SELECT
                    COALESCE(SUM(p.total_payment), 0) AS revenue,
                    COUNT(*)::INT AS transactions
                FROM payments p
                WHERE p.payment_status = 'paid'
                  AND p.paid_at::DATE = dd
            ) pd ON TRUE
            LEFT JOIN LATERAL (
                SELECT
                    COUNT(*)::INT AS sessions,
                    COALESCE(SUM(s.duration_minutes), 0)::INT AS playMinutes
                FROM sessions s
                WHERE s.status = 'completed'
                  AND s.created_at::DATE = dd
            ) sd ON TRUE
            ORDER BY dd
        ) d
    ),
    -- Jenis diskon yang diterapkan (rules summary)
    discount_rules_summary AS (
        SELECT jsonb_agg(row_to_json(dr)) AS list
        FROM (
            SELECT
                dr.name        AS "ruleName",
                dr.rule_type   AS "ruleType",
                dr.discount_type AS "discountType",
                dr.discount_value AS "discountValue",
                dr.is_active   AS "isActive"
            FROM discount_rules dr
            WHERE dr.is_active = TRUE
            ORDER BY dr.rule_type, dr.name
        ) dr
    )
    SELECT jsonb_build_object(
        'period', jsonb_build_object(
            'startDate', p_start_date,
            'endDate', p_end_date,
            'totalDays', v_total_days
        ),
        'revenue', jsonb_build_object(
            'totalRevenue', ra.total_revenue,
            'totalBaseAmount', ra.total_base_amount,
            'voucherDiscount', ra.voucher_discount,
            'autoDiscount', ra.auto_discount,
            'totalDiscount', ra.voucher_discount + ra.auto_discount,
            'totalCashReceived', ra.total_cash_received,
            'totalChange', ra.total_change
        ),
        'transactions', jsonb_build_object(
            'totalTransactions', ra.total_transactions,
            'voucherTransactions', ra.voucher_transactions,
            'averagePerDay', ROUND(ra.total_transactions::NUMERIC / GREATEST(v_total_days, 1), 1)
        ),
        'sessions', jsonb_build_object(
            'totalSessions', sa.total_sessions,
            'totalPlayMinutes', sa.total_play_minutes,
            'totalPlayHours', ROUND(sa.total_play_minutes::NUMERIC / 60.0, 1),
            'averageMinutes', sa.avg_duration_minutes
        ),
        'vouchers', COALESCE(vu.list, '[]'::JSONB),
        'consoles', COALESCE(cu.list, '[]'::JSONB),
        'dailyBreakdown', COALESCE(d.list, '[]'::JSONB),
        'activeDiscountRules', COALESCE(dr.list, '[]'::JSONB),
        'generatedAt', NOW()
    )
    INTO v_result
    FROM revenue_agg ra
    CROSS JOIN session_agg sa
    CROSS JOIN voucher_usage vu
    CROSS JOIN console_usage cu
    CROSS JOIN daily d
    CROSS JOIN discount_rules_summary dr;

    -- Kembalikan sebagai satu row
    RETURN QUERY SELECT v_result;
END;
$$;
