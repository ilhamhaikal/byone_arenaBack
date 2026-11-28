-- 000062_license_online.down.sql
DROP FUNCTION IF EXISTS "__SP__SetLicenseHeartbeatError"(TEXT, TIMESTAMPTZ);
DROP FUNCTION IF EXISTS "__SP__SetLicenseRemoteState"(VARCHAR, TIMESTAMPTZ, TEXT);
DROP FUNCTION IF EXISTS "__SP__UpdateLicenseHeartbeat"(TEXT, VARCHAR, VARCHAR, VARCHAR, INT, TIMESTAMPTZ, TIMESTAMPTZ, JSONB, TIMESTAMPTZ);
DROP FUNCTION IF EXISTS "__SP__UpdateLicenseActivationRequest"(UUID, VARCHAR, TIMESTAMPTZ);
DROP FUNCTION IF EXISTS "__SP__GetPendingLicenseActivationRequest"();
DROP FUNCTION IF EXISTS "__SP__StartLicenseActivationRequest"(VARCHAR, TEXT, TIMESTAMPTZ);

DROP TABLE IF EXISTS license_activation_requests;

DROP FUNCTION IF EXISTS "__SP__GetActiveLicense"();
CREATE OR REPLACE FUNCTION "__SP__GetActiveLicense"()
RETURNS TABLE(
    id UUID, license_key TEXT, license_id VARCHAR, customer VARCHAR,
    plan VARCHAR, max_units INT, issued_at TIMESTAMPTZ, expires_at TIMESTAMPTZ,
    instance_id UUID, activated_at TIMESTAMPTZ, is_active BOOLEAN, raw_payload JSONB
)
LANGUAGE sql AS $$
    SELECT l.id, l.license_key, l.license_id, l.customer, l.plan, l.max_units,
           l.issued_at, l.expires_at, l.instance_id, l.activated_at,
           l.is_active, l.raw_payload
      FROM licenses l
     WHERE l.is_active = TRUE
     ORDER BY l.activated_at DESC
     LIMIT 1;
$$;

ALTER TABLE licenses
    DROP COLUMN IF EXISTS last_heartbeat_error,
    DROP COLUMN IF EXISTS last_heartbeat_at,
    DROP COLUMN IF EXISTS remote_state;