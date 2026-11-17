-- =============================================================================
-- 000026_console_payment_procedures.down.sql
-- =============================================================================

DROP FUNCTION IF EXISTS "__SP__ConsolePricePerHour"(UUID);
DROP FUNCTION IF EXISTS "__SP__PendingPaymentIDs"(UUID);
DROP FUNCTION IF EXISTS "__SP__SetPaymentRefundedBy"(UUID, UUID);
DROP FUNCTION IF EXISTS "__SP__FinalizePaymentMethod"(UUID, VARCHAR);
