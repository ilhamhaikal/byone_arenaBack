-- Migration: 000014_add_extend_session_and_auto_stop.down.sql
DROP FUNCTION IF EXISTS "byoneConfirmExtendPayment"(UUID);
DROP FUNCTION IF EXISTS "byoneExtendSession"(UUID, INTEGER, NUMERIC, VARCHAR, TEXT);
ALTER TABLE payments ADD CONSTRAINT payments_session_id_key UNIQUE(session_id);
