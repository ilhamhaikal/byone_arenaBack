-- =============================================================================
-- 000055_membership_payment_nullable.up.sql
--
-- Payment membership tidak memiliki session_id (bukan pembayaran sesi rental).
-- Kolom payments.session_id harus nullable agar SellMembership dapat mencatat
-- pembayaran membership, termasuk membership gratis (harga 0).
--
-- Tidak ada data yang dihapus/diubah; hanya constraint NOT NULL yang dilepas.
-- =============================================================================

ALTER TABLE payments
    ALTER COLUMN session_id DROP NOT NULL;