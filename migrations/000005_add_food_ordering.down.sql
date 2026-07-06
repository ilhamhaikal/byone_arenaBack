-- Migration: 000005_add_food_ordering.down.sql

DROP FUNCTION IF EXISTS byoneUpdateFoodOrderStatus(UUID, VARCHAR);
DROP FUNCTION IF EXISTS byoneCreateFoodOrder(UUID, UUID, TEXT, JSONB);
DROP FUNCTION IF EXISTS generate_food_order_number();
DROP SEQUENCE IF EXISTS food_order_daily_seq;

DROP TABLE IF EXISTS food_order_items;
DROP TABLE IF EXISTS food_orders;
DROP TABLE IF EXISTS menu_items;
