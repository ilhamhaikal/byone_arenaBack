-- =============================================================================
-- 000024_janitor_session_procedures.down.sql
-- =============================================================================

DROP FUNCTION IF EXISTS "__SP__ExpiredActiveSessions"(INT);
DROP FUNCTION IF EXISTS "__SP__StaleActiveSessions"(INT);
