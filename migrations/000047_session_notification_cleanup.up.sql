-- =============================================================================
-- 000047_session_notification_cleanup.up.sql
--
-- Membersihkan notifikasi sistem/transient milik satu konsol ketika sesi
-- dimulai, berakhir, atau dibatalkan. Tujuannya:
--   * pesan dari sesi lama tidak muncul lagi di sesi baru;
--   * riwayat notifikasi tetap tersimpan (hanya is_active=false);
--   * notifikasi buatan admin (promo/himbauan) TIDAK ikut dinonaktifkan
--     karena judulnya berbeda dari tiga notifikasi sistem di bawah.
-- =============================================================================

CREATE OR REPLACE FUNCTION "__SP__DeactivateSessionTvNotifications"(p_console_id TEXT)
RETURNS TABLE(updated INT)
LANGUAGE plpgsql AS $$
DECLARE
    n INT;
BEGIN
    -- Sengaja TIDAK memfilter auto_show: flag itu hanya mengatur perilaku
    -- tampil, bukan penanda jenis notifikasi. Notifikasi sistem lama bisa
    -- saja auto_show=false. Pemisah yang benar adalah judul sistem di bawah.
    UPDATE tv_notifications
       SET is_active = false,
           updated_at = NOW()
     WHERE is_active = true
       AND title IN ('Pesanan Disiapkan', 'Sesi Diperpanjang', 'Pembayaran Tertunda')
       AND (target_all = true OR target_console_ids @> to_jsonb(p_console_id));

    GET DIAGNOSTICS n = ROW_COUNT;
    RETURN QUERY SELECT n;
END;
$$;
