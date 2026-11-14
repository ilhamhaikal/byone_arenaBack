-- Bonus waktu boleh dipakai berulang dalam satu sesi.
--
-- max_per_session = 0 berarti bebas (tanpa batas). Bila diisi > 0, admin
-- membatasi berapa kali bonus boleh diterapkan pada satu sesi.
ALTER TABLE promo_bonus_rules
    ADD COLUMN IF NOT EXISTS max_per_session INT NOT NULL DEFAULT 0;

ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS promo_apply_count INT NOT NULL DEFAULT 0;

COMMENT ON COLUMN promo_bonus_rules.max_per_session IS
    '0 = bebas dipakai berulang; >0 = maksimal berapa kali per sesi';
COMMENT ON COLUMN sessions.promo_apply_count IS
    'Berapa kali bonus promo diterapkan pada sesi ini';