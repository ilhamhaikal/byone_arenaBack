-- Migration: 000005_add_food_ordering.down.sql

DROP FUNCTION IF EXISTS sp_update_food_order_status(UUID, VARCHAR);
DROP FUNCTION IF EXISTS sp_create_food_order(UUID, UUID, TEXT, JSONB);
DROP FUNCTION IF EXISTS generate_food_order_number();
DROP SEQUENCE IF EXISTS food_order_daily_seq;

DROP TABLE IF EXISTS food_order_items;
DROP TABLE IF EXISTS food_orders;
DROP TABLE IF EXISTS menu_items;
