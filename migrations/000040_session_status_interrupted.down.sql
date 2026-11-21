-- =============================================================================
-- 000040_session_status_interrupted.down.sql
-- =============================================================================

ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_status_check;

ALTER TABLE sessions
    ADD CONSTRAINT sessions_status_check
    CHECK (status IN ('active', 'completed', 'cancelled'));
