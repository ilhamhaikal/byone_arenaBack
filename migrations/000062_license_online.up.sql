-- =============================================================================
-- 000062_license_online.up.sql
--
-- Integrasi license server vendor:
--   • state remote (active/suspended/revoked/expired/inactive)
--   • heartbeat terakhir
--   • request aktivasi QR (secret disimpan terenkripsi aplikasi)
--
-- Semua akses data tetap lewat stored procedure.
-- =============================================================================

ALTER TABLE licenses
    ADD COLUMN IF NOT EXISTS remote_state VARCHAR(20) NOT NULL DEFAULT 'active',
    ADD COLUMN IF NOT EXISTS last_heartbeat_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_heartbeat_error TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_licenses_remote_state ON licenses (remote_state);

CREATE TABLE IF NOT EXISTS license_activation_requests (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code          VARCHAR(40) NOT NULL,
    secret_cipher TEXT NOT NULL,
    status        VARCHAR(20) NOT NULL DEFAULT 'pending',
    expires_at    TIMESTAMPTZ NOT NULL,
    last_poll_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT license_activation_requests_status_check
        CHECK (status IN ('pending', 'claimed', 'rejected', 'expired', 'cancelled'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_license_activation_code
    ON license_activation_requests (code);
CREATE UNIQUE INDEX IF NOT EXISTS idx_license_activation_one_pending
    ON license_activation_requests ((status)) WHERE status = 'pending';

-- ── Aktivasi online ─────────────────────────────────────────────────────────
DROP FUNCTION IF EXISTS "__SP__StartLicenseActivationRequest"(VARCHAR, TEXT, TIMESTAMPTZ);
CREATE OR REPLACE FUNCTION "__SP__StartLicenseActivationRequest"(
    p_code         VARCHAR,
    p_secret_cipher TEXT,
    p_expires_at   TIMESTAMPTZ
)
RETURNS TABLE(id UUID)
LANGUAGE plpgsql AS $$
DECLARE
    v_id UUID;
BEGIN
    UPDATE license_activation_requests
       SET status = 'cancelled', updated_at = NOW()
     WHERE status = 'pending';

    INSERT INTO license_activation_requests (code, secret_cipher, expires_at)
    VALUES (p_code, p_secret_cipher, p_expires_at)
    RETURNING license_activation_requests.id INTO v_id;

    RETURN QUERY SELECT v_id;
END;
$$;

DROP FUNCTION IF EXISTS "__SP__GetPendingLicenseActivationRequest"();
CREATE OR REPLACE FUNCTION "__SP__GetPendingLicenseActivationRequest"()
RETURNS TABLE(
    id UUID, code VARCHAR, secret_cipher TEXT, expires_at TIMESTAMPTZ, created_at TIMESTAMPTZ
)
LANGUAGE sql AS $$
    SELECT r.id, r.code, r.secret_cipher, r.expires_at, r.created_at
      FROM license_activation_requests r
     WHERE r.status = 'pending' AND r.expires_at > NOW()
     ORDER BY r.created_at DESC
     LIMIT 1;
$$;

DROP FUNCTION IF EXISTS "__SP__UpdateLicenseActivationRequest"(UUID, VARCHAR, TIMESTAMPTZ);
CREATE OR REPLACE FUNCTION "__SP__UpdateLicenseActivationRequest"(
    p_id           UUID,
    p_status       VARCHAR,
    p_last_poll_at TIMESTAMPTZ
)
RETURNS VOID
LANGUAGE sql AS $$
    UPDATE license_activation_requests
       SET status = p_status,
           last_poll_at = p_last_poll_at,
           updated_at = NOW()
     WHERE id = p_id;
$$;

-- ── Lisensi aktif: sertakan state remote + heartbeat ────────────────────────
DROP FUNCTION IF EXISTS "__SP__GetActiveLicense"();
CREATE OR REPLACE FUNCTION "__SP__GetActiveLicense"()
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
           -- Pertahankan tanggal terbit pertama; heartbeat menandatangani token
           -- baru dengan issuedAt=now, jangan sampai riwayat ikut berubah.
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

DROP FUNCTION IF EXISTS "__SP__SetLicenseRemoteState"(VARCHAR, TIMESTAMPTZ, TEXT);
CREATE OR REPLACE FUNCTION "__SP__SetLicenseRemoteState"(
    p_state        VARCHAR,
    p_heartbeat_at TIMESTAMPTZ,
    p_error        TEXT
)
RETURNS VOID
LANGUAGE sql AS $$
    UPDATE licenses
       SET remote_state = COALESCE(NULLIF(p_state, ''), remote_state),
           last_heartbeat_at = COALESCE(p_heartbeat_at, last_heartbeat_at),
           last_heartbeat_error = COALESCE(p_error, ''),
           updated_at = NOW()
     WHERE is_active = TRUE;
$$;

DROP FUNCTION IF EXISTS "__SP__SetLicenseHeartbeatError"(TEXT, TIMESTAMPTZ);
CREATE OR REPLACE FUNCTION "__SP__SetLicenseHeartbeatError"(
    p_error        TEXT,
    p_heartbeat_at TIMESTAMPTZ
)
RETURNS VOID
LANGUAGE sql AS $$
    UPDATE licenses
       SET last_heartbeat_error = COALESCE(p_error, ''),
           last_heartbeat_at = COALESCE(p_heartbeat_at, last_heartbeat_at),
           updated_at = NOW()
     WHERE is_active = TRUE;
$$;