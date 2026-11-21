-- =============================================================================
-- 000039_session_recovery.up.sql
--
-- Pemulihan sesi saat listrik mati / TV-server terputus.
--
--   status = 'interrupted'  → sisa waktu disimpan, admin bisa "Pulihkan Sesi"
--   remaining_minutes       → sisa menit pada heartbeat terakhir
--   recovered_by_session_id → sesi baru hasil pemulihan
--   recovered_from_session_id → sesi asal (di sesi baru)
--
-- Sesi interrupted TIDAK dikenakan overtime dan TIDAK dikreditkan ke Bank
-- Waktu (agar tidak dobel dengan sesi pemulihan gratis via voucher GRATIS).
-- =============================================================================

ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS interrupted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS remaining_minutes INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS interrupted_reason TEXT,
    ADD COLUMN IF NOT EXISTS recovered_by_session_id UUID,
    ADD COLUMN IF NOT EXISTS recovered_from_session_id UUID;

CREATE INDEX IF NOT EXISTS idx_sessions_interrupted
    ON sessions (status, interrupted_at DESC);

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
BEGIN
    SELECT * INTO v_session FROM sessions WHERE sessions.id = p_session_id FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'SESSION_NOT_FOUND';
    END IF;
    IF v_session.status <> 'active' THEN
        RAISE EXCEPTION 'SESSION_NOT_ACTIVE: %', v_session.status;
    END IF;

    SELECT c.last_seen_at INTO v_last_seen
      FROM consoles c WHERE c.id = v_session.console_id;

    v_end := COALESCE(v_session.end_scheduled_at, v_now);
    -- Waktu terputus = heartbeat terakhir (sebelum jadwal selesai).
    IF v_last_seen IS NOT NULL THEN
        v_end := LEAST(v_last_seen, v_end);
    END IF;
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
