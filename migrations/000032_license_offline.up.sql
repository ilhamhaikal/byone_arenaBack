-- =============================================================================
-- 000032_license_offline.up.sql
--
-- Lisensi OFFLINE (tanpa server vendor):
--   • server_identity  : ID unik server (dibuat sekali) → pengikat lisensi
--   • licenses         : lisensi aktif + riwayat
--   • license_events   : audit aktivasi / penolakan
--
-- Semua akses data lewat stored procedure (aturan proyek).
-- =============================================================================

-- ── Identitas server ────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS server_identity (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO server_identity (id)
SELECT uuid_generate_v4()
WHERE NOT EXISTS (SELECT 1 FROM server_identity);

DROP FUNCTION IF EXISTS "__SP__GetOrCreateServerIdentity"();
CREATE OR REPLACE FUNCTION "__SP__GetOrCreateServerIdentity"()
RETURNS TABLE(id UUID, created_at TIMESTAMPTZ)
LANGUAGE plpgsql AS $$
DECLARE
    v_id  UUID;
    v_crt TIMESTAMPTZ;
BEGIN
    SELECT si.id, si.created_at INTO v_id, v_crt
      FROM server_identity si
     ORDER BY si.created_at ASC
     LIMIT 1;

    IF v_id IS NULL THEN
        INSERT INTO server_identity (id) VALUES (uuid_generate_v4())
        RETURNING server_identity.id, server_identity.created_at INTO v_id, v_crt;
    END IF;

    RETURN QUERY SELECT v_id, v_crt;
END;
$$;

-- ── Lisensi ─────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS licenses (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    license_key   TEXT NOT NULL,                 -- teks lisensi asli (bertanda tangan)
    license_id    VARCHAR(80)  NOT NULL DEFAULT '',
    customer      VARCHAR(150) NOT NULL DEFAULT '',
    plan          VARCHAR(60)  NOT NULL DEFAULT '',
    max_units     INT          NOT NULL DEFAULT 0,   -- 0 = unlimited
    issued_at     TIMESTAMPTZ,
    expires_at    TIMESTAMPTZ  NOT NULL,
    instance_id   UUID         NOT NULL,
    activated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    is_active     BOOLEAN      NOT NULL DEFAULT TRUE,
    raw_payload   JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT licenses_max_units_nonnegative CHECK (max_units >= 0)
);

CREATE INDEX IF NOT EXISTS idx_licenses_active   ON licenses (is_active);
CREATE INDEX IF NOT EXISTS idx_licenses_expires  ON licenses (expires_at);
CREATE INDEX IF NOT EXISTS idx_licenses_activated ON licenses (activated_at DESC);

CREATE TABLE IF NOT EXISTS license_events (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_type  VARCHAR(40) NOT NULL,
    license_id  VARCHAR(80) NOT NULL DEFAULT '',
    detail      JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_license_events_created ON license_events (created_at DESC);

-- ── Simpan/aktifkan lisensi (menonaktifkan lisensi lama) ────────────────────
DROP FUNCTION IF EXISTS "__SP__SaveLicense"(TEXT, VARCHAR, VARCHAR, VARCHAR, INT, TIMESTAMPTZ, TIMESTAMPTZ, UUID, JSONB);
CREATE OR REPLACE FUNCTION "__SP__SaveLicense"(
    p_license_key  TEXT,
    p_license_id   VARCHAR,
    p_customer     VARCHAR,
    p_plan         VARCHAR,
    p_max_units    INT,
    p_issued_at    TIMESTAMPTZ,
    p_expires_at   TIMESTAMPTZ,
    p_instance_id  UUID,
    p_raw_payload  JSONB
)
RETURNS TABLE(id UUID)
LANGUAGE plpgsql AS $$
DECLARE
    v_id UUID;
BEGIN
    UPDATE licenses
       SET is_active = FALSE, updated_at = NOW()
     WHERE is_active = TRUE;

    INSERT INTO licenses
        (license_key, license_id, customer, plan, max_units,
         issued_at, expires_at, instance_id, raw_payload)
    VALUES
        (p_license_key, COALESCE(p_license_id,''), COALESCE(p_customer,''),
         COALESCE(p_plan,''), GREATEST(COALESCE(p_max_units,0),0),
         p_issued_at, p_expires_at, p_instance_id, COALESCE(p_raw_payload,'{}'::jsonb))
    RETURNING licenses.id INTO v_id;

    RETURN QUERY SELECT v_id;
END;
$$;

-- ── Lisensi aktif ───────────────────────────────────────────────────────────
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

-- ── Jumlah unit terpakai (TV yang terhubung ke konsol) ──────────────────────
DROP FUNCTION IF EXISTS "__SP__CountLicensedUnits"();
CREATE OR REPLACE FUNCTION "__SP__CountLicensedUnits"()
RETURNS TABLE(units INT)
LANGUAGE sql AS $$
    SELECT COUNT(*)::INT AS units
      FROM consoles
     WHERE device_id IS NOT NULL;
$$;

-- ── Riwayat lisensi ─────────────────────────────────────────────────────────
DROP FUNCTION IF EXISTS "__SP__LicenseHistory"(INT);
CREATE OR REPLACE FUNCTION "__SP__LicenseHistory"(p_limit INT DEFAULT 20)
RETURNS TABLE(
    id UUID, license_id VARCHAR, customer VARCHAR, plan VARCHAR, max_units INT,
    issued_at TIMESTAMPTZ, expires_at TIMESTAMPTZ, activated_at TIMESTAMPTZ, is_active BOOLEAN
)
LANGUAGE sql AS $$
    SELECT l.id, l.license_id, l.customer, l.plan, l.max_units,
           l.issued_at, l.expires_at, l.activated_at, l.is_active
      FROM licenses l
     ORDER BY l.activated_at DESC
     LIMIT GREATEST(COALESCE(p_limit, 20), 1);
$$;

-- ── Audit event lisensi ─────────────────────────────────────────────────────
CREATE OR REPLACE FUNCTION "__SP__LogLicenseEvent"(
    p_event_type VARCHAR,
    p_license_id VARCHAR,
    p_detail     JSONB
)
RETURNS VOID
LANGUAGE sql AS $$
    INSERT INTO license_events (event_type, license_id, detail)
    VALUES (COALESCE(p_event_type,''), COALESCE(p_license_id,''), COALESCE(p_detail,'{}'::jsonb));
$$;
