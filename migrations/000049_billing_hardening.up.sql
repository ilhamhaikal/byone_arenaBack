-- =============================================================================
-- 000049_billing_hardening.up.sql
--
-- Perbaikan bug teknis billing TANPA mengubah aturan bisnis:
--   1. Race pemakaian voucher: trigger mencegah usage_count melebihi max_usage.
--   2. Konfirmasi pending payment ditolak untuk sesi cancelled/interrupted/
--      recovered (sesi yang tidak boleh ditagih lagi).
--   3. Konfirmasi pending cash tanpa cashReceived memakai uang pas (total).
--   4. Pelunasan banyak pending cash tidak lagi menghitung cash berlebih.
--   5. ExtendSession: sesi overtime mendapat tambahan penuh dari NOW(), dan
--      sessions.total_price ikut bertambah sebesar payment extension.
--   6. Food order served tidak bisa di-cancel (sesuai dokumentasi API).
--
-- Semua logika query tetap di stored procedure/trigger.
-- =============================================================================

-- ── 1. Guard limit voucher (atomik, semua jalur pemakaian) ─────────────────
CREATE OR REPLACE FUNCTION "__SP__EnforceVoucherUsageLimit"()
RETURNS TRIGGER
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.max_usage > 0 AND NEW.usage_count > NEW.max_usage THEN
        RAISE EXCEPTION 'VOUCHER_LIMIT_REACHED';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_voucher_usage_limit ON vouchers;
CREATE TRIGGER trg_voucher_usage_limit
BEFORE UPDATE OF usage_count ON vouchers
FOR EACH ROW EXECUTE FUNCTION "__SP__EnforceVoucherUsageLimit"();

-- ── 2. ExtendSession: perbaiki end time overtime + total_price ─────────────
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

-- ── 3. ConfirmExtendPayment: status sesi + default cash ─────────────────────
DROP FUNCTION IF EXISTS "__SP__ConfirmExtendPayment"(UUID, NUMERIC);

CREATE OR REPLACE FUNCTION "__SP__ConfirmExtendPayment"(p_payment_id UUID, p_cash_received NUMERIC DEFAULT NULL)
RETURNS TABLE(payment_id UUID, payment_status VARCHAR, paid_at TIMESTAMPTZ, total_payment NUMERIC, cash_received NUMERIC, change_amount NUMERIC)
LANGUAGE plpgsql AS $$
DECLARE
    v_payment payments%ROWTYPE;
    v_now TIMESTAMPTZ:=NOW();
    v_pending BIGINT;
    v_console_id UUID;
    v_cash NUMERIC(10,2);
    v_change NUMERIC(10,2);
    v_session_status VARCHAR;
BEGIN
    SELECT * INTO v_payment FROM payments WHERE payments.id=p_payment_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'PAYMENT_NOT_FOUND'; END IF;
    IF v_payment.payment_status!='pending' THEN RAISE EXCEPTION 'PAYMENT_NOT_PENDING'; END IF;

    IF v_payment.session_id IS NOT NULL THEN
        SELECT s.status INTO v_session_status FROM sessions s WHERE s.id=v_payment.session_id;
        IF v_session_status IN ('cancelled','interrupted','recovered') THEN
            RAISE EXCEPTION 'SESSION_NOT_BILLABLE: %', v_session_status;
        END IF;
    END IF;

    IF p_cash_received IS NOT NULL THEN
        IF p_cash_received<v_payment.total_payment THEN
            RAISE EXCEPTION 'INSUFFICIENT_CASH: butuh Rp %.0f',v_payment.total_payment;
        END IF;
        v_cash:=p_cash_received;
        v_change:=p_cash_received-v_payment.total_payment;
    ELSE
        -- Tanpa input cash: untuk cash anggap uang pas; non-cash pakai nilai tersimpan.
        IF v_payment.payment_method='cash' THEN
            v_cash:=v_payment.total_payment;
            v_change:=0;
        ELSE
            v_cash:=v_payment.cash_received;
            v_change:=v_payment.change_amount;
        END IF;
    END IF;

    UPDATE payments SET payment_status='paid',paid_at=v_now,cash_received=v_cash,change_amount=v_change,updated_at=v_now
     WHERE payments.id=p_payment_id;

    SELECT COUNT(*) INTO v_pending FROM payments
     WHERE payments.session_id=v_payment.session_id AND payments.payment_status='pending';
    IF v_pending=0 THEN
        SELECT s.console_id INTO v_console_id FROM sessions s WHERE s.id=v_payment.session_id;
        IF v_console_id IS NOT NULL THEN
            UPDATE tv_notifications SET is_active=false,updated_at=v_now
             WHERE title='Pembayaran Tertunda' AND is_active=true
               AND (target_all=true OR target_console_ids::jsonb@>to_jsonb(v_console_id::TEXT));
        END IF;
    END IF;

    RETURN QUERY SELECT p_payment_id,'paid'::VARCHAR,v_now,v_payment.total_payment,v_cash,v_change;
END;
$$;

-- ── 4. ConfirmSessionPendingPayments: status sesi + cash tidak overcount ────
DROP FUNCTION IF EXISTS "__SP__ConfirmSessionPendingPayments"(UUID, NUMERIC);

CREATE OR REPLACE FUNCTION "__SP__ConfirmSessionPendingPayments"(p_session_id UUID, p_cash_received NUMERIC)
RETURNS TABLE(session_id UUID, confirmed_count INT, total_paid NUMERIC, cash_received NUMERIC, change_amount NUMERIC)
LANGUAGE plpgsql AS $$
DECLARE
    v_total NUMERIC(10,2);
    v_count INT;
    v_change NUMERIC(10,2);
    v_now TIMESTAMPTZ:=NOW();
    v_last_payment_id UUID;
    v_console_id UUID;
    v_session_status VARCHAR;
BEGIN
    SELECT s.status INTO v_session_status FROM sessions s WHERE s.id=p_session_id;
    IF v_session_status IN ('cancelled','interrupted','recovered') THEN
        RAISE EXCEPTION 'SESSION_NOT_BILLABLE: %', v_session_status;
    END IF;

    PERFORM 1 FROM payments p
     WHERE p.session_id=p_session_id AND p.payment_status='pending' FOR UPDATE;
    SELECT COALESCE(SUM(p.total_payment),0),COUNT(*) INTO v_total,v_count
      FROM payments p
     WHERE p.session_id=p_session_id AND p.payment_status='pending';
    IF v_count=0 THEN RAISE EXCEPTION 'NO_PENDING_PAYMENT'; END IF;
    IF p_cash_received<v_total THEN
        RAISE EXCEPTION 'INSUFFICIENT_CASH: butuh Rp %.0f',v_total;
    END IF;
    v_change:=p_cash_received-v_total;

    -- Pilih satu payment yang menampung uang cash/kembalian. Payment lain
    -- di-set 0 agar jumlah cash_received tidak melebihi uang yang diterima.
    SELECT p.id INTO v_last_payment_id FROM payments p
     WHERE p.session_id=p_session_id AND p.payment_status='pending'
     ORDER BY p.created_at DESC, p.id DESC
     LIMIT 1;

    UPDATE payments p SET payment_status='paid',paid_at=v_now,cash_received=0,change_amount=0,updated_at=v_now
     WHERE p.session_id=p_session_id AND p.payment_status='pending';

    UPDATE payments p SET cash_received=p_cash_received,change_amount=v_change,updated_at=v_now
     WHERE p.id=v_last_payment_id;

    SELECT s.console_id INTO v_console_id FROM sessions s WHERE s.id=p_session_id;
    IF v_console_id IS NOT NULL THEN
        UPDATE tv_notifications SET is_active=false,updated_at=v_now
         WHERE title='Pembayaran Tertunda' AND is_active=true
           AND (target_all=true OR target_console_ids::jsonb@>to_jsonb(v_console_id::TEXT));
    END IF;

    RETURN QUERY SELECT p_session_id,v_count,v_total,p_cash_received,v_change;
END;
$$;

-- ── 5. Food order: served tidak boleh di-cancel ────────────────────────────
DROP FUNCTION IF EXISTS "__SP__UpdateFoodOrderStatus"(UUID, VARCHAR);

CREATE OR REPLACE FUNCTION "__SP__UpdateFoodOrderStatus"(p_order_id UUID, p_status VARCHAR)
RETURNS VOID
LANGUAGE plpgsql AS $$
DECLARE
    v_current_status VARCHAR;
    v_item RECORD;
BEGIN
    SELECT status INTO v_current_status FROM food_orders WHERE id=p_order_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'ORDER_NOT_FOUND'; END IF;
    IF v_current_status='cancelled' THEN RAISE EXCEPTION 'ORDER_CANCELLED'; END IF;

    -- Sesuai dokumentasi API: served adalah status final, tidak bisa diubah
    -- lagi (termasuk menjadi cancelled). Mencegah revenue hilang padahal
    -- makanan sudah disajikan dan stok sudah terpakai.
    IF v_current_status='served' THEN RAISE EXCEPTION 'ORDER_SERVED'; END IF;

    IF p_status='cancelled' AND v_current_status IN ('pending','preparing') THEN
        FOR v_item IN
            SELECT menu_item_id, quantity FROM food_order_items WHERE order_id=p_order_id
        LOOP
            UPDATE menu_items SET stock = stock + v_item.quantity, updated_at = NOW()
             WHERE id = v_item.menu_item_id;
        END LOOP;
    END IF;
    UPDATE food_orders SET status=p_status,updated_at=NOW() WHERE id=p_order_id;
END;
$$;