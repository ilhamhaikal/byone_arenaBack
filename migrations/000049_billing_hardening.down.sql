-- Down migration sengaja TIDAK mengembalikan SP ke versi yang bug.
-- Yang dilepas hanya guard trigger voucher; SP hardening tetap aman dipakai.
DROP TRIGGER IF EXISTS trg_voucher_usage_limit ON vouchers;
DROP FUNCTION IF EXISTS "__SP__EnforceVoucherUsageLimit"();