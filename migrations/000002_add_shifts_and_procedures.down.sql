-- =============================================================================
-- 000002_procedures.down.sql — Drop semua stored procedures & functions
-- =============================================================================

-- Session & Payment SPs
DROP FUNCTION IF EXISTS "byoneStartSession"(UUID, UUID, TEXT, INTEGER);
DROP FUNCTION IF EXISTS "byoneEndSession"(UUID);
DROP FUNCTION IF EXISTS "byoneCancelSession"(UUID);
DROP FUNCTION IF EXISTS "byoneCreatePayment"(UUID, NUMERIC, TEXT, VARCHAR);
DROP FUNCTION IF EXISTS "byoneRefundPayment"(UUID);
DROP FUNCTION IF EXISTS "byoneValidateKasirShift"(UUID);
DROP FUNCTION IF EXISTS "byoneStartSessionWithPayment"(UUID, UUID, TEXT, INTEGER, NUMERIC, VARCHAR);
DROP FUNCTION IF EXISTS "byoneExtendSession"(UUID, INTEGER, NUMERIC, BOOLEAN, VARCHAR, TEXT);
DROP FUNCTION IF EXISTS "byoneConfirmExtendPayment"(UUID);

-- Voucher & Discount SPs
DROP FUNCTION IF EXISTS "byoneApplyVoucher"(VARCHAR, NUMERIC, NUMERIC, INT);
DROP FUNCTION IF EXISTS "byoneEvaluateDiscountRules"(NUMERIC, TIMESTAMPTZ, BOOLEAN);
DROP FUNCTION IF EXISTS "byoneCalculatePrice"(UUID, INTEGER);
DROP FUNCTION IF EXISTS "byonePreviewPrice"(UUID, INTEGER, VARCHAR, UUID);

-- Food Order SPs
DROP FUNCTION IF EXISTS "generate_food_order_number"();
DROP FUNCTION IF EXISTS "byoneCreateFoodOrder"(UUID, UUID, TEXT, JSONB);
DROP FUNCTION IF EXISTS "byoneUpdateFoodOrderStatus"(UUID, VARCHAR);

-- Daily Rental SPs
DROP FUNCTION IF EXISTS "byoneCreateDailyRental"(UUID, UUID, DATE, DATE, NUMERIC, VARCHAR, TEXT);
DROP FUNCTION IF EXISTS "byoneReturnDailyRental"(UUID);

-- Booking SPs
DROP FUNCTION IF EXISTS "byoneCreateBooking"(UUID, UUID, DATE, INT, INT, INT, TEXT);

-- Membership SPs
DROP FUNCTION IF EXISTS "byoneSellMembership"(UUID, NUMERIC);

-- Dashboard & Report SPs
DROP FUNCTION IF EXISTS "byoneDashboardSummary"(DATE);
DROP FUNCTION IF EXISTS "byoneReportSummary"(DATE, DATE);

-- TV Activity SPs
DROP FUNCTION IF EXISTS "byoneLogTvActivity"(UUID, VARCHAR, UUID);
DROP FUNCTION IF EXISTS "byoneGetTvLogs"(UUID, DATE);

-- Cleanup any leftover overloads
DROP FUNCTION IF EXISTS "byoneStartSession"(UUID, UUID, TEXT);
DROP FUNCTION IF EXISTS "byoneCreatePayment"(UUID, NUMERIC, TEXT);
DROP FUNCTION IF EXISTS "byoneCreateDailyRental"(UUID, UUID, DATE, DATE, NUMERIC, NUMERIC, TEXT);
DROP FUNCTION IF EXISTS "byoneCreateDailyRental"(UUID, UUID, DATE, DATE, NUMERIC, NUMERIC, VARCHAR, TEXT);
DROP FUNCTION IF EXISTS "byoneCreateDailyRental"(UUID, UUID, DATE, DATE, NUMERIC, NUMERIC, VARCHAR);
DROP FUNCTION IF EXISTS "byoneCreateDailyRental"(UUID, UUID, DATE, DATE, NUMERIC, VARCHAR, TEXT);
DROP FUNCTION IF EXISTS "byoneApplyVoucher"(VARCHAR, NUMERIC);
DROP FUNCTION IF EXISTS "byoneApplyVoucher"(VARCHAR, NUMERIC, NUMERIC, INT);
DROP FUNCTION IF EXISTS "byoneExtendSession"(UUID, INTEGER, NUMERIC, VARCHAR, TEXT);
DROP FUNCTION IF EXISTS "byoneGetTvLogs"(UUID, DATE);
