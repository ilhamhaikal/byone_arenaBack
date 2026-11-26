-- =============================================================================
-- 000051_promo_audience_and_saved_time_expiry.up.sql
--
-- 1. Bonus waktu punya audiens:
--      general = pelanggan umum / tanpa pelanggan
--      member  = pelanggan yang valid sebagai member
--    Saat mulai sesi, hanya aturan yang sesuai audiens pelanggan yang dipakai.
--
-- 2. Seed aturan weekend:
--      - Bonus Weekend Umum   : Sabtu/Minggu, min 60 menit, bonus 30 menit
--      - Bonus Weekend Member : Sabtu/Minggu, min 60 menit, bonus 60 menit
--    Nilai bisa diubah lewat menu Bonus Waktu. Data default aktif; jika
--    client tidak ingin memakai bonus, cukup nonaktifkan aturannya.
--
-- 3. Saved time yang sudah lewat masa berlaku dibersihkan:
--      - non-member/reguler : 24 jam
--      - paket              : 7 hari (sudah diatur migrasi 000050)
--    Sesi yang kedaluwarsa tidak lagi muncul di daftar dan tidak bisa dipulihkan.
-- =============================================================================

ALTER TABLE promo_bonus_rules
    ADD COLUMN IF NOT EXISTS audience VARCHAR(20) NOT NULL DEFAULT 'general';

ALTER TABLE promo_bonus_rules
    DROP CONSTRAINT IF EXISTS promo_bonus_rules_audience_check;

ALTER TABLE promo_bonus_rules
    ADD CONSTRAINT promo_bonus_rules_audience_check
    CHECK (audience IN ('general', 'member'));

CREATE INDEX IF NOT EXISTS idx_promo_bonus_rules_audience
    ON promo_bonus_rules (audience, is_active);

-- Seed bonus umum weekend bila belum ada aturan aktif yang mencakup Sabtu+Minggu.
-- 7 diterima sebagai alias Minggu untuk data lama (lihat promo.ParseDays).
INSERT INTO promo_bonus_rules
    (name, days_of_week, min_paid_minutes, bonus_minutes,
     max_per_session, audience, is_active, created_at, updated_at)
SELECT
    'Bonus Weekend Umum', '0,6', 60, 30,
    0, 'general', TRUE, NOW(), NOW()
WHERE NOT EXISTS (
    SELECT 1
      FROM promo_bonus_rules r
     WHERE r.is_active = TRUE
       AND r.audience = 'general'
       AND (',' || REPLACE(r.days_of_week, ' ', '') || ',') LIKE '%,6,%'
       AND (
           (',' || REPLACE(r.days_of_week, ' ', '') || ',') LIKE '%,0,%'
           OR (',' || REPLACE(r.days_of_week, ' ', '') || ',') LIKE '%,7,%'
       )
);

-- Seed bonus member weekend bila belum ada.
INSERT INTO promo_bonus_rules
    (name, days_of_week, min_paid_minutes, bonus_minutes,
     max_per_session, audience, is_active, created_at, updated_at)
SELECT
    'Bonus Weekend Member', '0,6', 60, 60,
    0, 'member', TRUE, NOW(), NOW()
WHERE NOT EXISTS (
    SELECT 1
      FROM promo_bonus_rules r
     WHERE r.is_active = TRUE
       AND r.audience = 'member'
       AND (',' || REPLACE(r.days_of_week, ' ', '') || ',') LIKE '%,6,%'
       AND (
           (',' || REPLACE(r.days_of_week, ' ', '') || ',') LIKE '%,0,%'
           OR (',' || REPLACE(r.days_of_week, ' ', '') || ',') LIKE '%,7,%'
       )
);

-- ── Expire saved time ───────────────────────────────────────────────────────
-- Dipanggil oleh InterruptedSessions agar data yang sudah kedaluwarsa
-- langsung dibersihkan saat dashboard meminta daftar.
DROP FUNCTION IF EXISTS "__SP__ExpireSavedTimeSessions"();

CREATE OR REPLACE FUNCTION "__SP__ExpireSavedTimeSessions"()
RETURNS INT
LANGUAGE plpgsql AS $$
DECLARE
    v_count INT := 0;
BEGIN
    UPDATE sessions
       SET remaining_minutes = 0,
           updated_at = NOW()
     WHERE status = 'interrupted'
       AND remaining_minutes > 0
       AND saved_time_expires_at IS NOT NULL
       AND saved_time_expires_at <= NOW();

    GET DIAGNOSTICS v_count = ROW_COUNT;
    RETURN v_count;
END;
$$;

-- ── InterruptedSessions: bersihkan yang kedaluwarsa lalu tampilkan sisanya ──
DROP FUNCTION IF EXISTS "__SP__InterruptedSessions"(INT);

CREATE OR REPLACE FUNCTION "__SP__InterruptedSessions"(p_limit INT DEFAULT 20)
RETURNS TABLE (
    session_id UUID,
    console_id UUID,
    console_name VARCHAR,
    customer_name VARCHAR,
    remaining_minutes INT,
    interrupted_at TIMESTAMPTZ,
    reason TEXT,
    notes TEXT,
    saved_time_expires_at TIMESTAMPTZ,
    is_package BOOLEAN
)
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM "__SP__ExpireSavedTimeSessions"();

    RETURN QUERY
    SELECT
        s.id AS session_id,
        s.console_id,
        c.name AS console_name,
        COALESCE(cust.name, 'Umum') AS customer_name,
        s.remaining_minutes,
        s.interrupted_at,
        COALESCE(s.interrupted_reason, '') AS reason,
        COALESCE(s.notes, '') AS notes,
        s.saved_time_expires_at,
        s.is_package
    FROM sessions s
    JOIN consoles c ON c.id = s.console_id
    LEFT JOIN customers cust ON cust.id = s.customer_id
    WHERE s.status = 'interrupted'
      AND s.remaining_minutes > 0
      AND (s.saved_time_expires_at IS NULL OR s.saved_time_expires_at > NOW())
    ORDER BY s.interrupted_at DESC NULLS LAST
    LIMIT GREATEST(1, LEAST(COALESCE(p_limit, 20), 100));
END;
$$;