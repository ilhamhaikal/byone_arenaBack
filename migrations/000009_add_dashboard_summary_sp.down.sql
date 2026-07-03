-- Migration: 000009_add_dashboard_summary_sp.down.sql
DROP FUNCTION IF EXISTS sp_dashboard_summary(DATE);
