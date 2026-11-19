-- =============================================================================
-- 000029_seed_free_voucher.up.sql
--
-- Voucher DEFAULT yang sudah ada sejak setup pertama:
--   Kode   : GRATIS
--   Efek   : 100% dari harga sesi -> sesi apa pun (30 menit, 2 jam, dst) GRATIS
--   Batas  : tanpa batas maks diskon & tanpa batas pemakaian
--
-- Idempotent: aman dijalankan ulang (ON CONFLICT code -> skip).
-- =============================================================================

INSERT INTO vouchers
    (code, name, discount_type, discount_value, min_purchase,
     max_discount, max_usage, usage_count, is_active)
VALUES
    ('GRATIS', 'Gratis (Default)', 'percentage', 100, 0,
     0, 0, 0, TRUE)
ON CONFLICT (code) DO NOTHING;
