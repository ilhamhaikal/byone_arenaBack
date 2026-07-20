-- Fix: add voucher_id to daily_rentals + update SPs for proper breakdown
-- Run: PGPASSWORD=By0N3-4r3NA psql -h localhost -U tesla -d byone_arena -f this_file.sql

-- 1. Add voucher_id column
ALTER TABLE daily_rentals ADD COLUMN IF NOT EXISTS voucher_id UUID REFERENCES vouchers(id);

-- 2. Update byoneCreateDailyRental to store voucher_id
DROP FUNCTION IF EXISTS "byoneCreateDailyRental"(UUID, UUID, DATE, DATE, NUMERIC, VARCHAR, TEXT);

CREATE OR REPLACE FUNCTION "byoneCreateDailyRental"(
    p_console_id    UUID,
    p_customer_id   UUID,
    p_start_date    DATE,
    p_end_date      DATE,
    p_daily_price   NUMERIC,
    p_voucher_code  VARCHAR DEFAULT NULL,
    p_notes         TEXT DEFAULT NULL
)
RETURNS TABLE(
    rental_id UUID, total_days INT, total_amount NUMERIC,
    discount_amount NUMERIC, free_days_used INT, final_amount NUMERIC, status VARCHAR
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_rental_id         UUID;
    v_console_stat      VARCHAR;
    v_total_days        INT;
    v_total_amount      NUMERIC(10,2);
    v_voucher_discount  NUMERIC(10,2) := 0;
    v_free_days         INT := 0;
    v_final_amount      NUMERIC(10,2);
    v_actual_end_date   DATE;
    v_voucher_type      VARCHAR(20);
    v_voucher_id        UUID := NULL;
    v_now               TIMESTAMPTZ := NOW();
BEGIN
    SELECT c.status INTO v_console_stat FROM consoles c WHERE c.id = p_console_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'CONSOLE_NOT_FOUND'; END IF;
    IF v_console_stat != 'available' THEN RAISE EXCEPTION 'CONSOLE_NOT_AVAILABLE'; END IF;
    IF p_end_date <= p_start_date THEN RAISE EXCEPTION 'INVALID_DATE'; END IF;

    v_total_days := p_end_date - p_start_date;
    IF v_total_days < 1 THEN v_total_days := 1; END IF;
    v_total_amount := v_total_days * p_daily_price;

    IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code) != '' THEN
        SELECT discount_type INTO v_voucher_type FROM vouchers WHERE code = UPPER(p_voucher_code);
        
        IF v_voucher_type = 'free_days' THEN
            BEGIN
                SELECT va.free_days_used INTO v_free_days
                FROM "byoneApplyVoucher"(p_voucher_code, v_total_amount, p_daily_price, v_total_days) va;
                UPDATE vouchers SET usage_count = usage_count + 1, updated_at = v_now
                WHERE code = UPPER(p_voucher_code);
            EXCEPTION WHEN OTHERS THEN v_free_days := 0; END;
            v_voucher_discount := 0;
        ELSE
            BEGIN
                SELECT va.voucher_id, va.discount_amount INTO v_voucher_id, v_voucher_discount
                FROM "byoneApplyVoucher"(p_voucher_code, v_total_amount) va;
                UPDATE vouchers SET usage_count = usage_count + 1, updated_at = v_now
                WHERE code = UPPER(p_voucher_code);
            EXCEPTION WHEN OTHERS THEN v_voucher_discount := 0; END;
            v_free_days := 0;
        END IF;
    END IF;

    IF v_voucher_discount > v_total_amount THEN v_voucher_discount := v_total_amount; END IF;
    v_final_amount := v_total_amount - v_voucher_discount;

    v_rental_id := uuid_generate_v4();
    IF v_free_days > 0 THEN
        v_total_days := v_total_days + v_free_days;
        v_actual_end_date := p_start_date + v_total_days;
    ELSE
        v_actual_end_date := p_end_date;
    END IF;

    INSERT INTO daily_rentals(id, console_id, customer_id, start_date, end_date,
        daily_price, total_days, free_days_used, total_amount,
        discount_amount, final_amount, voucher_id,
        status, notes, created_at, updated_at)
    VALUES(v_rental_id, p_console_id, p_customer_id, p_start_date, v_actual_end_date,
        p_daily_price, v_total_days, v_free_days, v_total_amount,
        v_voucher_discount, v_final_amount, v_voucher_id,
        'active', p_notes, v_now, v_now);

    UPDATE consoles SET status = 'rented_out', updated_at = v_now
    WHERE consoles.id = p_console_id;

    RETURN QUERY SELECT v_rental_id, v_total_days, v_total_amount,
        v_voucher_discount, v_free_days, v_final_amount, 'active'::VARCHAR;
END;
$$;

-- 3. Update byoneReportSummary: voucher usage from BOTH sessions AND daily rentals
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
            COUNT(d.voucher_id)::INT            AS voucher_trx
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
    -- Voucher usage: session payments + daily rentals
    voucher_usage AS (
        SELECT COALESCE(jsonb_agg(row_to_json(v) ORDER BY v."usageCount" DESC), '[]'::JSONB) AS list FROM (
            SELECT v.name AS "voucherName", v.code AS "voucherCode",
                v.discount_type AS "discountType",
                COUNT(*)::INT AS "usageCount",
                SUM(src.discount_amount)::NUMERIC(10,2) AS "totalDiscount"
            FROM (
                -- Session vouchers
                SELECT p.voucher_id, p.discount_amount
                FROM payments p
                WHERE p.payment_status = 'paid' AND p.voucher_id IS NOT NULL
                  AND p.paid_at::DATE BETWEEN p_start_date AND p_end_date
                UNION ALL
                -- Daily rental vouchers
                SELECT d.voucher_id, d.discount_amount
                FROM daily_rentals d
                WHERE d.voucher_id IS NOT NULL
                  AND d.created_at::DATE BETWEEN p_start_date AND p_end_date
            ) src
            JOIN vouchers v ON v.id = src.voucher_id
            GROUP BY v.id, v.name, v.code, v.discount_type
        ) v
    ),
    console_usage AS (
        SELECT COALESCE(jsonb_agg(row_to_json(c) ORDER BY c."totalMinutes" DESC), '[]'::JSONB) AS list FROM (
            SELECT c.name AS "consoleName", c.console_type AS "consoleType",
                COUNT(*)::INT AS "totalSessions",
                COALESCE(SUM(s.duration_minutes), 0)::INT AS "totalMinutes"
            FROM sessions s JOIN consoles c ON c.id = s.console_id
            WHERE s.status = 'completed'
              AND s.created_at::DATE BETWEEN p_start_date AND p_end_date
            GROUP BY c.id, c.name, c.console_type
        ) c
    ),
    daily AS (
        SELECT COALESCE(jsonb_agg(row_to_json(d) ORDER BY d."date"), '[]'::JSONB) AS list FROM (
            SELECT dd::DATE AS "date",
                COALESCE(pd.rev, 0) + COALESCE(drd.rev, 0) + COALESCE(md.rev, 0) AS "revenue",
                COALESCE(pd.trx, 0) + COALESCE(drd.trx, 0) + COALESCE(md.trx, 0) AS "transactions",
                COALESCE(sd.sessions, 0) AS "sessions",
                COALESCE(sd.playMinutes, 0) AS "playMinutes",
                COALESCE(drd.rental_rev, 0) AS "rentalRevenue",
                COALESCE(drd.rental_count, 0) AS "dailyRentals",
                COALESCE(md.member_rev, 0) AS "membershipRevenue",
                COALESCE(md.member_count, 0) AS "memberships"
            FROM generate_series(p_start_date, p_end_date, '1 day'::INTERVAL) dd
            LEFT JOIN LATERAL (
                SELECT COALESCE(SUM(p.total_payment), 0) AS rev, COUNT(*)::INT AS trx
                FROM payments p WHERE p.payment_status = 'paid' AND p.session_id IS NOT NULL AND p.paid_at::DATE = dd
            ) pd ON TRUE
            LEFT JOIN LATERAL (
                SELECT COALESCE(SUM(d2.final_amount), 0) AS rev,
                       COALESCE(SUM(d2.total_amount), 0) AS rental_rev,
                       COUNT(*)::INT AS trx, COUNT(*)::INT AS rental_count
                FROM daily_rentals d2 WHERE d2.created_at::DATE = dd
            ) drd ON TRUE
            LEFT JOIN LATERAL (
                SELECT COALESCE(SUM(p2.total_payment), 0) AS rev,
                       COALESCE(SUM(p2.amount), 0) AS member_rev,
                       COUNT(*)::INT AS trx, COUNT(*)::INT AS member_count
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

-- 4. Backfill: update existing daily_rentals that used HEMAT50RB voucher
UPDATE daily_rentals d
SET voucher_id = (SELECT id FROM vouchers WHERE code = 'HEMAT50RB')
WHERE d.discount_amount = 50000 AND d.voucher_id IS NULL;

-- 5. Update byoneDashboardSummary: include daily rental voucher count + membership
DROP FUNCTION IF EXISTS "byoneDashboardSummary"(DATE);

CREATE OR REPLACE FUNCTION "byoneDashboardSummary"(p_date DATE DEFAULT CURRENT_DATE)
RETURNS TABLE(
    total_revenue NUMERIC, total_base_amount NUMERIC, total_transactions BIGINT,
    total_discount NUMERIC, total_auto_discount NUMERIC, voucher_usage_count BIGINT,
    total_cash_received NUMERIC, total_change NUMERIC,
    active_sessions INT, available_consoles INT, total_consoles INT,
    daily_rental_revenue NUMERIC, daily_rental_count BIGINT,
    membership_revenue NUMERIC, membership_count BIGINT,
    voucher_details JSONB
)
LANGUAGE plpgsql
AS $$
BEGIN
    WITH payment_agg AS (
        SELECT
            COALESCE(SUM(p.total_payment), 0)        AS total_rev,
            COALESCE(SUM(p.amount), 0)                AS total_base,
            COUNT(*)::BIGINT                          AS total_trx,
            COALESCE(SUM(p.discount_amount), 0)       AS total_disc,
            COALESCE(SUM(p.auto_discount_amount), 0)  AS total_auto,
            COUNT(p.voucher_id)::BIGINT               AS voucher_count,
            COALESCE(SUM(p.cash_received), 0)         AS total_cash,
            COALESCE(SUM(p.change_amount), 0)         AS total_chg
        FROM payments p
        WHERE p.payment_status = 'paid' AND p.session_id IS NOT NULL
          AND p.paid_at::DATE = p_date
    ),
    daily_rental_agg AS (
        SELECT COALESCE(SUM(d.final_amount), 0) AS rental_rev, COUNT(*)::BIGINT AS rental_count
        FROM daily_rentals d WHERE d.created_at::DATE = p_date
    ),
    membership_agg AS (
        SELECT COALESCE(SUM(p.total_payment), 0) AS member_rev, COUNT(*)::BIGINT AS member_count
        FROM payments p WHERE p.payment_status = 'paid' AND p.session_id IS NULL AND p.paid_at::DATE = p_date
    ),
    voucher_agg AS (
        SELECT v.name AS voucher_name, v.code AS voucher_code,
            COUNT(*) AS usage_count, SUM(src.discount_amount) AS total_discount
        FROM (
            SELECT p.voucher_id, p.discount_amount FROM payments p
            WHERE p.payment_status = 'paid' AND p.voucher_id IS NOT NULL AND p.paid_at::DATE = p_date
            UNION ALL
            SELECT d.voucher_id, d.discount_amount FROM daily_rentals d
            WHERE d.voucher_id IS NOT NULL AND d.created_at::DATE = p_date
        ) src
        JOIN vouchers v ON v.id = src.voucher_id
        GROUP BY v.id, v.name, v.code ORDER BY usage_count DESC
    )
    SELECT
        pa.total_rev + dra.rental_rev + ma.member_rev,
        pa.total_base, pa.total_trx + dra.rental_count + ma.member_count,
        pa.total_disc, pa.total_auto,
        pa.voucher_count + (SELECT COUNT(*)::BIGINT FROM daily_rentals WHERE voucher_id IS NOT NULL AND created_at::DATE = p_date),
        pa.total_cash, pa.total_chg,
        (SELECT COUNT(*)::INT FROM sessions WHERE status = 'active'),
        (SELECT COUNT(*)::INT FROM consoles WHERE status = 'available'),
        (SELECT COUNT(*)::INT FROM consoles),
        dra.rental_rev, dra.rental_count,
        ma.member_rev, ma.member_count,
        COALESCE((SELECT jsonb_agg(row_to_json(va)) FROM voucher_agg va), '[]'::JSONB)
    INTO
        total_revenue, total_base_amount, total_transactions,
        total_discount, total_auto_discount, voucher_usage_count,
        total_cash_received, total_change,
        active_sessions, available_consoles, total_consoles,
        daily_rental_revenue, daily_rental_count,
        membership_revenue, membership_count,
        voucher_details
    FROM payment_agg pa
    CROSS JOIN daily_rental_agg dra
    CROSS JOIN membership_agg ma;
    RETURN NEXT;
END;
$$;
