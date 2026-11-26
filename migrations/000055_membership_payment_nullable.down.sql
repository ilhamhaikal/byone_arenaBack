-- Rollback 000055.
-- Akan gagal jika sudah ada payment membership dengan session_id NULL.
-- Hapus/relasikan payment tersebut terlebih dahulu sebelum rollback.

ALTER TABLE payments
    ALTER COLUMN session_id SET NOT NULL;