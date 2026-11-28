-- 000063_license_heartbeat_issued_at.up.sql
-- Heartbeat tidak boleh mengubah tanggal terbit lisensi menjadi waktu heartbeat.
DROP FUNCTION IF EXISTS "__SP__UpdateLicenseHeartbeat"(TEXT, VARCHAR, VARCHAR, VARCHAR, INT, TIMESTAMPTZ, TIMESTAMPTZ, JSONB, TIMESTAMPTZ);
CREATE OR REPLACE FUNCTION "__SP__UpdateLicenseHeartbeat"(
    p_license_key  TEXT,
    p_license_id   VARCHAR,
    p_customer     VARCHAR,
    p_plan         VARCHAR,
    p_max_units    INT,
    p_issued_at    TIMESTAMPTZ,
    p_expires_at   TIMESTAMPTZ,
    p_raw_payload  JSONB,
    p_heartbeat_at TIMESTAMPTZ
)
RETURNS TABLE(id UUID)
LANGUAGE plpgsql AS $$
DECLARE
    v_id UUID;
BEGIN
    UPDATE licenses
       SET license_key = p_license_key,
           license_id = COALESCE(p_license_id, license_id),
           customer = COALESCE(p_customer, customer),
           plan = COALESCE(p_plan, plan),
           max_units = GREATEST(COALESCE(p_max_units, max_units), 0),
           issued_at = COALESCE(issued_at, p_issued_at),
           expires_at = COALESCE(p_expires_at, expires_at),
           raw_payload = COALESCE(p_raw_payload, raw_payload),
           remote_state = 'active',
           last_heartbeat_at = p_heartbeat_at,
           last_heartbeat_error = '',
           updated_at = NOW()
     WHERE is_active = TRUE
     RETURNING licenses.id INTO v_id;

    RETURN QUERY SELECT v_id;
END;
$$;
