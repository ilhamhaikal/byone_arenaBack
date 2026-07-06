-- Migration: 000010_rename_procedures_camelcase.down.sql
ALTER FUNCTION "byoneApplyVoucher"(VARCHAR, NUMERIC) RENAME TO sp_apply_voucher;
ALTER FUNCTION "byoneCancelSession"(UUID) RENAME TO sp_cancel_session;
ALTER FUNCTION "byoneCreateFoodOrder"(UUID, UUID, TEXT, JSONB) RENAME TO sp_create_food_order;
ALTER FUNCTION "byoneCreatePayment"(UUID, NUMERIC, TEXT, VARCHAR) RENAME TO sp_create_payment;
ALTER FUNCTION "byoneDashboardSummary"(DATE) RENAME TO sp_dashboard_summary;
ALTER FUNCTION "byoneEndSession"(UUID) RENAME TO sp_end_session;
ALTER FUNCTION "byoneEvaluateDiscountRules"(NUMERIC, TIMESTAMPTZ, BOOLEAN) RENAME TO sp_evaluate_discount_rules;
ALTER FUNCTION "byoneRefundPayment"(UUID) RENAME TO sp_refund_payment;
ALTER FUNCTION "byoneStartSession"(UUID, UUID, TEXT, INTEGER) RENAME TO sp_start_session;
ALTER FUNCTION "byoneStartSessionWithPayment"(UUID, UUID, TEXT, INTEGER, NUMERIC, VARCHAR) RENAME TO sp_start_session_with_payment;
ALTER FUNCTION "byoneUpdateFoodOrderStatus"(UUID, VARCHAR) RENAME TO sp_update_food_order_status;
