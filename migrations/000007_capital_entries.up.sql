-- ============================================
-- 000007: Pembukuan modal awal (pengeluaran beli bahan)
-- ============================================
CREATE TABLE IF NOT EXISTS capital_entries (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    amount      NUMERIC(12,2) NOT NULL CHECK (amount >= 0),
    entry_date  DATE NOT NULL,
    notes       TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_capital_entries_date ON capital_entries(entry_date);

-- 1. Tambah catatan modal
CREATE OR REPLACE FUNCTION "__SP__AddCapitalEntry"(p_amount NUMERIC, p_entry_date DATE, p_notes TEXT)
RETURNS TABLE(id UUID, amount NUMERIC, entry_date DATE, notes TEXT, created_at TIMESTAMPTZ)
LANGUAGE plpgsql AS $$
DECLARE v_id UUID := uuid_generate_v4();
BEGIN
    INSERT INTO capital_entries(id, amount, entry_date, notes, created_at, updated_at)
    VALUES (v_id, p_amount, p_entry_date, p_notes, NOW(), NOW());
    RETURN QUERY SELECT ce.id, ce.amount, ce.entry_date, ce.notes, ce.created_at
                 FROM capital_entries ce WHERE ce.id = v_id;
END;
$$;

-- 2. Hapus catatan modal
CREATE OR REPLACE FUNCTION "__SP__DeleteCapitalEntry"(p_id UUID)
RETURNS VOID LANGUAGE plpgsql AS $$
BEGIN
    DELETE FROM capital_entries WHERE id = p_id;
    IF NOT FOUND THEN RAISE EXCEPTION 'CAPITAL_ENTRY_NOT_FOUND'; END IF;
END;
$$;

-- 3. Daftar catatan modal per rentang tanggal
CREATE OR REPLACE FUNCTION "__SP__ListCapitalEntries"(p_start_date DATE, p_end_date DATE)
RETURNS TABLE(id UUID, amount NUMERIC, entry_date DATE, notes TEXT, created_at TIMESTAMPTZ)
LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT ce.id, ce.amount, ce.entry_date, ce.notes, ce.created_at
    FROM capital_entries ce
    WHERE ce.entry_date BETWEEN p_start_date AND p_end_date
    ORDER BY ce.entry_date DESC, ce.created_at DESC;
END;
$$;

-- 4. Perbandingan bulanan: modal awal vs total pendapatan vs pendapatan makanan
CREATE OR REPLACE FUNCTION "__SP__MonthlyCapitalReport"(p_months INT DEFAULT 6)
RETURNS TABLE(month TEXT, initial_capital NUMERIC, total_revenue NUMERIC, food_revenue NUMERIC, net_profit NUMERIC)
LANGUAGE plpgsql AS $$
BEGIN
    IF p_months < 1 THEN p_months := 6; END IF;
    RETURN QUERY
    SELECT to_char(mm.m, 'YYYY-MM') AS month,
           COALESCE(cap.total, 0)::NUMERIC(12,2) AS initial_capital,
           COALESCE(rev.total, 0)::NUMERIC(12,2) AS total_revenue,
           COALESCE(food.total, 0)::NUMERIC(12,2) AS food_revenue,
           (COALESCE(rev.total,0) - COALESCE(cap.total,0))::NUMERIC(12,2) AS net_profit
    FROM generate_series(
             date_trunc('month', CURRENT_DATE) - (p_months - 1) * INTERVAL '1 month',
             date_trunc('month', CURRENT_DATE),
             '1 month') AS mm(m)
    LEFT JOIN LATERAL (
        SELECT COALESCE(SUM(ce.amount),0) AS total
        FROM capital_entries ce
        WHERE date_trunc('month', ce.entry_date) = mm.m
    ) cap ON TRUE
    LEFT JOIN LATERAL (
        SELECT COALESCE(SUM(x.v),0) AS total
        FROM (
            SELECT p.total_payment AS v FROM payments p
            WHERE p.payment_status='paid' AND date_trunc('month', p.paid_at) = mm.m
            UNION ALL
            SELECT d.final_amount FROM daily_rentals d
            WHERE date_trunc('month', d.created_at) = mm.m
        ) x
    ) rev ON TRUE
    LEFT JOIN LATERAL (
        SELECT COALESCE(SUM(fo.total_amount - COALESCE(fo.discount_amount,0)),0) AS total
        FROM food_orders fo
        WHERE fo.status='served' AND date_trunc('month', fo.updated_at) = mm.m
    ) food ON TRUE
    ORDER BY mm.m ASC;
END;
$$;
