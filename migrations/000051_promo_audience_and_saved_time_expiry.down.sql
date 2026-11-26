-- Rollback 000051.
-- Kembalikan InterruptedSessions ke versi 000050 (tanpa cleanup otomatis).

DROP FUNCTION IF EXISTS "__SP__ExpireSavedTimeSessions"();

DROP FUNCTION IF EXISTS "__SP__InterruptedSessions"(INT);

CREATE OR REPLACE FUNCTION "__SP__InterruptedSessions"(p_limit INT DEFAULT 20)
RETURNS TABLE (
    session_id UUID,
    console_id UUID,
    console_name VARCHAR,
    customer_name VARCHAR,
    remaining_minutes INT,
    interrupted_at TIMESTAMPTZ,
    reason TEXT,
    notes TEXT,
    saved_time_expires_at TIMESTAMPTZ,
    is_package BOOLEAN
)
LANGUAGE sql AS $$
    SELECT
        s.id AS session_id,
        s.console_id,
        c.name AS console_name,
        COALESCE(cust.name, 'Umum') AS customer_name,
        s.remaining_minutes,
        s.interrupted_at,
        COALESCE(s.interrupted_reason, '') AS reason,
        COALESCE(s.notes, '') AS notes,
        s.saved_time_expires_at,
        s.is_package
    FROM sessions s
    JOIN consoles c ON c.id = s.console_id
    LEFT JOIN customers cust ON cust.id = s.customer_id
    WHERE s.status = 'interrupted'
      AND s.remaining_minutes > 0
      AND (s.saved_time_expires_at IS NULL OR s.saved_time_expires_at > NOW())
    ORDER BY s.interrupted_at DESC NULLS LAST
    LIMIT GREATEST(1, LEAST(COALESCE(p_limit, 20), 100));
$$;

DELETE FROM promo_bonus_rules
 WHERE name IN ('Bonus Weekend Umum', 'Bonus Weekend Member');

DROP INDEX IF EXISTS idx_promo_bonus_rules_audience;

ALTER TABLE promo_bonus_rules
    DROP CONSTRAINT IF EXISTS promo_bonus_rules_audience_check;

ALTER TABLE promo_bonus_rules
    DROP COLUMN IF EXISTS audience;