-- Migration: 000010_rename_procedures_camelcase.up.sql
-- Rename semua stored procedure dari snake_case (sp_xxx) ke camelCase (byoneXxx)

ALTER FUNCTION sp_apply_voucher(VARCHAR, NUMERIC) RENAME TO "byoneApplyVoucher";
ALTER FUNCTION sp_cancel_session(UUID) RENAME TO "byoneCancelSession";
ALTER FUNCTION sp_create_food_order(UUID, UUID, TEXT, JSONB) RENAME TO "byoneCreateFoodOrder";
ALTER FUNCTION sp_create_payment(UUID, NUMERIC, TEXT, VARCHAR) RENAME TO "byoneCreatePayment";
ALTER FUNCTION sp_dashboard_summary(DATE) RENAME TO "byoneDashboardSummary";
ALTER FUNCTION sp_end_session(UUID) RENAME TO "byoneEndSession";
ALTER FUNCTION sp_evaluate_discount_rules(NUMERIC, TIMESTAMPTZ, BOOLEAN) RENAME TO "byoneEvaluateDiscountRules";
ALTER FUNCTION sp_refund_payment(UUID) RENAME TO "byoneRefundPayment";
ALTER FUNCTION sp_start_session(UUID, UUID, TEXT, INTEGER) RENAME TO "byoneStartSession";
ALTER FUNCTION sp_start_session_with_payment(UUID, UUID, TEXT, INTEGER, NUMERIC, VARCHAR) RENAME TO "byoneStartSessionWithPayment";
ALTER FUNCTION sp_update_food_order_status(UUID, VARCHAR) RENAME TO "byoneUpdateFoodOrderStatus";

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_proc WHERE proname = 'sp_validate_kasir_shift') THEN
        EXECUTE 'ALTER FUNCTION sp_validate_kasir_shift(UUID) RENAME TO "byoneValidateKasirShift"';
    END IF;
END;
$$;
