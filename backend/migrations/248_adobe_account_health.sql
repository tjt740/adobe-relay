-- Passive evidence only: never changes credentials, scheduling or login policy.
-- Unresolved failures survive log retention and a quiet traffic window.
CREATE TABLE IF NOT EXISTS adobe_account_health (
    account_id BIGINT NOT NULL,
    scope TEXT NOT NULL DEFAULT '',
    failed_at TIMESTAMPTZ,
    alert_failed_at TIMESTAMPTZ,
    status_code INTEGER NOT NULL DEFAULT 0,
    reason TEXT NOT NULL DEFAULT '',
    stage TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    success_at TIMESTAMPTZ,
    success_started_at TIMESTAMPTZ,
    success_source TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(account_id, scope)
);
CREATE INDEX IF NOT EXISTS idx_adobe_account_health_unresolved
    ON adobe_account_health(account_id, failed_at)
    WHERE failed_at IS NOT NULL AND (success_started_at IS NULL OR success_started_at <= failed_at);
