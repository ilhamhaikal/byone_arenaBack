-- =============================================================================
-- 000032_license_offline.down.sql
-- =============================================================================

DROP FUNCTION IF EXISTS "__SP__LogLicenseEvent"(VARCHAR, VARCHAR, JSONB);
DROP FUNCTION IF EXISTS "__SP__LicenseHistory"(INT);
DROP FUNCTION IF EXISTS "__SP__CountLicensedUnits"();
DROP FUNCTION IF EXISTS "__SP__GetActiveLicense"();
DROP FUNCTION IF EXISTS "__SP__SaveLicense"(TEXT, VARCHAR, VARCHAR, VARCHAR, INT, TIMESTAMPTZ, TIMESTAMPTZ, UUID, JSONB);
DROP FUNCTION IF EXISTS "__SP__GetOrCreateServerIdentity"();

DROP TABLE IF EXISTS license_events;
DROP TABLE IF EXISTS licenses;
DROP TABLE IF EXISTS server_identity;
