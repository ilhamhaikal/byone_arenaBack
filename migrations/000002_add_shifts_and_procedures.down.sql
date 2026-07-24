-- =============================================================================
-- 000002_procedures.down.sql — Drop semua stored procedures & functions
-- =============================================================================

-- Session & Payment SPs
DROP FUNCTION IF EXISTS "__SP__StartSession"(UUID, UUID, TEXT, INTEGER);
DROP FUNCTION IF EXISTS "__SP__EndSession"(UUID);
DROP FUNCTION IF EXISTS "__SP__CancelSession"(UUID);
DROP FUNCTION IF EXISTS "__SP__CreatePayment"(UUID, NUMERIC, TEXT, VARCHAR);
DROP FUNCTION IF EXISTS "__SP__RefundPayment"(UUID);
DROP FUNCTION IF EXISTS "__SP__ValidateKasirShift"(UUID);
DROP FUNCTION IF EXISTS "__SP__StartSessionWithPayment"(UUID, UUID, TEXT, INTEGER, NUMERIC, VARCHAR);
DROP FUNCTION IF EXISTS "__SP__ExtendSession"(UUID, INTEGER, NUMERIC, BOOLEAN, VARCHAR, TEXT);
DROP FUNCTION IF EXISTS "__SP__ConfirmExtendPayment"(UUID);

-- Voucher & Discount SPs
DROP FUNCTION IF EXISTS "__SP__ApplyVoucher"(VARCHAR, NUMERIC, NUMERIC, INT);
DROP FUNCTION IF EXISTS "__SP__EvaluateDiscountRules"(NUMERIC, TIMESTAMPTZ, BOOLEAN);
DROP FUNCTION IF EXISTS "__SP__CalculatePrice"(UUID, INTEGER);
DROP FUNCTION IF EXISTS "__SP__PreviewPrice"(UUID, INTEGER, VARCHAR, UUID);

-- Food Order SPs
DROP FUNCTION IF EXISTS "generate_food_order_number"();
DROP FUNCTION IF EXISTS "__SP__CreateFoodOrder"(UUID, UUID, TEXT, JSONB);
DROP FUNCTION IF EXISTS "__SP__UpdateFoodOrderStatus"(UUID, VARCHAR);

-- Daily Rental SPs
DROP FUNCTION IF EXISTS "__SP__CreateDailyRental"(UUID, UUID, DATE, DATE, NUMERIC, VARCHAR, TEXT);
DROP FUNCTION IF EXISTS "__SP__ReturnDailyRental"(UUID);

-- Booking SPs
DROP FUNCTION IF EXISTS "__SP__CreateBooking"(UUID, UUID, DATE, INT, INT, INT, TEXT);

-- Membership SPs
DROP FUNCTION IF EXISTS "__SP__SellMembership"(UUID, NUMERIC);

-- Dashboard & Report SPs
DROP FUNCTION IF EXISTS "__SP__DashboardSummary"(DATE);
DROP FUNCTION IF EXISTS "__SP__ReportSummary"(DATE, DATE);

-- TV Activity SPs
DROP FUNCTION IF EXISTS "__SP__LogTvActivity"(UUID, VARCHAR, UUID);
DROP FUNCTION IF EXISTS "__SP__GetTvLogs"(UUID, DATE);

-- Cleanup any leftover overloads
DROP FUNCTION IF EXISTS "__SP__StartSession"(UUID, UUID, TEXT);
DROP FUNCTION IF EXISTS "__SP__CreatePayment"(UUID, NUMERIC, TEXT);
DROP FUNCTION IF EXISTS "__SP__CreateDailyRental"(UUID, UUID, DATE, DATE, NUMERIC, NUMERIC, TEXT);
DROP FUNCTION IF EXISTS "__SP__CreateDailyRental"(UUID, UUID, DATE, DATE, NUMERIC, NUMERIC, VARCHAR, TEXT);
DROP FUNCTION IF EXISTS "__SP__CreateDailyRental"(UUID, UUID, DATE, DATE, NUMERIC, NUMERIC, VARCHAR);
DROP FUNCTION IF EXISTS "__SP__CreateDailyRental"(UUID, UUID, DATE, DATE, NUMERIC, VARCHAR, TEXT);
DROP FUNCTION IF EXISTS "__SP__ApplyVoucher"(VARCHAR, NUMERIC);
DROP FUNCTION IF EXISTS "__SP__ApplyVoucher"(VARCHAR, NUMERIC, NUMERIC, INT);
DROP FUNCTION IF EXISTS "__SP__ExtendSession"(UUID, INTEGER, NUMERIC, VARCHAR, TEXT);
DROP FUNCTION IF EXISTS "__SP__GetTvLogs"(UUID, DATE);
