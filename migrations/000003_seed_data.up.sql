-- =============================================================================
-- 000003_seed_data.up.sql — Default/seed data untuk fresh setup
-- =============================================================================

-- Admin default (password: password)
INSERT INTO users (id, username, password, full_name, role, is_active)
VALUES (
    uuid_generate_v4(),
    'admin',
    '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi',
    'Administrator',
    'admin',
    TRUE
) ON CONFLICT (username) DO NOTHING;

-- App settings default
INSERT INTO app_settings (key, value, description) VALUES
    ('membership_price', '0', 'Harga membership (lifetime)')
ON CONFLICT (key) DO NOTHING;
