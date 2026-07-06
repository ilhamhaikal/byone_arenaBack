-- Migration: 000011_add_report_summary_sp.down.sql
DROP FUNCTION IF EXISTS "byoneReportSummary"(DATE, DATE);
