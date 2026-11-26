-- =============================================================================
-- 000050_package_no_extend_and_saved_time_rules.up.sql
--
-- Keputusan bisnis yang diterapkan:
--   1. Sesi paket tidak boleh di-extend (waktu paket berakhir sesuai jadwal).
--   2. Saved time non-member = 24 jam sejak waktu disimpan/dideteksi outage.
--   3. Saved time non-member hanya menit BERBAYAR; bonus promo tidak ikut.
--   4. Food revenue di staff report memakai updated_at (tanggal served),
--      konsisten dengan dashboard/report umum.
-- =============================================================================

ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS is_package BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_sessions_is_package ON sessions (is_package);

-- Backfill best-effort untuk sesi paket lama: notes disimpan sebagai nama paket.
UPDATE sessions s
   SET is_package = TRUE
 WHERE s.is_package = FALSE
   AND EXISTS (
       SELECT 1 FROM rental_packages p WHERE p.name = s.notes
   );

-- Paket yang sudah terlanjur tersimpan dengan expiry 24 jam diberi 7 hari.
UPDATE sessions s
   SET saved_time_expires_at =
       COALESCE(s.interrupted_at, s.updated_at, NOW()) + INTERVAL '7 days'
 WHERE s.status = 'interrupted'
   AND s.is_package = TRUE
   AND s.remaining_minutes > 0
   AND (
       s.saved_time_expires_at IS NULL
       OR s.saved_time_expires_at <
          COALESCE(s.interrupted_at, s.updated_at, NOW()) + INTERVAL '7 days'
   );

-- ── ExtendSession: tolak sesi paket ───────────────────────────────────────

DROP FUNCTION IF EXISTS "__SP__ExtendSession"(UUID, INTEGER, NUMERIC, BOOLEAN, VARCHAR, TEXT);


CREATE OR REPLACE FUNCTION "__SP__ExtendSession"(
    p_session_id UUID,
    p_additional_minutes INTEGER,
    p_cash_received NUMERIC,
    p_pay_now BOOLEAN DEFAULT TRUE,
    p_voucher_code VARCHAR DEFAULT NULL,
    p_notes TEXT DEFAULT NULL
)
RETURNS TABLE(
    session_id UUID,
    session_booked_minutes INT,
    session_end_scheduled TIMESTAMPTZ,
    payment_id UUID,
    payment_amount NUMERIC,
    payment_discount NUMERIC,
    payment_total NUMERIC,
    payment_cash_received NUMERIC,
    payment_change NUMERIC,
    payment_voucher_id UUID,
    payment_status VARCHAR,
    payment_paid_at TIMESTAMPTZ
)
LANGUAGE plpgsql AS $$
DECLARE
    v_session sessions%ROWTYPE;
    v_payment_id UUID;
    v_amount NUMERIC(10,2):=0;
    v_voucher_discount NUMERIC(10,2):=0;
    v_voucher_id UUID:=NULL;
    v_final_amount NUMERIC(10,2):=0;
    v_change NUMERIC(10,2):=0;
    v_new_booked INT;
    v_new_paid INT;
    v_old_paid INT;
    v_new_end TIMESTAMPTZ;
    v_now TIMESTAMPTZ:=NOW();
    v_status VARCHAR;
    v_paid_at TIMESTAMPTZ;
BEGIN
    SELECT * INTO v_session FROM sessions WHERE sessions.id=p_session_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'SESSION_NOT_FOUND'; END IF;
    IF v_session.status!='active' THEN RAISE EXCEPTION 'SESSION_NOT_ACTIVE'; END IF;
    IF p_additional_minutes<=0 THEN RAISE EXCEPTION 'INVALID_DURATION: minimal 1 menit'; END IF;
    IF v_session.is_package
       OR EXISTS (SELECT 1 FROM rental_packages p WHERE p.name = v_session.notes)
    THEN
        RAISE EXCEPTION 'PACKAGE_NO_EXTEND';
    END IF;

    -- Kunci: hitung dari PAID duration, bukan BOOKED (yg sudah termasuk bank waktu & poin)
    v_old_paid:=COALESCE(v_session.paid_duration_minutes,0);
    v_new_paid:=v_old_paid+p_additional_minutes;
    v_new_booked:=v_session.booked_duration_minutes+p_additional_minutes;

    -- Harga = selisih CalculatePrice(paid_baru) - CalculatePrice(paid_lama)
    DECLARE
        v_old_paid_price NUMERIC(10,2):=0;
        v_new_paid_price NUMERIC(10,2);
    BEGIN
        SELECT pc.base_amount INTO v_new_paid_price FROM "__SP__CalculatePrice"(v_session.console_id, v_new_paid) pc;
        IF v_old_paid>0 THEN
            SELECT pc2.base_amount INTO v_old_paid_price FROM "__SP__CalculatePrice"(v_session.console_id, v_old_paid) pc2;
        END IF;
        v_amount:=GREATEST(v_new_paid_price-v_old_paid_price,0);
    END;

    IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code)!='' AND v_amount>0 THEN
        BEGIN
            SELECT va.voucher_id,va.discount_amount INTO v_voucher_id,v_voucher_discount FROM "__SP__ApplyVoucher"(p_voucher_code,v_amount) va;
        EXCEPTION WHEN OTHERS THEN
            v_voucher_discount:=0; v_voucher_id:=NULL;
        END;
        IF v_voucher_id IS NOT NULL THEN
            UPDATE vouchers SET usage_count=usage_count+1,updated_at=v_now WHERE id=v_voucher_id;
        END IF;
    END IF;

    IF v_voucher_discount>v_amount THEN v_voucher_discount:=v_amount; END IF;
    v_final_amount:=GREATEST(v_amount-v_voucher_discount,0);

    IF p_pay_now THEN
        IF p_cash_received<v_final_amount THEN RAISE EXCEPTION 'INSUFFICIENT_CASH: butuh Rp %.0f',v_final_amount; END IF;
        v_change:=p_cash_received-v_final_amount;
        v_status:='paid';
        v_paid_at:=v_now;
    ELSE
        v_change:=0;
        v_status:='pending';
        v_paid_at:=NULL;
    END IF;

    -- BUGFIX: sesi yang sudah overtime harus mendapat tambahan penuh dari NOW().
    -- Sebelumnya memakai end_scheduled_at lama, sehingga end baru bisa tetap
    -- di masa lalu dan waktu yang dibayar hangus.
    v_new_end:=GREATEST(COALESCE(v_session.end_scheduled_at,v_now),v_now)
               +(p_additional_minutes*INTERVAL'1 minute');

    -- BUGFIX: total_price ikut bertambah sebesar amount extension.
    UPDATE sessions
       SET booked_duration_minutes=v_new_booked,
           paid_duration_minutes=v_new_paid,
           end_scheduled_at=v_new_end,
           total_price=COALESCE(v_session.total_price,0)+v_amount,
           updated_at=v_now
     WHERE sessions.id=p_session_id;

    v_payment_id:=uuid_generate_v4();
    INSERT INTO payments(id,session_id,amount,discount_amount,auto_discount_amount,total_payment,payment_method,payment_status,cash_received,change_amount,voucher_id,notes,paid_at,created_at,updated_at)
    VALUES(v_payment_id,p_session_id,v_amount,0,v_voucher_discount,v_final_amount,'cash',v_status,
           CASE WHEN p_pay_now THEN p_cash_received ELSE 0 END, v_change,v_voucher_id,p_notes,v_paid_at,v_now,v_now);

    RETURN QUERY SELECT p_session_id,v_new_booked,v_new_end,
        v_payment_id,v_amount,v_voucher_discount,v_final_amount,
        CASE WHEN p_pay_now THEN p_cash_received ELSE 0 END, v_change,v_voucher_id,v_status,v_paid_at;
END;
$$;


-- ── SaveNonMemberTime: hanya menit berbayar ───────────────────────────────

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
    v_paid_min  INT;
    v_elapsed   INT;
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

    -- Hanya menit BERBAYAR yang disimpan. Bonus promo / bank / poin tidak ikut.
    v_paid_min := COALESCE(NULLIF(v_session.paid_duration_minutes, 0),
        GREATEST(v_session.booked_duration_minutes
                 - COALESCE(v_session.time_bank_minutes_used, 0)
                 - COALESCE(v_session.bonus_minutes_from_points, 0)
                 - COALESCE(v_session.promo_bonus_minutes, 0), 0));
    v_elapsed := GREATEST(EXTRACT(EPOCH FROM (v_now - v_session.start_time))::INT / 60, 0);
    v_remaining := GREATEST(v_paid_min - LEAST(v_elapsed, v_paid_min), 0);
    IF v_remaining <= 0 THEN
        RETURN QUERY SELECT FALSE, 'no_remaining'::TEXT, v_session.id, v_session.console_id, 0, NULL::TIMESTAMPTZ;
        RETURN;
    END IF;

    v_duration := GREATEST(v_elapsed, 1);
    -- Paket: sisa waktu bisa disimpan sampai 1 minggu. Non-paket: 24 jam.
    v_expires := v_now + CASE
        WHEN v_session.is_package THEN INTERVAL '7 days'
        ELSE INTERVAL '24 hours'
    END;

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


-- ── InterruptSession: non-member paid-only, expiry dari deteksi ────────────

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
    v_paid_min   INT := 0;
    v_elapsed    INT := 0;
    v_paid_amount NUMERIC(10,2) := 0;
    v_has_pending BOOLEAN := FALSE;
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

    IF v_is_member THEN
        -- Member: perilaku lama (sisa jadwal penuh).
        IF v_session.end_scheduled_at IS NOT NULL THEN
            v_remaining := GREATEST(
                CEIL(EXTRACT(EPOCH FROM (v_session.end_scheduled_at - v_end)) / 60)::INT,
                0
            );
        END IF;
    ELSE
        -- Non-member: hanya menit berbayar; pending/refunded/free -> 0.
        v_paid_min := COALESCE(NULLIF(v_session.paid_duration_minutes, 0),
            GREATEST(v_session.booked_duration_minutes
                     - COALESCE(v_session.time_bank_minutes_used, 0)
                     - COALESCE(v_session.bonus_minutes_from_points, 0)
                     - COALESCE(v_session.promo_bonus_minutes, 0), 0));
        v_elapsed := GREATEST(EXTRACT(EPOCH FROM (v_end - v_session.start_time))::INT / 60, 0);
        SELECT EXISTS (
            SELECT 1 FROM payments p
             WHERE p.session_id = p_session_id AND p.payment_status = 'pending'
        ) INTO v_has_pending;
        SELECT COALESCE(SUM(p.total_payment), 0) INTO v_paid_amount
          FROM payments p
         WHERE p.session_id = p_session_id AND p.payment_status = 'paid';
        IF v_has_pending OR v_paid_amount <= 0 THEN
            v_remaining := 0;
        ELSE
            v_remaining := GREATEST(v_paid_min - LEAST(v_elapsed, v_paid_min), 0);
        END IF;
    END IF;

    UPDATE sessions
       SET end_time = v_end,
           duration_minutes = v_duration,
           status = 'interrupted',
           interrupted_at = v_now,
           remaining_minutes = v_remaining,
           interrupted_reason = p_reason,
           -- Non-member: berlaku 24 jam sejak waktu terdeteksi/disimpan.
           -- Member: NULL (bank waktu member tidak memakai expiry).
           saved_time_expires_at = CASE
               WHEN v_is_member THEN NULL
               WHEN v_session.is_package THEN v_now + INTERVAL '7 days'
               ELSE v_now + INTERVAL '24 hours'
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


-- ── StaffDailyRevenue: food revenue pakai updated_at (served) ─────────────

DROP FUNCTION IF EXISTS "__SP__StaffDailyRevenue"(DATE, DATE, UUID);


CREATE OR REPLACE FUNCTION "__SP__StaffDailyRevenue"(
    p_start_date DATE,
    p_end_date DATE,
    p_user_id UUID DEFAULT NULL
)
RETURNS TABLE (
    user_id UUID,
    report_date DATE,
    rental_revenue NUMERIC,
    food_revenue NUMERIC,
    daily_rental_revenue NUMERIC,
    membership_revenue NUMERIC,
    rental_trx INT,
    food_trx INT,
    daily_rental_trx INT,
    membership_trx INT
)
LANGUAGE sql AS $$
WITH src AS (
    -- Rental sesi konsol
    SELECT p.created_by AS user_id,
           p.paid_at::DATE AS report_date,
           p.total_payment AS rental_revenue,
           0::NUMERIC AS food_revenue,
           0::NUMERIC AS daily_rental_revenue,
           0::NUMERIC AS membership_revenue,
           1 AS rental_trx,
           0 AS food_trx,
           0 AS daily_rental_trx,
           0 AS membership_trx
      FROM payments p
     WHERE p.payment_status = 'paid'
       AND p.session_id IS NOT NULL
       AND p.paid_at::DATE BETWEEN p_start_date AND p_end_date

    UNION ALL

    -- Membership (payment tanpa sesi)
    SELECT p.created_by,
           p.paid_at::DATE,
           0, 0, 0,
           p.total_payment,
           0, 0, 0, 1
      FROM payments p
     WHERE p.payment_status = 'paid'
       AND p.session_id IS NULL
       AND p.paid_at::DATE BETWEEN p_start_date AND p_end_date

    UNION ALL

    -- Makanan: hanya yang served, diatribusikan ke pembuat order
    SELECT fo.created_by,
           fo.updated_at::DATE,
           0,
           GREATEST(fo.total_amount - COALESCE(fo.discount_amount, 0), 0),
           0, 0,
           0, 1, 0, 0
      FROM food_orders fo
     WHERE fo.status = 'served'
       AND fo.updated_at::DATE BETWEEN p_start_date AND p_end_date

    UNION ALL

    -- Rental harian (bukan cancelled)
    SELECT dr.created_by,
           dr.created_at::DATE,
           0, 0,
           dr.final_amount,
           0,
           0, 0, 1, 0
      FROM daily_rentals dr
     WHERE dr.status <> 'cancelled'
       AND dr.created_at::DATE BETWEEN p_start_date AND p_end_date
)
SELECT user_id,
       report_date,
       COALESCE(SUM(rental_revenue), 0)::NUMERIC(12,2),
       COALESCE(SUM(food_revenue), 0)::NUMERIC(12,2),
       COALESCE(SUM(daily_rental_revenue), 0)::NUMERIC(12,2),
       COALESCE(SUM(membership_revenue), 0)::NUMERIC(12,2),
       COALESCE(SUM(rental_trx), 0)::INT,
       COALESCE(SUM(food_trx), 0)::INT,
       COALESCE(SUM(daily_rental_trx), 0)::INT,
       COALESCE(SUM(membership_trx), 0)::INT
  FROM src
 WHERE p_user_id IS NULL OR user_id = p_user_id
 GROUP BY user_id, report_date
 ORDER BY report_date, user_id;
$$;

-- ── InterruptedSessions: sertakan flag paket ────────────────────────────────
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
$$;
