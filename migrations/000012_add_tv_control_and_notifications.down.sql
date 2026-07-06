-- Migration: 000012_add_tv_control_and_notifications.down.sql
DROP TABLE IF EXISTS tv_notifications;
ALTER TABLE consoles DROP COLUMN IF EXISTS screen_status;
ALTER TABLE consoles DROP COLUMN IF EXISTS adb_port;
ALTER TABLE consoles DROP COLUMN IF EXISTS mac_address;
