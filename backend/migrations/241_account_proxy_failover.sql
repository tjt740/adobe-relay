CREATE TABLE account_proxy_failover (
    account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    primary_proxy_id BIGINT NOT NULL,
    backup_proxy_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    revision BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE proxy_failover_health (
    proxy_id BIGINT PRIMARY KEY REFERENCES proxies(id) ON DELETE CASCADE,
    fingerprint TEXT NOT NULL,
    status TEXT NOT NULL,
    failures INTEGER NOT NULL DEFAULT 0,
    latency_ms BIGINT NOT NULL DEFAULT 0,
    checked_at TIMESTAMPTZ NOT NULL,
    message TEXT NOT NULL DEFAULT ''
);
CREATE TABLE proxy_failover_events (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    from_proxy_id BIGINT NOT NULL,
    to_proxy_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX proxy_failover_events_account_idx ON proxy_failover_events(account_id, id DESC);
