-- Migration: 000009_add_dashboard_summary_sp.up.sql
-- Stored procedure untuk ringkasan dashboard + detail penggunaan voucher

CREATE OR REPLACE FUNCTION sp_dashboard_summary(p_date DATE DEFAULT CURRENT_DATE)
RETURNS TABLE (
    total_revenue       NUMERIC,
    total_base_amount   NUMERIC,
    total_transactions  BIGINT,
    total_discount      NUMERIC,
    total_auto_discount NUMERIC,
    voucher_usage_count BIGINT,
    total_cash_received NUMERIC,
    total_change        NUMERIC,
    active_sessions     INT,
    available_consoles  INT,
    total_consoles      INT,
    voucher_details     JSONB
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_active_sessions    INT;
    v_available_consoles INT;
    v_total_consoles     INT;
    v_voucher_json       JSONB;
BEGIN
    -- Hitung ringkasan pembayaran untuk tanggal yang diminta
    -- (gunakan temporary table untuk hold hasil agregasi)
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
        WHERE p.payment_status = 'paid'
          AND p.paid_at::DATE = p_date
    ),
    -- Detail per voucher yang digunakan hari ini
    voucher_agg AS (
        SELECT
            v.name       AS voucher_name,
            v.code       AS voucher_code,
            COUNT(*)     AS usage_count,
            SUM(p.discount_amount) AS total_discount
        FROM payments p
        JOIN vouchers v ON v.id = p.voucher_id
        WHERE p.payment_status = 'paid'
          AND p.paid_at::DATE = p_date
          AND p.voucher_id IS NOT NULL
        GROUP BY v.id, v.name, v.code
        ORDER BY usage_count DESC
    )
    SELECT
        pa.total_rev,
        pa.total_base,
        pa.total_trx,
        pa.total_disc,
        pa.total_auto,
        pa.voucher_count,
        pa.total_cash,
        pa.total_chg,
        -- Ambil dari outer query
        (SELECT COUNT(*)::INT FROM sessions WHERE status = 'active'),
        (SELECT COUNT(*)::INT FROM consoles WHERE status = 'available'),
        (SELECT COUNT(*)::INT FROM consoles),
        -- Konversi voucher detail ke JSON array
        COALESCE(
            (SELECT jsonb_agg(row_to_json(va))
             FROM voucher_agg va),
            '[]'::JSONB
        )
    INTO
        total_revenue, total_base_amount, total_transactions,
        total_discount, total_auto_discount, voucher_usage_count,
        total_cash_received, total_change,
        active_sessions, available_consoles, total_consoles,
        voucher_details
    FROM payment_agg pa;

    RETURN NEXT;
END;
$$;
