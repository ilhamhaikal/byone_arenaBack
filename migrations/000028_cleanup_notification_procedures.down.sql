-- =============================================================================
-- 000028_cleanup_notification_procedures.down.sql
-- =============================================================================

DROP FUNCTION IF EXISTS "__SP__DeactivateTvNotificationByTitle"(TEXT);
DROP FUNCTION IF EXISTS "__SP__DeactivateStalePendingPaymentNotifications"();
