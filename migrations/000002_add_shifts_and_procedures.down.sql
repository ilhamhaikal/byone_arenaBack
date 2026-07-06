-- Migration: 000002_add_shifts_and_procedures.down.sql

DROP FUNCTION IF EXISTS byoneValidateKasirShift(UUID);
DROP FUNCTION IF EXISTS byoneRefundPayment(UUID);
DROP FUNCTION IF EXISTS byoneCreatePayment(UUID, NUMERIC, TEXT);
DROP FUNCTION IF EXISTS byoneCancelSession(UUID);
DROP FUNCTION IF EXISTS byoneEndSession(UUID);
DROP FUNCTION IF EXISTS byoneStartSession(UUID, UUID, TEXT);

ALTER TABLE payments DROP COLUMN IF EXISTS cash_received;
ALTER TABLE payments DROP COLUMN IF EXISTS change_amount;

DROP TABLE IF EXISTS shifts;
