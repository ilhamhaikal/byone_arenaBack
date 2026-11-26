-- Rollback 000053: kembalikan ReportSummary 000052 dan ExtendSession lama.

CREATE OR REPLACE FUNCTION "__SP__ReportSummary"(p_start_date date, p_end_date date)
 RETURNS TABLE(report jsonb)
 LANGUAGE plpgsql
AS $function$
DECLARE v_total_days INT; v_result JSONB;
BEGIN
    IF p_start_date IS NULL THEN p_start_date:=CURRENT_DATE-7; END IF;
    IF p_end_date IS NULL THEN p_end_date:=CURRENT_DATE; END IF;
    IF p_start_date>p_end_date THEN RAISE EXCEPTION 'INVALID_DATE_RANGE'; END IF;
    v_total_days:=p_end_date-p_start_date+1;
    WITH session_rev AS (
        SELECT COALESCE(SUM(p.total_payment),0) AS revenue, COALESCE(SUM(p.amount),0) AS base_amount, COALESCE(SUM(p.discount_amount),0) AS discount, COALESCE(SUM(p.auto_discount_amount),0) AS auto_discount, COALESCE(SUM(p.cash_received),0) AS cash_received, COALESCE(SUM(p.change_amount),0) AS change_amount, COUNT(*)::INT AS trx_count, COUNT(p.voucher_id)::INT AS voucher_trx
        FROM payments p WHERE p.payment_status='paid' AND p.session_id IS NOT NULL AND p.paid_at::DATE BETWEEN p_start_date AND p_end_date),
    rental_rev AS (
        SELECT COALESCE(SUM(d.final_amount),0) AS revenue, COALESCE(SUM(d.total_amount),0) AS base_amount, COALESCE(SUM(d.discount_amount),0) AS discount, 0::NUMERIC AS auto_discount, COALESCE(SUM(d.final_amount),0) AS cash_received, 0::NUMERIC AS change_amount, COUNT(*)::INT AS trx_count, COUNT(d.voucher_id)::INT AS voucher_trx
        FROM daily_rentals d WHERE d.created_at::DATE BETWEEN p_start_date AND p_end_date),
    member_rev AS (
        SELECT COALESCE(SUM(p.total_payment),0) AS revenue, COALESCE(SUM(p.amount),0) AS base_amount, COALESCE(SUM(p.discount_amount),0) AS discount, 0::NUMERIC AS auto_discount, COALESCE(SUM(p.cash_received),0) AS cash_received, COALESCE(SUM(p.change_amount),0) AS change_amount, COUNT(*)::INT AS trx_count, 0::INT AS voucher_trx
        FROM payments p WHERE p.payment_status='paid' AND p.session_id IS NULL AND p.paid_at::DATE BETWEEN p_start_date AND p_end_date),
    food_rev AS (
        SELECT COALESCE(SUM(fo.total_amount - COALESCE(fo.discount_amount,0)),0) AS revenue,
               COALESCE(SUM(fo.total_amount),0) AS base_amount,
               COALESCE(SUM(fo.discount_amount),0) AS discount,
               COALESCE(SUM(fh.hpp),0) AS hpp,
               COALESCE(SUM(CASE WHEN fo.notes LIKE 'Bundling paket %' THEN 0 ELSE fh.hpp END),0) AS hpp_real,
               COUNT(*)::INT AS order_count
        FROM food_orders fo
        LEFT JOIN LATERAL (
            SELECT COALESCE(SUM(foi.quantity * COALESCE(foi.unit_hpp,0)),0) AS hpp
            FROM food_order_items foi WHERE foi.order_id = fo.id
        ) fh ON TRUE
        WHERE fo.status='served' AND fo.updated_at::DATE BETWEEN p_start_date AND p_end_date),
    combined AS (
        SELECT sr.revenue+rr.revenue+mr.revenue+fr.revenue AS total_revenue,
               sr.base_amount+rr.base_amount+mr.base_amount+fr.base_amount AS total_base,
               sr.discount+rr.discount+mr.discount+fr.discount AS total_discount,
               sr.auto_discount+rr.auto_discount+mr.auto_discount AS total_auto,
               sr.cash_received+rr.cash_received+mr.cash_received AS total_cash,
               sr.change_amount+rr.change_amount+mr.change_amount AS total_change,
               sr.trx_count+rr.trx_count+mr.trx_count+fr.order_count AS total_trx,
               sr.voucher_trx+rr.voucher_trx+mr.voucher_trx AS voucher_trx,
               rr.revenue AS rental_rev, rr.trx_count AS rental_count,
               mr.revenue AS member_rev, mr.trx_count AS member_count,
               fr.revenue AS food_rev, fr.base_amount AS food_base, fr.discount AS food_disc, fr.hpp AS food_hpp, fr.hpp_real AS food_hpp_real, fr.order_count AS food_count
        FROM session_rev sr CROSS JOIN rental_rev rr CROSS JOIN member_rev mr CROSS JOIN food_rev fr),
    session_agg AS (
        -- Sesi = transaksi rental awal. Sesi pemulihan (recovered_from_session_id)
        -- adalah lanjutan dari sesi awal, jadi tidak dihitung sebagai sesi baru.
        -- Durasi = total waktu aktual semua segmen (completed/interrupted/recovered).
        SELECT
            COUNT(*) FILTER (WHERE s.recovered_from_session_id IS NULL)::INT AS total_sessions,
            COALESCE(FLOOR(SUM(GREATEST(EXTRACT(EPOCH FROM (
                COALESCE(s.end_time, s.updated_at, s.start_time) - s.start_time
            )), 0)) / 60), 0)::INT AS total_play_minutes,
            COALESCE(ROUND((
                SUM(GREATEST(EXTRACT(EPOCH FROM (
                    COALESCE(s.end_time, s.updated_at, s.start_time) - s.start_time
                )), 0)) / 60.0
            ) / NULLIF(COUNT(*) FILTER (WHERE s.recovered_from_session_id IS NULL), 0)), 0)::INT AS avg_duration_minutes
        FROM sessions s
        WHERE s.status IN ('completed','interrupted','recovered')
          AND (s.start_time AT TIME ZONE 'Asia/Jakarta')::DATE BETWEEN p_start_date AND p_end_date),
    voucher_usage AS (
        SELECT COALESCE(jsonb_agg(row_to_json(v) ORDER BY v."usageCount" DESC),'[]'::JSONB) AS list
        FROM (SELECT v.name AS "voucherName", v.code AS "voucherCode", v.discount_type AS "discountType", COUNT(*)::INT AS "usageCount", SUM(src.discount_amount)::NUMERIC(10,2) AS "totalDiscount"
              FROM (SELECT p.voucher_id, p.discount_amount FROM payments p WHERE p.payment_status='paid' AND p.voucher_id IS NOT NULL AND p.paid_at::DATE BETWEEN p_start_date AND p_end_date UNION ALL SELECT d.voucher_id, d.discount_amount FROM daily_rentals d WHERE d.voucher_id IS NOT NULL AND d.created_at::DATE BETWEEN p_start_date AND p_end_date) src
              JOIN vouchers v ON v.id=src.voucher_id GROUP BY v.id,v.name,v.code,v.discount_type) v),
    console_usage AS (
        SELECT COALESCE(jsonb_agg(row_to_json(c) ORDER BY c."totalMinutes" DESC),'[]'::JSONB) AS list
        FROM (SELECT c.name AS "consoleName", c.console_type AS "consoleType",
                     COUNT(*) FILTER (WHERE s.recovered_from_session_id IS NULL)::INT AS "totalSessions",
                     COALESCE(FLOOR(SUM(GREATEST(EXTRACT(EPOCH FROM (
                         COALESCE(s.end_time, s.updated_at, s.start_time) - s.start_time
                     )), 0)) / 60), 0)::INT AS "totalMinutes"
              FROM sessions s JOIN consoles c ON c.id=s.console_id
              WHERE s.status IN ('completed','interrupted','recovered')
                AND (s.start_time AT TIME ZONE 'Asia/Jakarta')::DATE BETWEEN p_start_date AND p_end_date
              GROUP BY c.id,c.name,c.console_type) c),
    daily AS (
        SELECT COALESCE(jsonb_agg(row_to_json(d) ORDER BY d."date"),'[]'::JSONB) AS list
        FROM (SELECT dd::DATE AS "date",
                     COALESCE(pd.rev,0)+COALESCE(drd.rev,0)+COALESCE(md.rev,0)+COALESCE(fd.food_rev,0) AS "revenue",
                     COALESCE(pd.trx,0)+COALESCE(drd.trx,0)+COALESCE(md.trx,0)+COALESCE(fd.food_count,0) AS "transactions",
                     COALESCE(sd.sessions,0) AS "sessions", COALESCE(sd.playMinutes,0) AS "playMinutes",
                     COALESCE(drd.rental_rev,0) AS "rentalRevenue", COALESCE(drd.rental_count,0) AS "dailyRentals",
                     COALESCE(md.member_rev,0) AS "membershipRevenue", COALESCE(md.member_count,0) AS "memberships",
                     COALESCE(fd.food_rev,0) AS "foodRevenue", COALESCE(fd.food_count,0) AS "foodOrders"
              FROM generate_series(p_start_date,p_end_date,'1 day'::INTERVAL) dd
              LEFT JOIN LATERAL (SELECT COALESCE(SUM(p.total_payment),0) AS rev, COUNT(*)::INT AS trx FROM payments p WHERE p.payment_status='paid' AND p.session_id IS NOT NULL AND p.paid_at::DATE=dd) pd ON TRUE
              LEFT JOIN LATERAL (SELECT COALESCE(SUM(d2.final_amount),0) AS rev, COALESCE(SUM(d2.total_amount),0) AS rental_rev, COUNT(*)::INT AS trx, COUNT(*)::INT AS rental_count FROM daily_rentals d2 WHERE d2.created_at::DATE=dd) drd ON TRUE
              LEFT JOIN LATERAL (SELECT COALESCE(SUM(p2.total_payment),0) AS rev, COALESCE(SUM(p2.amount),0) AS member_rev, COUNT(*)::INT AS trx, COUNT(*)::INT AS member_count FROM payments p2 WHERE p2.payment_status='paid' AND p2.session_id IS NULL AND p2.paid_at::DATE=dd) md ON TRUE
              LEFT JOIN LATERAL (
                  SELECT
                      COUNT(*) FILTER (WHERE s2.recovered_from_session_id IS NULL)::INT AS sessions,
                      COALESCE(FLOOR(SUM(GREATEST(EXTRACT(EPOCH FROM (
                          COALESCE(s2.end_time, s2.updated_at, s2.start_time) - s2.start_time
                      )), 0)) / 60), 0)::INT AS playMinutes
                  FROM sessions s2
                  WHERE s2.status IN ('completed','interrupted','recovered')
                    AND (s2.start_time AT TIME ZONE 'Asia/Jakarta')::DATE=dd
              ) sd ON TRUE
              LEFT JOIN LATERAL (SELECT COALESCE(SUM(fo3.total_amount - COALESCE(fo3.discount_amount,0)),0) AS food_rev, COUNT(*)::INT AS food_count FROM food_orders fo3 WHERE fo3.status='served' AND fo3.updated_at::DATE=dd) fd ON TRUE) d),
    discount_rules_summary AS (
        SELECT COALESCE(jsonb_agg(row_to_json(dr)),'[]'::JSONB) AS list
        FROM (SELECT dr.name AS "ruleName", dr.rule_type AS "ruleType", dr.discount_type AS "discountType", dr.discount_value AS "discountValue", dr.is_active AS "isActive" FROM discount_rules dr WHERE dr.is_active=TRUE ORDER BY dr.rule_type,dr.name) dr),
    food_top_items AS (
        SELECT COALESCE(jsonb_agg(row_to_json(fi) ORDER BY fi."revenue" DESC),'[]'::JSONB) AS list
        FROM (SELECT mi.name AS "itemName", mi.category AS "category",
                     SUM(foi.quantity)::INT AS "quantitySold",
                     SUM(foi.subtotal)::NUMERIC(10,2) AS "revenue",
                     SUM(foi.quantity * COALESCE(foi.unit_hpp,0))::NUMERIC(10,2) AS "hpp",
                     SUM(CASE WHEN fo2.notes LIKE 'Bundling paket %' THEN 0
                              ELSE foi.subtotal - foi.quantity * COALESCE(foi.unit_hpp,0)
                         END)::NUMERIC(10,2) AS "netRevenue"
              FROM food_order_items foi JOIN food_orders fo2 ON fo2.id=foi.order_id JOIN menu_items mi ON mi.id=foi.menu_item_id
              WHERE fo2.status='served' AND fo2.updated_at::DATE BETWEEN p_start_date AND p_end_date GROUP BY mi.id,mi.name,mi.category) fi)
    SELECT jsonb_build_object(
        'period',jsonb_build_object('startDate',p_start_date,'endDate',p_end_date,'totalDays',v_total_days),
        'revenue',jsonb_build_object('totalRevenue',cb.total_revenue,'totalBaseAmount',cb.total_base,'voucherDiscount',cb.total_discount,'autoDiscount',cb.total_auto,'totalDiscount',cb.total_discount+cb.total_auto,'totalCashReceived',cb.total_cash,'totalChange',cb.total_change,'dailyRentalRevenue',cb.rental_rev,'dailyRentalCount',cb.rental_count,'membershipRevenue',cb.member_rev,'membershipCount',cb.member_count,'foodSalesRevenue',cb.food_rev,'foodSalesBaseAmount',cb.food_base,'foodSalesDiscount',cb.food_disc,'foodSalesHpp',cb.food_hpp,'foodSalesNetRevenue',cb.food_rev - cb.food_hpp_real,'foodSalesCount',cb.food_count),
        'transactions',jsonb_build_object('totalTransactions',cb.total_trx,'voucherTransactions',cb.voucher_trx,'averagePerDay',ROUND(cb.total_trx::NUMERIC/GREATEST(v_total_days,1),1)),
        'sessions',jsonb_build_object('totalSessions',sa.total_sessions,'totalPlayMinutes',sa.total_play_minutes,'totalPlayHours',ROUND(sa.total_play_minutes::NUMERIC/60.0,1),'averageMinutes',sa.avg_duration_minutes),
        'vouchers',vu.list,'consoles',cu.list,'dailyBreakdown',d.list,'activeDiscountRules',dr.list,
        'foodSales',jsonb_build_object('totalRevenue',cb.food_rev,'baseAmount',cb.food_base,'discountAmount',cb.food_disc,'totalHpp',cb.food_hpp,'netRevenue',cb.food_rev - cb.food_hpp_real,'totalOrders',cb.food_count,'averageOrderValue',ROUND(cb.food_rev/GREATEST(cb.food_count,1),2),'topItems',fti.list),
        'generatedAt',NOW()
    ) INTO v_result FROM combined cb CROSS JOIN session_agg sa CROSS JOIN voucher_usage vu CROSS JOIN console_usage cu CROSS JOIN daily d CROSS JOIN discount_rules_summary dr CROSS JOIN food_top_items fti;
    RETURN QUERY SELECT v_result;
END;
$function$;



CREATE OR REPLACE FUNCTION "__SP__ExtendSession"(p_session_id uuid, p_additional_minutes integer, p_cash_received numeric, p_pay_now boolean DEFAULT true, p_voucher_code character varying DEFAULT NULL::character varying, p_notes text DEFAULT NULL::text)
 RETURNS TABLE(session_id uuid, session_booked_minutes integer, session_end_scheduled timestamp with time zone, payment_id uuid, payment_amount numeric, payment_discount numeric, payment_total numeric, payment_cash_received numeric, payment_change numeric, payment_voucher_id uuid, payment_status character varying, payment_paid_at timestamp with time zone)
 LANGUAGE plpgsql
AS $function$
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
$function$;
