-- =============================================================================
-- 000040_session_status_interrupted.up.sql
-- Tambahkan status interrupted/recovered ke constraint sessions.status.
-- =============================================================================

ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_status_check;

ALTER TABLE sessions
    ADD CONSTRAINT sessions_status_check
    CHECK (status IN ('active', 'completed', 'cancelled', 'interrupted', 'recovered'));
