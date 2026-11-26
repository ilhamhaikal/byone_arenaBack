-- =============================================================================
-- 000048_saved_time_expiry.up.sql
--
-- Simpan waktu untuk sesi NON-MEMBER:
--   * "Akhiri" otomatis menyimpan sisa menit berbayar (alasan ended_saved).
--   * Tombol "Simpan Waktu" menyimpan sisa menit tanpa menunggu waktu habis
--     (alasan saved_by_admin).
--   * Sesi yang terputus karena listrik mati / TV offline juga disimpan.
--   * Sisa waktu non-member berlaku 24 jam (saved_time_expires_at).
--   * Member TIDAK memakai mekanisme ini; sisa waktunya masuk Bank Waktu
--     (tanpa kedaluwarsa) seperti sebelumnya.
--
-- Semua logika query berada di stored procedure. Handler hanya memanggil SP.
-- =============================================================================

ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS saved_time_expires_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_sessions_saved_time_expires
    ON sessions (status, saved_time_expires_at);

-- Backfill: sesi interrupted non-member yang sudah ada mengikuti aturan baru.
-- Sesi member tetap NULL (Bank Waktu tanpa kedaluwarsa).
UPDATE sessions s
   SET saved_time_expires_at =
       COALESCE(s.interrupted_at, s.updated_at, NOW()) + INTERVAL '24 hours'
 WHERE s.status = 'interrupted'
   AND s.remaining_minutes > 0
   AND s.saved_time_expires_at IS NULL
   AND (
       s.customer_id IS NULL
       OR NOT EXISTS (
           SELECT 1 FROM customers c
            WHERE c.id = s.customer_id AND c.is_member = TRUE
       )
   );

-- ── Simpan waktu manual / otomatis (non-member) ─────────────────────────────
DROP FUNCTION IF EXISTS "__SP__SaveNonMemberTime"(UUID, TEXT);

CREATE OR REPLACE FUNCTION "__SP__SaveNonMemberTime"(
    p_session_id UUID,
    p_reason TEXT DEFAULT 'saved_by_admin'
)
RETURNS TABLE (
    saved BOOLEAN,
    reason TEXT,
    session_id UUID,
    console_id UUID,
    remaining_minutes INT,
    expires_at TIMESTAMPTZ
)
LANGUAGE plpgsql AS $$
DECLARE
    v_session   sessions%ROWTYPE;
    v_is_member BOOLEAN := FALSE;
    v_pending   BOOLEAN := FALSE;
    v_paid      NUMERIC(10,2) := 0;
    v_now       TIMESTAMPTZ := NOW();
    v_duration  INT;
    v_remaining INT;
    v_expires   TIMESTAMPTZ;
BEGIN
    SELECT * INTO v_session FROM sessions WHERE sessions.id = p_session_id FOR UPDATE;
    IF NOT FOUND THEN
        RETURN QUERY SELECT FALSE, 'not_found'::TEXT, NULL::UUID, NULL::UUID, 0, NULL::TIMESTAMPTZ;
        RETURN;
    END IF;

    IF v_session.status <> 'active' THEN
        RETURN QUERY SELECT FALSE, 'not_active'::TEXT, v_session.id, v_session.console_id, 0, NULL::TIMESTAMPTZ;
        RETURN;
    END IF;

    IF v_session.customer_id IS NOT NULL THEN
        SELECT COALESCE(is_member, FALSE) INTO v_is_member
          FROM customers WHERE customers.id = v_session.customer_id;
    END IF;
    IF v_is_member THEN
        -- Member memakai Bank Waktu (tanpa kedaluwarsa), bukan mekanisme ini.
        RETURN QUERY SELECT FALSE, 'member'::TEXT, v_session.id, v_session.console_id, 0, NULL::TIMESTAMPTZ;
        RETURN;
    END IF;

    SELECT EXISTS (
        SELECT 1 FROM payments p
         WHERE p.session_id = p_session_id AND p.payment_status = 'pending'
    ) INTO v_pending;
    IF v_pending THEN
        RETURN QUERY SELECT FALSE, 'pending'::TEXT, v_session.id, v_session.console_id, 0, NULL::TIMESTAMPTZ;
        RETURN;
    END IF;

    SELECT COALESCE(SUM(p.total_payment), 0) INTO v_paid
      FROM payments p
     WHERE p.session_id = p_session_id AND p.payment_status = 'paid';
    IF v_paid <= 0 THEN
        -- Sesi gratis (voucher 100%) tidak punya menit berbayar untuk disimpan.
        RETURN QUERY SELECT FALSE, 'unpaid'::TEXT, v_session.id, v_session.console_id, 0, NULL::TIMESTAMPTZ;
        RETURN;
    END IF;

    -- Hanya simpan bila masih ada sisa dan belum overtime.
    IF v_session.end_scheduled_at IS NULL OR v_session.end_scheduled_at <= v_now THEN
        RETURN QUERY SELECT FALSE, 'no_remaining'::TEXT, v_session.id, v_session.console_id, 0, NULL::TIMESTAMPTZ;
        RETURN;
    END IF;

    v_duration := GREATEST(EXTRACT(EPOCH FROM (v_now - v_session.start_time))::INT / 60, 1);
    v_remaining := GREATEST(
        CEIL(EXTRACT(EPOCH FROM (v_session.end_scheduled_at - v_now)) / 60)::INT,
        0
    );
    IF v_remaining <= 0 THEN
        RETURN QUERY SELECT FALSE, 'no_remaining'::TEXT, v_session.id, v_session.console_id, 0, NULL::TIMESTAMPTZ;
        RETURN;
    END IF;

    v_expires := v_now + INTERVAL '24 hours';

    UPDATE sessions
       SET end_time = v_now,
           duration_minutes = v_duration,
           status = 'interrupted',
           interrupted_at = v_now,
           remaining_minutes = v_remaining,
           interrupted_reason = p_reason,
           saved_time_expires_at = v_expires,
           updated_at = v_now
     WHERE sessions.id = p_session_id;

    UPDATE consoles
       SET status = 'available',
           screen_status = 'off',
           manual_live = FALSE,
           updated_at = v_now
     WHERE consoles.id = v_session.console_id;

    INSERT INTO tv_activity_logs
        (id, console_id, event, session_id, is_authorized, duration_minutes, created_at)
    VALUES
        (uuid_generate_v4(), v_session.console_id, 'off', p_session_id, TRUE, v_duration, v_now);

    RETURN QUERY SELECT TRUE, 'saved'::TEXT, p_session_id, v_session.console_id, v_remaining, v_expires;
END;
$$;

-- ── InterruptSession: tambah expiry untuk non-member ────────────────────────
DROP FUNCTION IF EXISTS "__SP__InterruptSession"(UUID, TEXT);

CREATE OR REPLACE FUNCTION "__SP__InterruptSession"(
    p_session_id UUID,
    p_reason TEXT DEFAULT 'power_outage'
)
RETURNS TABLE (
    session_id UUID,
    console_id UUID,
    remaining_minutes INT,
    duration_minutes INT
)
LANGUAGE plpgsql AS $$
DECLARE
    v_session    sessions%ROWTYPE;
    v_last_seen  TIMESTAMPTZ;
    v_end        TIMESTAMPTZ;
    v_now        TIMESTAMPTZ := NOW();
    v_duration   INT;
    v_remaining  INT := 0;
    v_is_member  BOOLEAN := FALSE;
BEGIN
    SELECT * INTO v_session FROM sessions WHERE sessions.id = p_session_id FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'SESSION_NOT_FOUND';
    END IF;
    IF v_session.status <> 'active' THEN
        RAISE EXCEPTION 'SESSION_NOT_ACTIVE: %', v_session.status;
    END IF;

    IF v_session.customer_id IS NOT NULL THEN
        SELECT COALESCE(is_member, FALSE) INTO v_is_member
          FROM customers WHERE customers.id = v_session.customer_id;
    END IF;

    SELECT c.last_seen_at INTO v_last_seen
      FROM consoles c WHERE c.id = v_session.console_id;

    v_end := LEAST(
        COALESCE(v_last_seen, v_now),
        COALESCE(v_session.end_scheduled_at, v_now)
    );
    IF v_end < v_session.start_time THEN
        v_end := v_session.start_time;
    END IF;

    v_duration := GREATEST(EXTRACT(EPOCH FROM (v_end - v_session.start_time))::INT / 60, 1);

    IF v_session.end_scheduled_at IS NOT NULL THEN
        v_remaining := GREATEST(
            CEIL(EXTRACT(EPOCH FROM (v_session.end_scheduled_at - v_end)) / 60)::INT,
            0
        );
    END IF;

    UPDATE sessions
       SET end_time = v_end,
           duration_minutes = v_duration,
           status = 'interrupted',
           interrupted_at = v_now,
           remaining_minutes = v_remaining,
           interrupted_reason = p_reason,
           -- Non-member: berlaku 24 jam sejak waktu terputus. Member: NULL
           -- (bank waktu member tidak memakai expiry).
           saved_time_expires_at = CASE
               WHEN v_is_member THEN NULL
               ELSE v_end + INTERVAL '24 hours'
           END,
           updated_at = v_now
     WHERE sessions.id = p_session_id;

    -- TV kembali ke kondisi aman: konsol available, layar off, bukan live manual.
    UPDATE consoles
       SET status = 'available',
           screen_status = 'off',
           manual_live = FALSE,
           updated_at = v_now
     WHERE consoles.id = v_session.console_id;

    INSERT INTO tv_activity_logs
        (id, console_id, event, session_id, is_authorized, duration_minutes, created_at)
    VALUES
        (uuid_generate_v4(), v_session.console_id, 'off', p_session_id, TRUE, v_duration, v_now);

    RETURN QUERY SELECT p_session_id, v_session.console_id, v_remaining, v_duration;
END;
$$;

-- ── Daftar sesi terputus: sembunyikan yang sudah kedaluwarsa ────────────────
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
    saved_time_expires_at TIMESTAMPTZ
)
LANGUAGE sql AS $$
    SELECT
        s.id AS session_id,
        s.console_id,
        c.name AS console_name,
        COALESCE(cust.name, 'Umum') AS customer_name,
        s.remaining_minutes,
        s.interrupted_at,
        COALESCE(s.interrupted_reason, '') AS reason,
        COALESCE(s.notes, '') AS notes,
        s.saved_time_expires_at
    FROM sessions s
    JOIN consoles c ON c.id = s.console_id
    LEFT JOIN customers cust ON cust.id = s.customer_id
    WHERE s.status = 'interrupted'
      AND s.remaining_minutes > 0
      AND (s.saved_time_expires_at IS NULL OR s.saved_time_expires_at > NOW())
    ORDER BY s.interrupted_at DESC NULLS LAST
    LIMIT GREATEST(1, LEAST(COALESCE(p_limit, 20), 100));
$$;