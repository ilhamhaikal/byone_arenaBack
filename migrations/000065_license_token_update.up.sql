-- 000065_license_token_update.up.sql
-- Database billing hanya menyimpan token lisensi lokal. Heartbeat boleh
-- memperbarui token, tetapi state remote/aktivasi online tidak disimpan di sini.
DROP FUNCTION IF EXISTS "__SP__UpdateLicenseToken"(TEXT, VARCHAR, VARCHAR, VARCHAR, INT, TIMESTAMPTZ, JSONB, TIMESTAMPTZ);
CREATE OR REPLACE FUNCTION "__SP__UpdateLicenseToken"(
    p_license_key  TEXT,
    p_license_id   VARCHAR,
    p_customer     VARCHAR,
    p_plan         VARCHAR,
    p_max_units    INT,
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
           expires_at = COALESCE(p_expires_at, expires_at),
           raw_payload = COALESCE(p_raw_payload, raw_payload),
           updated_at = COALESCE(p_heartbeat_at, NOW())
     WHERE is_active = TRUE
     RETURNING licenses.id INTO v_id;

    RETURN QUERY SELECT v_id;
END;
$$;
