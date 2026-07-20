-- Migration: 000017_add_tiered_pricing.up.sql
-- Sistem tarif bertingkat (tiered pricing) per konsol.
-- Jika pricing_tiers kosong, fallback ke price_per_hour biasa.

-- =============================================
-- 1. Tambah kolom pricing_tiers JSONB
-- =============================================
ALTER TABLE consoles ADD COLUMN IF NOT EXISTS pricing_tiers JSONB DEFAULT '[]';

-- =============================================
-- 2. SP: byoneCalculatePrice — hitung harga berdasarkan tier
--    Dipakai oleh PreviewPrice, StartSessionWithPayment, ExtendSession
-- =============================================
CREATE OR REPLACE FUNCTION "byoneCalculatePrice"(
    p_console_id       UUID,
    p_duration_minutes INTEGER
)
RETURNS TABLE (
    base_amount     NUMERIC,      -- total harga sebelum diskon
    price_per_hour  NUMERIC,      -- harga per jam default (fallback)
    breakdown       JSONB,        -- rincian per tier [{startMin, endMin, minutes, price, subtotal}]
    tier_count      INT           -- jumlah tier yang dipakai
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_console        consoles%ROWTYPE;
    v_tiers          JSONB;
    v_tier           JSONB;
    v_total          NUMERIC(10,2) := 0;
    v_breakdown      JSONB := '[]'::JSONB;
    v_minute         INT := 0;
    v_tier_start     INT;
    v_tier_end       INT;
    v_tier_price     NUMERIC(10,2);
    v_minutes_in_tier INT;
    v_subtotal       NUMERIC(10,2);
    v_idx            INT := 0;
    v_tier_used      INT := 0;
    v_remaining      INT := p_duration_minutes;
BEGIN
    IF p_duration_minutes <= 0 THEN
        RAISE EXCEPTION 'INVALID_DURATION: Durasi harus > 0';
    END IF;

    -- Ambil data konsol
    SELECT * INTO v_console FROM consoles WHERE consoles.id = p_console_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'CONSOLE_NOT_FOUND: Konsol tidak ditemukan';
    END IF;

    v_tiers := v_console.pricing_tiers;

    -- Jika tidak ada tier, pakai harga flat per jam
    IF v_tiers IS NULL OR jsonb_array_length(v_tiers) = 0 THEN
        v_total := ROUND((p_duration_minutes::NUMERIC / 60.0) * v_console.price_per_hour, 2);
        RETURN QUERY SELECT v_total, v_console.price_per_hour,
            jsonb_build_array(jsonb_build_object(
                'startMinute', 0, 'endMinute', p_duration_minutes,
                'minutes', p_duration_minutes, 'pricePerHour', v_console.price_per_hour,
                'subtotal', v_total
            )), 1;
        RETURN;
    END IF;

    -- Iterasi tier, hitung harga bertingkat
    v_remaining := p_duration_minutes;
    FOR v_idx IN 0..jsonb_array_length(v_tiers)-1 LOOP
        v_tier := v_tiers->v_idx;
        v_tier_start := (v_tier->>'startMinute')::INT;
        v_tier_end   := (v_tier->>'endMinute')::INT;  -- NULL berarti unlimited
        v_tier_price := (v_tier->>'price')::NUMERIC(10,2);

        IF v_remaining <= 0 THEN EXIT; END IF;

        -- Tentukan menit yang masuk tier ini
        IF v_tier_end IS NULL THEN
            v_minutes_in_tier := v_remaining;  -- tier unlimited
        ELSE
            -- Tier ini berlaku dari startMinute sampai endMinute
            -- Hitung berapa menit dari remaining yang jatuh di tier ini
            v_minutes_in_tier := LEAST(v_remaining, v_tier_end - v_tier_start);
            IF v_minutes_in_tier <= 0 THEN CONTINUE; END IF;
        END IF;

        v_subtotal := ROUND((v_minutes_in_tier::NUMERIC / 60.0) * v_tier_price, 2);
        v_total := v_total + v_subtotal;

        v_breakdown := v_breakdown || jsonb_build_object(
            'startMinute', v_tier_start,
            'endMinute', v_tier_end,
            'minutes', v_minutes_in_tier,
            'pricePerHour', v_tier_price,
            'subtotal', v_subtotal
        );

        v_tier_used := v_tier_used + 1;
        v_remaining := v_remaining - v_minutes_in_tier;
    END LOOP;

    -- Sisa menit yang tidak masuk tier manapun → pakai harga per jam default
    IF v_remaining > 0 THEN
        v_subtotal := ROUND((v_remaining::NUMERIC / 60.0) * v_console.price_per_hour, 2);
        v_total := v_total + v_subtotal;
        v_breakdown := v_breakdown || jsonb_build_object(
            'startMinute', p_duration_minutes - v_remaining,
            'endMinute', p_duration_minutes,
            'minutes', v_remaining,
            'pricePerHour', v_console.price_per_hour,
            'subtotal', v_subtotal,
            'fallback', true
        );
        v_tier_used := v_tier_used + 1;
    END IF;

    RETURN QUERY SELECT v_total, v_console.price_per_hour, v_breakdown, v_tier_used;
END;
$$;

-- =============================================
-- 3. Update byonePreviewPrice — pakai tiered pricing
-- =============================================
DROP FUNCTION IF EXISTS "byonePreviewPrice"(UUID, INTEGER, VARCHAR, UUID);
CREATE OR REPLACE FUNCTION "byonePreviewPrice"(
    p_console_id       UUID,
    p_duration_minutes INTEGER,
    p_voucher_code     VARCHAR DEFAULT NULL,
    p_customer_id      UUID DEFAULT NULL
)
RETURNS TABLE (
    price_per_hour   NUMERIC,
    duration_minutes INT,
    base_amount      NUMERIC,
    auto_discount    NUMERIC,
    voucher_discount NUMERIC,
    total_discount   NUMERIC,
    final_amount     NUMERIC,
    voucher_applied  BOOLEAN,
    voucher_name     VARCHAR,
    price_breakdown  JSONB,       -- rincian per tier
    tier_count       INT,
    message          VARCHAR
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_price_per_hour   NUMERIC(10,2);
    v_status           VARCHAR;
    v_amount           NUMERIC(10,2);
    v_breakdown        JSONB;
    v_tier_count       INT;
    v_auto_discount    NUMERIC(10,2) := 0;
    v_voucher_discount NUMERIC(10,2) := 0;
    v_voucher_id       UUID;
    v_voucher_name     VARCHAR;
    v_is_member        BOOLEAN := FALSE;
    v_total_disc       NUMERIC(10,2);
    v_final            NUMERIC(10,2);
    v_voucher_ok       BOOLEAN := FALSE;
    v_msg              VARCHAR := '';
BEGIN
    IF p_duration_minutes < 30 THEN
        RAISE EXCEPTION 'INVALID_DURATION: Durasi minimal 30 menit';
    END IF;

    SELECT c.price_per_hour, c.status
    INTO v_price_per_hour, v_status
    FROM consoles c WHERE c.id = p_console_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'CONSOLE_NOT_FOUND: Konsol tidak ditemukan';
    END IF;

    IF v_status != 'available' THEN
        v_msg := 'Konsol sedang tidak tersedia (status: ' || v_status || ')';
    END IF;

    -- Hitung harga menggunakan tiered pricing
    SELECT pc.base_amount, pc.breakdown, pc.tier_count
    INTO v_amount, v_breakdown, v_tier_count
    FROM "byoneCalculatePrice"(p_console_id, p_duration_minutes) pc;

    IF p_customer_id IS NOT NULL THEN
        SELECT COALESCE(is_member, FALSE) INTO v_is_member
        FROM customers WHERE id = p_customer_id;
    END IF;

    v_auto_discount := "byoneEvaluateDiscountRules"(v_amount, NOW(), v_is_member);

    IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code) != '' THEN
        BEGIN
            SELECT va.voucher_id, va.discount_amount
            INTO v_voucher_id, v_voucher_discount
            FROM "byoneApplyVoucher"(p_voucher_code, v_amount) va;
            v_voucher_ok := TRUE;
            SELECT name INTO v_voucher_name FROM vouchers WHERE id = v_voucher_id;
        EXCEPTION WHEN OTHERS THEN
            v_voucher_discount := 0; v_voucher_ok := FALSE;
            v_msg := v_msg || ' Voucher tidak valid.';
        END;
    END IF;

    v_total_disc := v_auto_discount + v_voucher_discount;
    IF v_total_disc > v_amount THEN v_total_disc := v_amount; END IF;
    v_final := GREATEST(v_amount - v_total_disc, 0);

    IF v_msg = '' AND v_status = 'available' THEN
        v_msg := 'Konsol tersedia. Total: Rp ' || v_final::TEXT ||
                 ' untuk ' || p_duration_minutes::TEXT || ' menit';
    END IF;

    RETURN QUERY SELECT v_price_per_hour, p_duration_minutes, v_amount,
        v_auto_discount, v_voucher_discount, v_total_disc,
        v_final, v_voucher_ok, v_voucher_name,
        v_breakdown, v_tier_count, v_msg;
END;
$$;

-- =============================================
-- 4. Update byoneStartSessionWithPayment — pakai tiered pricing
-- =============================================
CREATE OR REPLACE FUNCTION "byoneStartSessionWithPayment"(
    p_console_id              UUID,
    p_customer_id             UUID,
    p_notes                   TEXT,
    p_booked_duration_minutes INTEGER,
    p_cash_received           NUMERIC,
    p_voucher_code            VARCHAR DEFAULT NULL
)
RETURNS TABLE (
    session_id              UUID,
    session_status          VARCHAR,
    session_start_time      TIMESTAMPTZ,
    session_booked_minutes  INT,
    session_end_scheduled   TIMESTAMPTZ,
    payment_id              UUID,
    base_amount             NUMERIC,
    discount_amount         NUMERIC,
    auto_discount_amount    NUMERIC,
    total_payment           NUMERIC,
    cash_received           NUMERIC,
    change_amount           NUMERIC,
    voucher_id              UUID,
    paid_at                 TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_session_id       UUID;
    v_payment_id       UUID;
    v_console_status   VARCHAR;
    v_price_per_hour   NUMERIC(10,2);
    v_end_scheduled    TIMESTAMPTZ;
    v_amount           NUMERIC(10,2);
    v_voucher_discount NUMERIC(10,2) := 0;
    v_auto_discount    NUMERIC(10,2) := 0;
    v_total_discount   NUMERIC(10,2);
    v_final_amount     NUMERIC(10,2);
    v_change           NUMERIC(10,2);
    v_voucher_id       UUID := NULL;
    v_is_member        BOOLEAN := FALSE;
    v_now              TIMESTAMPTZ := NOW();
BEGIN
    IF p_booked_duration_minutes < 30 THEN
        RAISE EXCEPTION 'INVALID_DURATION: Durasi minimal 30 menit';
    END IF;

    SELECT c.status, c.price_per_hour
    INTO v_console_status, v_price_per_hour
    FROM consoles c WHERE c.id = p_console_id FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'CONSOLE_NOT_FOUND: Konsol tidak ditemukan';
    END IF;
    IF v_console_status != 'available' THEN
        RAISE EXCEPTION 'CONSOLE_NOT_AVAILABLE: Konsol tidak tersedia (status: %)', v_console_status;
    END IF;
    IF EXISTS (
        SELECT 1 FROM sessions s2
        WHERE s2.console_id = p_console_id AND s2.status = 'active'
    ) THEN
        RAISE EXCEPTION 'SESSION_ALREADY_ACTIVE: Konsol masih memiliki sesi aktif';
    END IF;

    -- Hitung harga dengan tiered pricing
    SELECT pc.base_amount INTO v_amount
    FROM "byoneCalculatePrice"(p_console_id, p_booked_duration_minutes) pc;

    v_end_scheduled := v_now + (p_booked_duration_minutes * INTERVAL '1 minute');

    IF p_customer_id IS NOT NULL THEN
        SELECT COALESCE(is_member, FALSE) INTO v_is_member
        FROM customers WHERE id = p_customer_id;
    END IF;

    v_auto_discount := "byoneEvaluateDiscountRules"(v_amount, v_now, v_is_member);

    IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code) != '' THEN
        SELECT va.voucher_id, va.discount_amount
        INTO v_voucher_id, v_voucher_discount
        FROM "byoneApplyVoucher"(p_voucher_code, v_amount) va;
        UPDATE vouchers SET usage_count = usage_count + 1, updated_at = v_now
        WHERE id = v_voucher_id;
    END IF;

    v_total_discount := v_auto_discount + v_voucher_discount;
    IF v_total_discount > v_amount THEN
        v_total_discount := v_amount;
        IF v_auto_discount > v_amount THEN
            v_auto_discount := v_amount; v_voucher_discount := 0;
        ELSE
            v_voucher_discount := v_amount - v_auto_discount;
        END IF;
    END IF;

    v_final_amount := GREATEST(v_amount - v_total_discount, 0);
    IF p_cash_received < v_final_amount THEN
        RAISE EXCEPTION 'INSUFFICIENT_CASH: Uang kurang. Butuh Rp %.0f, diterima Rp %.0f',
            v_final_amount, p_cash_received;
    END IF;
    v_change := p_cash_received - v_final_amount;

    v_session_id := uuid_generate_v4();
    INSERT INTO sessions (
        id, console_id, customer_id, start_time,
        booked_duration_minutes, end_scheduled_at,
        total_price, status, notes, created_at, updated_at
    ) VALUES (
        v_session_id, p_console_id, p_customer_id,
        v_now, p_booked_duration_minutes, v_end_scheduled,
        v_amount, 'active', p_notes, v_now, v_now
    );

    UPDATE consoles SET status = 'in_use', updated_at = v_now WHERE consoles.id = p_console_id;

    v_payment_id := uuid_generate_v4();
    INSERT INTO payments (
        id, session_id, amount, discount_amount, auto_discount_amount,
        total_payment, payment_method, payment_status,
        cash_received, change_amount, voucher_id, notes,
        paid_at, created_at, updated_at
    ) VALUES (
        v_payment_id, v_session_id,
        v_amount, v_voucher_discount, v_auto_discount,
        v_final_amount, 'cash', 'paid',
        p_cash_received, v_change, v_voucher_id, p_notes,
        v_now, v_now, v_now
    );

    RETURN QUERY SELECT
        v_session_id, 'active'::VARCHAR, v_now, p_booked_duration_minutes, v_end_scheduled,
        v_payment_id, v_amount, v_voucher_discount, v_auto_discount, v_final_amount,
        p_cash_received, v_change, v_voucher_id, v_now;
END;
$$;

-- =============================================
-- 5. Update byoneExtendSession — pakai tiered pricing
-- =============================================
CREATE OR REPLACE FUNCTION "byoneExtendSession"(
    p_session_id              UUID,
    p_additional_minutes      INTEGER,
    p_cash_received           NUMERIC,
    p_voucher_code            VARCHAR DEFAULT NULL,
    p_notes                   TEXT DEFAULT NULL
)
RETURNS TABLE (
    session_id                  UUID,
    session_booked_minutes      INT,
    session_end_scheduled       TIMESTAMPTZ,
    payment_id                  UUID,
    payment_amount              NUMERIC,
    payment_discount            NUMERIC,
    payment_total               NUMERIC,
    payment_cash_received       NUMERIC,
    payment_change              NUMERIC,
    payment_voucher_id          UUID,
    payment_status              VARCHAR,
    payment_paid_at             TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_session           sessions%ROWTYPE;
    v_payment_id        UUID;
    v_amount            NUMERIC(10,2);
    v_voucher_discount  NUMERIC(10,2) := 0;
    v_voucher_id        UUID := NULL;
    v_final_amount      NUMERIC(10,2);
    v_change            NUMERIC(10,2);
    v_new_booked        INT;
    v_new_end           TIMESTAMPTZ;
    v_now               TIMESTAMPTZ := NOW();
BEGIN
    SELECT * INTO v_session FROM sessions WHERE sessions.id = p_session_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'SESSION_NOT_FOUND: Sesi tidak ditemukan'; END IF;
    IF v_session.status != 'active' THEN RAISE EXCEPTION 'SESSION_NOT_ACTIVE: Sesi sudah tidak aktif'; END IF;
    IF p_additional_minutes < 30 THEN RAISE EXCEPTION 'INVALID_DURATION: Minimal tambah waktu 30 menit'; END IF;

    -- Hitung ulang total harga untuk SELURUH durasi (existing + additional)
    -- lalu kurangi yang sudah dibayar untuk dapat tambahan biaya
    -- Pendekatan: hitung harga untuk booked_duration + additional, lalu kurangi total_price saat ini
    DECLARE
        v_new_total NUMERIC(10,2);
        v_old_total NUMERIC(10,2) := v_session.total_price;
        v_total_minutes INT := v_session.booked_duration_minutes + p_additional_minutes;
    BEGIN
        SELECT pc.base_amount INTO v_new_total
        FROM "byoneCalculatePrice"(v_session.console_id, v_total_minutes) pc;
        v_amount := v_new_total - v_old_total;
        IF v_amount < 0 THEN v_amount := 0; END IF;
    END;

    IF p_voucher_code IS NOT NULL AND TRIM(p_voucher_code) != '' THEN
        BEGIN
            SELECT va.voucher_id, va.discount_amount INTO v_voucher_id, v_voucher_discount
            FROM "byoneApplyVoucher"(p_voucher_code, v_amount) va;
        EXCEPTION WHEN OTHERS THEN v_voucher_discount := 0; v_voucher_id := NULL; END;
        IF v_voucher_id IS NOT NULL THEN
            UPDATE vouchers SET usage_count = usage_count + 1, updated_at = v_now WHERE id = v_voucher_id;
        END IF;
    END IF;

    IF v_voucher_discount > v_amount THEN v_voucher_discount := v_amount; END IF;
    v_final_amount := v_amount - v_voucher_discount;
    IF v_final_amount < 0 THEN v_final_amount := 0; END IF;
    v_change := GREATEST(p_cash_received - v_final_amount, 0);

    v_new_booked := v_session.booked_duration_minutes + p_additional_minutes;
    v_new_end := v_now + (v_new_booked * INTERVAL '1 minute');

    UPDATE sessions SET
        booked_duration_minutes = v_new_booked,
        end_scheduled_at = v_new_end,
        total_price = v_session.total_price + v_amount,
        updated_at = v_now
    WHERE sessions.id = p_session_id;

    v_payment_id := uuid_generate_v4();
    INSERT INTO payments (
        id, session_id, amount, discount_amount, auto_discount_amount,
        total_payment, payment_method, payment_status,
        cash_received, change_amount, voucher_id, notes,
        created_at, updated_at
    ) VALUES (
        v_payment_id, p_session_id,
        v_amount, v_voucher_discount, 0,
        v_final_amount, 'cash', 'pending',
        p_cash_received, v_change, v_voucher_id, p_notes,
        v_now, v_now
    );

    RETURN QUERY SELECT
        p_session_id, v_new_booked, v_new_end,
        v_payment_id, v_amount, v_voucher_discount, v_final_amount,
        p_cash_received, v_change, v_voucher_id,
        'pending'::VARCHAR, NULL::TIMESTAMPTZ;
END;
$$;
