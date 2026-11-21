-- =============================================================================
-- 000046_staff_reporting.up.sql
--
-- Fondasi laporan per petugas:
--   - auth_sessions: sesi login–logout per user
--   - created_by pada food_orders & daily_rentals
--   - stored procedure agregasi pendapatan harian per petugas
-- =============================================================================

CREATE TABLE IF NOT EXISTS auth_sessions (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    jti         VARCHAR(64) NOT NULL UNIQUE,
    login_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    logout_at   TIMESTAMPTZ,
    expires_at  TIMESTAMPTZ,
    ip          VARCHAR(50) NOT NULL DEFAULT '',
    user_agent  VARCHAR(255) NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_user_login
    ON auth_sessions (user_id, login_at DESC);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_login
    ON auth_sessions (login_at DESC);

ALTER TABLE food_orders
    ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE daily_rentals
    ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_food_orders_created_by
    ON food_orders (created_by);
CREATE INDEX IF NOT EXISTS idx_daily_rentals_created_by
    ON daily_rentals (created_by);

-- Pendapatan harian per petugas. user_id NULL = tidak diketahui.
DROP FUNCTION IF EXISTS "__SP__StaffDailyRevenue"(DATE, DATE, UUID);

CREATE OR REPLACE FUNCTION "__SP__StaffDailyRevenue"(
    p_start_date DATE,
    p_end_date DATE,
    p_user_id UUID DEFAULT NULL
)
RETURNS TABLE (
    user_id UUID,
    report_date DATE,
    rental_revenue NUMERIC,
    food_revenue NUMERIC,
    daily_rental_revenue NUMERIC,
    membership_revenue NUMERIC,
    rental_trx INT,
    food_trx INT,
    daily_rental_trx INT,
    membership_trx INT
)
LANGUAGE sql AS $$
WITH src AS (
    -- Rental sesi konsol
    SELECT p.created_by AS user_id,
           p.paid_at::DATE AS report_date,
           p.total_payment AS rental_revenue,
           0::NUMERIC AS food_revenue,
           0::NUMERIC AS daily_rental_revenue,
           0::NUMERIC AS membership_revenue,
           1 AS rental_trx,
           0 AS food_trx,
           0 AS daily_rental_trx,
           0 AS membership_trx
      FROM payments p
     WHERE p.payment_status = 'paid'
       AND p.session_id IS NOT NULL
       AND p.paid_at::DATE BETWEEN p_start_date AND p_end_date

    UNION ALL

    -- Membership (payment tanpa sesi)
    SELECT p.created_by,
           p.paid_at::DATE,
           0, 0, 0,
           p.total_payment,
           0, 0, 0, 1
      FROM payments p
     WHERE p.payment_status = 'paid'
       AND p.session_id IS NULL
       AND p.paid_at::DATE BETWEEN p_start_date AND p_end_date

    UNION ALL

    -- Makanan: hanya yang served, diatribusikan ke pembuat order
    SELECT fo.created_by,
           fo.created_at::DATE,
           0,
           GREATEST(fo.total_amount - COALESCE(fo.discount_amount, 0), 0),
           0, 0,
           0, 1, 0, 0
      FROM food_orders fo
     WHERE fo.status = 'served'
       AND fo.created_at::DATE BETWEEN p_start_date AND p_end_date

    UNION ALL

    -- Rental harian (bukan cancelled)
    SELECT dr.created_by,
           dr.created_at::DATE,
           0, 0,
           dr.final_amount,
           0,
           0, 0, 1, 0
      FROM daily_rentals dr
     WHERE dr.status <> 'cancelled'
       AND dr.created_at::DATE BETWEEN p_start_date AND p_end_date
)
SELECT user_id,
       report_date,
       COALESCE(SUM(rental_revenue), 0)::NUMERIC(12,2),
       COALESCE(SUM(food_revenue), 0)::NUMERIC(12,2),
       COALESCE(SUM(daily_rental_revenue), 0)::NUMERIC(12,2),
       COALESCE(SUM(membership_revenue), 0)::NUMERIC(12,2),
       COALESCE(SUM(rental_trx), 0)::INT,
       COALESCE(SUM(food_trx), 0)::INT,
       COALESCE(SUM(daily_rental_trx), 0)::INT,
       COALESCE(SUM(membership_trx), 0)::INT
  FROM src
 WHERE p_user_id IS NULL OR user_id = p_user_id
 GROUP BY user_id, report_date
 ORDER BY report_date, user_id;
$$;

-- Sesi login per user. Durasi memakai logout_at, atau expires_at/NOW jika
-- belum logout.
DROP FUNCTION IF EXISTS "__SP__StaffLoginSessions"(DATE, DATE, UUID);

CREATE OR REPLACE FUNCTION "__SP__StaffLoginSessions"(
    p_start_date DATE,
    p_end_date DATE,
    p_user_id UUID DEFAULT NULL
)
RETURNS TABLE (
    user_id UUID,
    login_at TIMESTAMPTZ,
    logout_at TIMESTAMPTZ,
    duration_minutes INT,
    still_logged_in BOOLEAN
)
LANGUAGE sql AS $$
    SELECT s.user_id,
           s.login_at,
           s.logout_at,
           GREATEST(
               FLOOR(
                   EXTRACT(EPOCH FROM (
                       COALESCE(s.logout_at, LEAST(NOW(), COALESCE(s.expires_at, NOW())))
                       - s.login_at
                   )) / 60
               )::INT,
               0
           ) AS duration_minutes,
           s.logout_at IS NULL AS still_logged_in
      FROM auth_sessions s
     WHERE s.login_at::DATE BETWEEN p_start_date AND p_end_date
       AND (p_user_id IS NULL OR s.user_id = p_user_id)
     ORDER BY s.login_at;
$$;
