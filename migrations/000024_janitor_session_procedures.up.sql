-- =============================================================================
-- 000024_janitor_session_procedures.up.sql
--
-- Memindahkan query pemilihan sesi oleh janitor ke STORED PROCEDURE,
-- agar logika cukup diperbaiki di procedure tanpa build ulang aplikasi.
-- =============================================================================

-- 1. Sesi aktif yang sudah melewati jadwal selesai (untuk auto-end).
CREATE OR REPLACE FUNCTION "__SP__ExpiredActiveSessions"(p_limit INT DEFAULT 200)
RETURNS TABLE(id UUID)
LANGUAGE plpgsql AS $$
BEGIN
    IF p_limit IS NULL OR p_limit <= 0 THEN p_limit := 200; END IF;

    RETURN QUERY
    SELECT s.id
      FROM sessions s
     WHERE s.status = 'active'
       AND s.end_scheduled_at IS NOT NULL
       AND s.end_scheduled_at <= NOW()
     LIMIT p_limit;
END;
$$;

-- 2. Sesi terlantar: telat > 6 jam dan konsol tidak mengirim heartbeat > 30 menit.
CREATE OR REPLACE FUNCTION "__SP__StaleActiveSessions"(p_limit INT DEFAULT 100)
RETURNS TABLE(id UUID)
LANGUAGE plpgsql AS $$
BEGIN
    IF p_limit IS NULL OR p_limit <= 0 THEN p_limit := 100; END IF;

    RETURN QUERY
    SELECT s.id
      FROM sessions s
      JOIN consoles c ON c.id = s.console_id
     WHERE s.status = 'active'
       AND s.end_scheduled_at IS NOT NULL
       AND s.end_scheduled_at < NOW() - INTERVAL '6 hours'
       AND (c.last_seen_at IS NULL
            OR c.last_seen_at < NOW() - INTERVAL '30 minutes')
     LIMIT p_limit;
END;
$$;
