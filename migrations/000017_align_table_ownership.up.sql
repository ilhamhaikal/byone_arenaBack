-- Samakan kepemilikan seluruh tabel publik dengan pemilik tabel `users`.
--
-- Tujuan: migrasi yang dijalankan oleh role berbeda (mis. `postgres` saat
-- setup Windows/pemulihan manual) tidak meninggalkan tabel yang tidak bisa
-- diakses role aplikasi. Idempoten: bila semua sudah satu owner, tidak ada
-- perubahan.

DO $$
DECLARE
    v_owner text;
    r       record;
BEGIN
    SELECT tableowner INTO v_owner
      FROM pg_tables
     WHERE schemaname = 'public' AND tablename = 'users';

    IF v_owner IS NULL OR v_owner = current_user THEN
        RETURN;
    END IF;

    FOR r IN
        SELECT tablename FROM pg_tables
         WHERE schemaname = 'public' AND tableowner <> v_owner
    LOOP
        EXECUTE format('ALTER TABLE public.%I OWNER TO %I', r.tablename, v_owner);
    END LOOP;

    FOR r IN
        SELECT sequencename FROM pg_sequences
         WHERE schemaname = 'public'
    LOOP
        EXECUTE format('ALTER SEQUENCE public.%I OWNER TO %I', r.sequencename, v_owner);
    END LOOP;

    -- Pastikan role aplikasi punya akses penuh ke objek yang baru dibuat.
    EXECUTE format('GRANT ALL ON ALL TABLES IN SCHEMA public TO %I', v_owner);
    EXECUTE format('GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO %I', v_owner);
END
$$;