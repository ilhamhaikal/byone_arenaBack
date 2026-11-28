-- 000064_license_active_v2.up.sql
-- Jaga kompatibilitas rolling deploy: binary lama tetap memakai
-- GetActiveLicense (12 kolom), binary baru memakai GetActiveLicenseV2.
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

DROP FUNCTION IF EXISTS "__SP__GetActiveLicenseV2"();
CREATE OR REPLACE FUNCTION "__SP__GetActiveLicenseV2"()
RETURNS TABLE(
    id UUID, license_key TEXT, license_id VARCHAR, customer VARCHAR,
    plan VARCHAR, max_units INT, issued_at TIMESTAMPTZ, expires_at TIMESTAMPTZ,
    instance_id UUID, activated_at TIMESTAMPTZ, is_active BOOLEAN, raw_payload JSONB,
    remote_state VARCHAR, last_heartbeat_at TIMESTAMPTZ, last_heartbeat_error TEXT
)
LANGUAGE sql AS $$
    SELECT l.id, l.license_key, l.license_id, l.customer, l.plan, l.max_units,
           l.issued_at, l.expires_at, l.instance_id, l.activated_at,
           l.is_active, l.raw_payload, l.remote_state, l.last_heartbeat_at,
           l.last_heartbeat_error
      FROM licenses l
     WHERE l.is_active = TRUE
     ORDER BY l.activated_at DESC
     LIMIT 1;
$$;
