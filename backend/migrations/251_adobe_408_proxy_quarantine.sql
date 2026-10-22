-- Separate durable model failures, allocator-owned pauses and short-lived tests.
-- Existing credentials, configuration and operator state are not rewritten.
CREATE TABLE IF NOT EXISTS adobe_proxy_408_blocks (
    account_id BIGINT NOT NULL,
    scope TEXT NOT NULL DEFAULT '',
    failed_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY(account_id, scope)
);
CREATE TABLE IF NOT EXISTS adobe_proxy_408_holds (
    account_id BIGINT PRIMARY KEY,
    resume_schedulable BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE TABLE IF NOT EXISTS adobe_proxy_test_leases (
    account_id BIGINT PRIMARY KEY,
    owner TEXT NOT NULL,
    proxy_id BIGINT NOT NULL,
    proxy_updated_at TIMESTAMPTZ NOT NULL,
    credential_hash TEXT NOT NULL,
    scope TEXT NOT NULL DEFAULT '',
    acquired_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_adobe_proxy_test_leases_expiry ON adobe_proxy_test_leases(expires_at);

INSERT INTO adobe_proxy_408_blocks(account_id,scope,failed_at)
SELECT h.account_id,h.scope,h.failed_at FROM adobe_account_health h
JOIN accounts a ON a.id=h.account_id
WHERE a.platform='adobe' AND a.deleted_at IS NULL AND h.status_code=408
AND h.failed_at IS NOT NULL AND (h.success_started_at IS NULL OR h.success_started_at<=h.failed_at)
ON CONFLICT(account_id,scope) DO NOTHING;
