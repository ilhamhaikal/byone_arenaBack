-- =============================================================================
-- 000028_cleanup_notification_procedures.up.sql
--
-- Memindahkan query cleanup notifikasi di cmd/ ke STORED PROCEDURE.
-- =============================================================================

-- Nonaktifkan semua notifikasi aktif dengan judul tertentu.
CREATE OR REPLACE FUNCTION "__SP__DeactivateTvNotificationByTitle"(p_title TEXT)
RETURNS TABLE(updated INT)
LANGUAGE plpgsql AS $$
DECLARE n INT;
BEGIN
    UPDATE tv_notifications
       SET is_active = false, updated_at = NOW()
     WHERE title = p_title AND is_active = true;
    GET DIAGNOSTICS n = ROW_COUNT;
    RETURN QUERY SELECT n;
END;
$$;

-- Nonaktifkan notifikasi "Pembayaran Tertunda" milik konsol yang sesinya sudah tidak aktif.
CREATE OR REPLACE FUNCTION "__SP__DeactivateStalePendingPaymentNotifications"()
RETURNS TABLE(updated INT)
LANGUAGE plpgsql AS $$
DECLARE n INT;
BEGIN
    UPDATE tv_notifications tn
       SET is_active = false, updated_at = NOW()
      FROM sessions s
     WHERE tn.title = 'Pembayaran Tertunda'
       AND tn.is_active = true
       AND tn.target_console_ids @> to_jsonb(s.console_id::text)
       AND s.status <> 'active';
    GET DIAGNOSTICS n = ROW_COUNT;
    RETURN QUERY SELECT n;
END;
$$;
