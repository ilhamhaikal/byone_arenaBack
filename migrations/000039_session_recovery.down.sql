-- =============================================================================
-- 000039_session_recovery.down.sql
-- =============================================================================

DROP FUNCTION IF EXISTS "__SP__InterruptSession"(UUID, TEXT);

DROP INDEX IF EXISTS idx_sessions_interrupted;

ALTER TABLE sessions
    DROP COLUMN IF EXISTS interrupted_at,
    DROP COLUMN IF EXISTS remaining_minutes,
    DROP COLUMN IF EXISTS interrupted_reason,
    DROP COLUMN IF EXISTS recovered_by_session_id,
    DROP COLUMN IF EXISTS recovered_from_session_id;
