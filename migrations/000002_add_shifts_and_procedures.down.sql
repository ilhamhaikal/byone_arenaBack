-- Migration: 000002_add_shifts_and_procedures.down.sql

DROP FUNCTION IF EXISTS sp_validate_kasir_shift(UUID);
DROP FUNCTION IF EXISTS sp_refund_payment(UUID);
DROP FUNCTION IF EXISTS sp_create_payment(UUID, NUMERIC, TEXT);
DROP FUNCTION IF EXISTS sp_cancel_session(UUID);
DROP FUNCTION IF EXISTS sp_end_session(UUID);
DROP FUNCTION IF EXISTS sp_start_session(UUID, UUID, TEXT);

ALTER TABLE payments DROP COLUMN IF EXISTS cash_received;
ALTER TABLE payments DROP COLUMN IF EXISTS change_amount;

DROP TABLE IF EXISTS shifts;
