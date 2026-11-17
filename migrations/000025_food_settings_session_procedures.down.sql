-- =============================================================================
-- 000025_food_settings_session_procedures.down.sql
-- =============================================================================

DROP FUNCTION IF EXISTS "__SP__SetFoodOrderPaymentMethod"(UUID, VARCHAR);
DROP FUNCTION IF EXISTS "__SP__MenuItemPrice"(UUID);
DROP FUNCTION IF EXISTS "__SP__CustomerIsMember"(UUID);
DROP FUNCTION IF EXISTS "__SP__AppSettingValue"(TEXT);
DROP FUNCTION IF EXISTS "__SP__SetAppSetting"(TEXT, TEXT);
DROP FUNCTION IF EXISTS "__SP__SetSessionCreatedBy"(UUID, UUID);
DROP FUNCTION IF EXISTS "__SP__SetPaymentCreatedBy"(UUID, UUID);
DROP FUNCTION IF EXISTS "__SP__SetSessionEndedBy"(UUID, UUID);
DROP FUNCTION IF EXISTS "__SP__SetSessionCancelledBy"(UUID, UUID);
DROP FUNCTION IF EXISTS "__SP__CountPendingPayments"(UUID);
DROP FUNCTION IF EXISTS "__SP__SessionConsoleID"(UUID);
DROP FUNCTION IF EXISTS "__SP__DeactivateTvNotification"(TEXT, TEXT);
DROP FUNCTION IF EXISTS "__SP__InsertTvNotification"(TEXT, TEXT, TEXT, BOOLEAN, VARCHAR);
