-- =============================================================================
-- 000037_live_without_session.down.sql
-- Kembalikan perilaku lama (ON tanpa sesi = unauthorized).
-- =============================================================================

CREATE OR REPLACE FUNCTION "__SP__LogTvActivity"(p_console_id UUID, p_event VARCHAR, p_session_id UUID DEFAULT NULL)
RETURNS TABLE(log_id UUID, is_authorized BOOLEAN, session_id UUID, duration_minutes INT, message VARCHAR)
LANGUAGE plpgsql AS $$
DECLARE
    v_log_id       UUID;
    v_sid          UUID := NULL;
    v_auth         BOOLEAN := FALSE;
    v_dur          INT := NULL;
    v_last_on      TIMESTAMPTZ;
    v_last_on_auth BOOLEAN;
    v_last_evt     VARCHAR;
    v_now          TIMESTAMPTZ := NOW();
BEGIN
    IF NOT EXISTS(SELECT 1 FROM consoles WHERE id=p_console_id) THEN
        RAISE EXCEPTION 'CONSOLE_NOT_FOUND'; END IF;

    -- DEDUP: skip kalau status sama
    SELECT t.event INTO v_last_evt FROM tv_activity_logs t
    WHERE t.console_id=p_console_id ORDER BY t.created_at DESC LIMIT 1;
    IF v_last_evt = p_event THEN
        RETURN QUERY SELECT NULL::UUID, FALSE, NULL::UUID, NULL::INT,
            ('Status TV tetap: '||p_event||' (tidak dicatat ulang)')::VARCHAR;
        RETURN;
    END IF;

    IF p_event = 'on' THEN
        SELECT s.id INTO v_sid FROM sessions s
        WHERE s.console_id=p_console_id AND s.status='active'
        ORDER BY s.start_time DESC LIMIT 1;
        v_auth := (v_sid IS NOT NULL);
        v_log_id := uuid_generate_v4();
        INSERT INTO tv_activity_logs(id,console_id,event,session_id,is_authorized,created_at)
        VALUES(v_log_id,p_console_id,'on',v_sid,v_auth,v_now);
        RETURN QUERY SELECT v_log_id, v_auth, v_sid, NULL::INT,
            (CASE WHEN v_auth THEN 'TV menyala — SESI AKTIF'
                 ELSE '⚠️ TV menyala — TANPA SESI (unauthorized)' END)::VARCHAR;
    ELSE
        SELECT t.created_at, t.is_authorized INTO v_last_on, v_last_on_auth
        FROM tv_activity_logs t
        WHERE t.console_id=p_console_id AND t.event='on'
        ORDER BY t.created_at DESC LIMIT 1;
        SELECT s.id INTO v_sid FROM sessions s
        WHERE s.console_id=p_console_id AND s.status='active'
        ORDER BY s.start_time DESC LIMIT 1;
        v_auth := COALESCE(v_last_on_auth, (v_sid IS NOT NULL));
        IF v_last_on IS NOT NULL THEN
            v_dur := EXTRACT(EPOCH FROM (v_now - v_last_on))/60; END IF;
        v_log_id := uuid_generate_v4();
        INSERT INTO tv_activity_logs(id,console_id,event,session_id,is_authorized,duration_minutes,created_at)
        VALUES(v_log_id,p_console_id,p_event,v_sid,v_auth,v_dur,v_now);
        RETURN QUERY SELECT v_log_id, v_auth, v_sid, v_dur,
            ('TV '||p_event||'. '||
            CASE WHEN v_dur IS NOT NULL THEN 'Durasi: '||v_dur||' menit. ' ELSE '' END||
            CASE WHEN v_sid IS NOT NULL THEN 'Sesi aktif.' ELSE 'Tanpa sesi.' END)::VARCHAR;
    END IF;
END; $$;

CREATE OR REPLACE FUNCTION "__SP__GetTvLogs"(p_console_id UUID, p_date DATE DEFAULT NULL)
RETURNS TABLE(
    logs JSONB, unauthorized_count BIGINT, total_on_minutes BIGINT,
    authorized_minutes BIGINT, unauthorized_minutes BIGINT,
    active_session JSONB, unauthorized_logs JSONB)
LANGUAGE plpgsql AS $$
DECLARE
    v_logs JSONB; v_unauth BIGINT; v_tot BIGINT; v_auth BIGINT; v_una BIGINT;
    v_ses JSONB; v_ulogs JSONB; v_run INT;
    v_now TIMESTAMPTZ := NOW();
    v_is_today BOOLEAN;
    v_last_on TIMESTAMPTZ;
    v_last_on_auth BOOLEAN;
BEGIN
    v_is_today := (p_date IS NULL OR p_date = v_now::DATE);

    SELECT COALESCE(jsonb_agg(row_to_json(l) ORDER BY l.so DESC),'[]'::JSONB) INTO v_logs
    FROM (SELECT t.id, t.event, t.is_authorized AS "isAuthorized",
          t.duration_minutes AS "durationMinutes", t.session_id AS "sessionId",
          to_char(t.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"') AS "createdAt",
          c.name AS "consoleName", t.created_at AS so
          FROM tv_activity_logs t JOIN consoles c ON c.id=t.console_id
          WHERE t.console_id=p_console_id AND (p_date IS NULL OR t.created_at::DATE=p_date)
          ORDER BY t.created_at DESC LIMIT 100) l;

    SELECT COALESCE(jsonb_agg(row_to_json(u) ORDER BY u.so DESC),'[]'::JSONB) INTO v_ulogs
    FROM (SELECT t.id, t.event, t.is_authorized AS "isAuthorized",
          t.duration_minutes AS "durationMinutes",
          to_char(t.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"') AS "createdAt", t.created_at AS so
          FROM tv_activity_logs t WHERE t.console_id=p_console_id AND t.is_authorized=FALSE
          AND (p_date IS NULL OR t.created_at::DATE=p_date) ORDER BY t.created_at DESC LIMIT 20) u;

    SELECT COUNT(*) INTO v_unauth FROM tv_activity_logs
    WHERE console_id=p_console_id AND event='on' AND is_authorized=FALSE
    AND (p_date IS NULL OR created_at::DATE=p_date);

    SELECT COALESCE(SUM(duration_minutes),0) INTO v_tot FROM tv_activity_logs
    WHERE console_id=p_console_id AND event IN('off','sleep','screensaver')
    AND duration_minutes IS NOT NULL AND (p_date IS NULL OR created_at::DATE=p_date);

    SELECT COALESCE(SUM(duration_minutes),0) INTO v_auth FROM tv_activity_logs
    WHERE console_id=p_console_id AND event IN('off','sleep','screensaver')
    AND is_authorized=TRUE AND duration_minutes IS NOT NULL
    AND (p_date IS NULL OR created_at::DATE=p_date);

    SELECT COALESCE(SUM(duration_minutes),0) INTO v_una FROM tv_activity_logs
    WHERE console_id=p_console_id AND event IN('off','sleep','screensaver')
    AND is_authorized=FALSE AND duration_minutes IS NOT NULL
    AND (p_date IS NULL OR created_at::DATE=p_date);

    SELECT COALESCE(to_jsonb(s),'null'::JSONB) INTO v_ses
    FROM (SELECT s.id AS "sessionId", s.start_time AS "startTime",
          s.booked_duration_minutes AS "bookedMinutes",
          EXTRACT(EPOCH FROM (NOW()-s.start_time))/60 AS "runningMinutes",
          s.status, cust.name AS "customerName"
          FROM sessions s LEFT JOIN customers cust ON cust.id=s.customer_id
          WHERE s.console_id=p_console_id AND s.status='active'
          ORDER BY s.start_time DESC LIMIT 1) s;

    -- Kalau ada sesi aktif (authorized) yang sedang berjalan → tambahkan running time
    IF v_is_today AND v_ses IS NOT NULL AND v_ses != 'null'::JSONB THEN
        v_run := (v_ses->>'runningMinutes')::NUMERIC::INT;
        IF v_run > 0 THEN
            v_auth := v_auth + v_run;
            v_tot := v_tot + v_run;
        END IF;
    END IF;

    -- Kalau TIDAK ada sesi aktif tapi TV masih ON tanpa sesi (unauthorized) dan
    -- belum ada event OFF/sleep/screensaver penutup → tambahkan running time-nya
    -- juga ke unauthorizedMinutes & totalOnMinutes.
    IF v_is_today AND (v_ses IS NULL OR v_ses = 'null'::JSONB) THEN
        SELECT t.created_at, t.is_authorized INTO v_last_on, v_last_on_auth
        FROM tv_activity_logs t
        WHERE t.console_id = p_console_id AND t.event = 'on'
        ORDER BY t.created_at DESC LIMIT 1;

        IF v_last_on IS NOT NULL AND v_last_on_auth = FALSE AND NOT EXISTS (
            SELECT 1 FROM tv_activity_logs t2
            WHERE t2.console_id = p_console_id
              AND t2.event IN ('off','sleep','screensaver')
              AND t2.created_at > v_last_on
        ) THEN
            v_run := EXTRACT(EPOCH FROM (v_now - v_last_on))/60;
            IF v_run > 0 THEN
                v_una := v_una + v_run;
                v_tot := v_tot + v_run;
            END IF;
        END IF;
    END IF;

    RETURN QUERY SELECT v_logs,v_unauth,v_tot,v_auth,v_una,v_ses,v_ulogs;
END; $$;
