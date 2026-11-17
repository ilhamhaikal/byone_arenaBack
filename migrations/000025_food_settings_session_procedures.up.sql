-- =============================================================================
-- 000025_food_settings_session_procedures.up.sql
--
-- Memindahkan query mentah dari handler food order, settings, dan session ke
-- STORED PROCEDURE agar perbaikan logika cukup di procedure tanpa rebuild app.
-- =============================================================================

-- ── FOOD ORDER ──────────────────────────────────────────────────────────────

-- Set metode pembayaran sebuah pesanan makanan.
CREATE OR REPLACE FUNCTION "__SP__SetFoodOrderPaymentMethod"(p_order_id UUID, p_method VARCHAR)
RETURNS VOID
LANGUAGE sql AS $$
    UPDATE food_orders SET payment_method = p_method WHERE id = p_order_id;
$$;

-- Harga satu menu.
CREATE OR REPLACE FUNCTION "__SP__MenuItemPrice"(p_menu_id UUID)
RETURNS TABLE(price NUMERIC)
LANGUAGE sql AS $$
    SELECT price FROM menu_items WHERE id = p_menu_id;
$$;

-- Status member seorang customer (false bila tidak ditemukan).
CREATE OR REPLACE FUNCTION "__SP__CustomerIsMember"(p_customer_id UUID)
RETURNS TABLE(is_member BOOLEAN)
LANGUAGE sql AS $$
    SELECT COALESCE(is_member, false) FROM customers WHERE id = p_customer_id;
$$;

-- ── SETTINGS (app_settings) ─────────────────────────────────────────────────

-- Ambil nilai satu setting.
CREATE OR REPLACE FUNCTION "__SP__AppSettingValue"(p_key TEXT)
RETURNS TABLE(value TEXT)
LANGUAGE sql AS $$
    SELECT value::text FROM app_settings WHERE key = p_key;
$$;

-- Set nilai satu setting.
CREATE OR REPLACE FUNCTION "__SP__SetAppSetting"(p_key TEXT, p_value TEXT)
RETURNS VOID
LANGUAGE sql AS $$
    UPDATE app_settings SET value = p_value, updated_at = NOW() WHERE key = p_key;
$$;

-- ── SESSION (atribusi & pembacaan ringan) ───────────────────────────────────

CREATE OR REPLACE FUNCTION "__SP__SetSessionCreatedBy"(p_session_id UUID, p_user_id UUID)
RETURNS VOID
LANGUAGE sql AS $$
    UPDATE sessions SET created_by = p_user_id
     WHERE id = p_session_id AND created_by IS NULL;
$$;

CREATE OR REPLACE FUNCTION "__SP__SetPaymentCreatedBy"(p_session_id UUID, p_user_id UUID)
RETURNS VOID
LANGUAGE sql AS $$
    UPDATE payments SET created_by = p_user_id
     WHERE session_id = p_session_id AND created_by IS NULL;
$$;

CREATE OR REPLACE FUNCTION "__SP__SetSessionEndedBy"(p_session_id UUID, p_user_id UUID)
RETURNS VOID
LANGUAGE sql AS $$
    UPDATE sessions SET ended_by = p_user_id WHERE id = p_session_id;
$$;

CREATE OR REPLACE FUNCTION "__SP__SetSessionCancelledBy"(p_session_id UUID, p_user_id UUID)
RETURNS VOID
LANGUAGE sql AS $$
    UPDATE sessions SET cancelled_by = p_user_id WHERE id = p_session_id;
$$;

-- Jumlah pembayaran yang masih pending untuk satu sesi.
CREATE OR REPLACE FUNCTION "__SP__CountPendingPayments"(p_session_id UUID)
RETURNS TABLE("count" BIGINT)
LANGUAGE sql AS $$
    SELECT COUNT(*)::BIGINT AS "count"
      FROM payments
     WHERE session_id = p_session_id AND payment_status = 'pending';
$$;

-- console_id dari sebuah sesi.
CREATE OR REPLACE FUNCTION "__SP__SessionConsoleID"(p_session_id UUID)
RETURNS TABLE(console_id UUID)
LANGUAGE sql AS $$
    SELECT console_id FROM sessions WHERE id = p_session_id;
$$;

-- ── TV NOTIFICATIONS (target per konsol) ────────────────────────────────────

-- Nonaktifkan notifikasi lama berdasarkan judul + konsol target.
CREATE OR REPLACE FUNCTION "__SP__DeactivateTvNotification"(p_title TEXT, p_console_id TEXT)
RETURNS VOID
LANGUAGE sql AS $$
    UPDATE tv_notifications
       SET is_active = false, updated_at = NOW()
     WHERE title = p_title
       AND is_active = true
       AND target_console_ids @> to_jsonb(p_console_id);
$$;

-- Sisipkan notifikasi baru yang menargetkan satu konsol.
CREATE OR REPLACE FUNCTION "__SP__InsertTvNotification"(
    p_title TEXT,
    p_message TEXT,
    p_console_id TEXT,
    p_active_sessions_only BOOLEAN,
    p_priority VARCHAR
)
RETURNS VOID
LANGUAGE sql AS $$
    INSERT INTO tv_notifications
        (id, title, message, target_all, target_console_ids,
         active_sessions_only, loop_enabled, is_active, priority,
         created_at, updated_at)
    VALUES
        (uuid_generate_v4(), p_title, p_message, false,
         jsonb_build_array(p_console_id),
         p_active_sessions_only, false, true, p_priority,
         NOW(), NOW());
$$;
