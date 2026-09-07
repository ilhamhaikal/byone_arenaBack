-- ============================================
-- 000006: Soft delete untuk menu_items
-- ============================================
-- Masalah: DELETE menu gagal karena food_order_items punya FK
--          menu_item_id REFERENCES menu_items(id) ON DELETE RESTRICT.
-- Solusi: soft delete. Menu yang pernah dipesan tidak dihapus fisik
--         (riwayat/laporan tetap utuh), hanya disembunyikan dari daftar.

ALTER TABLE menu_items ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_menu_items_deleted_at ON menu_items(deleted_at);
