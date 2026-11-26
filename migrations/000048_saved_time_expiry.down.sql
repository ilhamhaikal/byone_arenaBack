DROP FUNCTION IF EXISTS "__SP__SaveNonMemberTime"(UUID, TEXT);
DROP FUNCTION IF EXISTS "__SP__InterruptSession"(UUID, TEXT);
DROP FUNCTION IF EXISTS "__SP__InterruptedSessions"(INT);
DROP INDEX IF EXISTS idx_sessions_saved_time_expires;
ALTER TABLE sessions DROP COLUMN IF EXISTS saved_time_expires_at;