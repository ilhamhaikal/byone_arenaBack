-- =============================================================================
-- 000027_rental_promo_procedures.down.sql
-- =============================================================================

DROP FUNCTION IF EXISTS "__SP__ConsoleDailyPrice"(UUID);
DROP FUNCTION IF EXISTS "__SP__SetDailyRentalPaymentMethod"(UUID, VARCHAR);
DROP FUNCTION IF EXISTS "__SP__SetFoodOrderDiscount"(UUID, NUMERIC);
DROP FUNCTION IF EXISTS "__SP__PromoBonusReport"(DATE, DATE);
