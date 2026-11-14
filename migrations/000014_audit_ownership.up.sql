-- Atribusi pelaku transaksi + audit trail.
--
-- Sebelum migrasi ini tidak ada satu pun kolom yang mencatat SIAPA yang
-- memulai/mengakhiri/membatalkan sesi atau memproses refund, sehingga selisih
-- kas tidak bisa ditelusuri. Semua kolom bersifat nullable agar data lama dan
-- rollback aplikasi tetap aman.

ALTER TABLE sessions ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS ended_by   UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS cancelled_by UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE payments ADD COLUMN IF NOT EXISTS created_by   UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS refunded_by  UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS refunded_at  TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_sessions_created_by ON sessions (created_by);
CREATE INDEX IF NOT EXISTS idx_payments_created_by ON payments (created_by);

-- Audit trail umum: satu baris per aksi yang menyentuh uang atau konfigurasi.
CREATE TABLE IF NOT EXISTS audit_logs (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID REFERENCES users(id) ON DELETE SET NULL,
    username   VARCHAR(50)  NOT NULL DEFAULT '',
    role       VARCHAR(20)  NOT NULL DEFAULT '',
    action     VARCHAR(50)  NOT NULL,
    entity     VARCHAR(50)  NOT NULL DEFAULT '',
    entity_id  UUID,
    detail     JSONB        NOT NULL DEFAULT '{}'::jsonb,
    ip         VARCHAR(50)  NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id    ON audit_logs (user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action     ON audit_logs (action);