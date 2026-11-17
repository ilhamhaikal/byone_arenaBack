-- =============================================================================
-- 000026_console_payment_procedures.up.sql
--
-- Memindahkan query mentah sisa (console price, payment pending/refund,
-- finalisasi metode bayar) ke STORED PROCEDURE.
-- =============================================================================

-- ── CONSOLE ─────────────────────────────────────────────────────────────────

CREATE OR REPLACE FUNCTION "__SP__ConsolePricePerHour"(p_console_id UUID)
RETURNS TABLE(price_per_hour NUMERIC)
LANGUAGE sql AS $$
    SELECT price_per_hour FROM consoles WHERE id = p_console_id;
$$;

-- ── PAYMENT ─────────────────────────────────────────────────────────────────

-- Daftar payment pending milik satu sesi.
CREATE OR REPLACE FUNCTION "__SP__PendingPaymentIDs"(p_session_id UUID)
RETURNS TABLE(id UUID)
LANGUAGE sql AS $$
    SELECT id FROM payments
     WHERE session_id = p_session_id AND payment_status = 'pending';
$$;

-- Catat siapa yang melakukan refund.
CREATE OR REPLACE FUNCTION "__SP__SetPaymentRefundedBy"(p_payment_id UUID, p_user_id UUID)
RETURNS VOID
LANGUAGE sql AS $$
    UPDATE payments
       SET refunded_by = p_user_id, refunded_at = NOW()
     WHERE id = p_payment_id;
$$;

-- Koreksi baris payment untuk pembayaran non-tunai (QRIS/transfer):
-- metode diset sesuai pilihan, uang diterima = total, kembalian 0.
-- Untuk metode cash tidak melakukan apa-apa (SP lama sudah menghitungnya).
CREATE OR REPLACE FUNCTION "__SP__FinalizePaymentMethod"(p_payment_id UUID, p_method VARCHAR)
RETURNS VOID
LANGUAGE sql AS $$
    UPDATE payments
       SET payment_method = p_method,
           cash_received = total_payment,
           change_amount = 0
     WHERE id = p_payment_id
       AND p_method <> 'cash';
$$;
