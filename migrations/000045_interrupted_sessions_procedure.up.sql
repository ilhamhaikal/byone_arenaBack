-- Daftar sesi terputus yang masih bisa dipulihkan (sisa waktu > 0).
-- Dipakai dashboard; logika query disimpan di stored procedure agar
-- perubahan aturan tidak perlu mengubah/menyebar build backend.
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
    notes TEXT
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
        COALESCE(s.notes, '') AS notes
    FROM sessions s
    JOIN consoles c ON c.id = s.console_id
    LEFT JOIN customers cust ON cust.id = s.customer_id
    WHERE s.status = 'interrupted'
      AND s.remaining_minutes > 0
    ORDER BY s.interrupted_at DESC NULLS LAST
    LIMIT GREATEST(1, LEAST(COALESCE(p_limit, 20), 100));
$$;
