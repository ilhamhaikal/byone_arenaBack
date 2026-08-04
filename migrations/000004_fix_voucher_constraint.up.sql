-- 000004_fix_voucher_constraint.up.sql
-- Memperbaiki CHECK constraint discount_type pada tabel vouchers.
--
-- Masalah: versi awal migration 000001 hanya mengenal 'percentage' dan
-- 'fixed_amount'. Saat fitur 'free_days' dan 'unlimited_play' ditambahkan
-- ke entitas Go, constraint di database yang sudah ada TIDAK ikut ter-update
-- karena CREATE TABLE IF NOT EXISTS tidak memodifikasi constraint yang sudah
-- ada. Akibatnya INSERT dengan discount_type='free_days' gagal:
--   ERROR: new row for relation "vouchers" violates check constraint
--          "vouchers_discount_type_check" (SQLSTATE 23514)
--
-- Perbaikan: DROP constraint lama, lalu ADD constraint baru yang mencakup
-- keempat jenis diskon: percentage, fixed_amount, free_days, unlimited_play.
-- Operasi ini idempoten — aman dijalankan ulang maupun pada database yang
-- sudah mempunyai constraint versi baru (fresh install).

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE  constraint_name = 'vouchers_discount_type_check'
        AND    table_name      = 'vouchers'
    ) THEN
        ALTER TABLE vouchers DROP CONSTRAINT vouchers_discount_type_check;
    END IF;
END $$;

ALTER TABLE vouchers
    ADD CONSTRAINT vouchers_discount_type_check
    CHECK (discount_type IN ('percentage', 'fixed_amount', 'free_days', 'unlimited_play'));
