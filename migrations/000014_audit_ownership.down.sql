DROP INDEX IF EXISTS idx_audit_logs_action;
DROP INDEX IF EXISTS idx_audit_logs_user_id;
DROP INDEX IF EXISTS idx_audit_logs_created_at;
DROP TABLE IF EXISTS audit_logs;

DROP INDEX IF EXISTS idx_payments_created_by;
DROP INDEX IF EXISTS idx_sessions_created_by;

ALTER TABLE payments DROP COLUMN IF EXISTS refunded_at;
ALTER TABLE payments DROP COLUMN IF EXISTS refunded_by;
ALTER TABLE payments DROP COLUMN IF EXISTS created_by;

ALTER TABLE sessions DROP COLUMN IF EXISTS cancelled_by;
ALTER TABLE sessions DROP COLUMN IF EXISTS ended_by;
ALTER TABLE sessions DROP COLUMN IF EXISTS created_by;