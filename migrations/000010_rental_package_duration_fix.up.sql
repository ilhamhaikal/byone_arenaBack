-- ============================================
-- 000010: Perbaiki perhitungan durasi paket lintas malam
-- ============================================
-- Bug: (p_end + INTERVAL '24 hours') pada tipe TIME wrap-around ke jam yang
-- sama, sehingga 21:00 -> 06:00 menghasilkan -900 menit (-15 jam) bukan 540.
-- Fix: hitung menit secara eksplisit.
CREATE OR REPLACE FUNCTION "__SP__PackageDurationMinutes"(p_start TIME, p_end TIME)
RETURNS INT LANGUAGE plpgsql AS $$
DECLARE
    v_start_min INT;
    v_end_min INT;
BEGIN
    v_start_min := EXTRACT(HOUR FROM p_start)::INT * 60 + EXTRACT(MINUTE FROM p_start)::INT;
    v_end_min   := EXTRACT(HOUR FROM p_end)::INT   * 60 + EXTRACT(MINUTE FROM p_end)::INT;

    IF v_end_min > v_start_min THEN
        RETURN v_end_min - v_start_min;
    ELSE
        -- Lintas tengah malam: tambahkan 24 jam (1440 menit)
        RETURN (v_end_min + 1440) - v_start_min;
    END IF;
END;
$$;
