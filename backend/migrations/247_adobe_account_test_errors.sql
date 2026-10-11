-- Passive alert telemetry for completed account tests, separate from production
-- request metrics. Keep history even when a later test succeeds. No credentials
-- or raw upstream response bodies are stored here.
CREATE TABLE IF NOT EXISTS adobe_account_test_errors (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    account_id BIGINT NOT NULL,
    group_ids BIGINT[] NOT NULL DEFAULT '{}',
    status_code INTEGER NOT NULL CHECK (status_code IN (401, 408)),
    reason TEXT NOT NULL DEFAULT '',
    is_timeout BOOLEAN NOT NULL DEFAULT FALSE,
    CHECK (NOT is_timeout OR status_code = 408)
);
CREATE INDEX IF NOT EXISTS idx_adobe_account_test_errors_created_at
    ON adobe_account_test_errors (created_at);
