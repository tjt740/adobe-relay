-- Node observations describe real generation, independently of HEAD probes.
-- No production IDs, credentials or subscription URLs are embedded here.
-- NULL keeps the existing unresolved-408 release policy. A transient review
-- records only the scheduling state temporarily taken by its own lease.
ALTER TABLE adobe_proxy_test_leases ADD COLUMN return_schedulable BOOLEAN;

CREATE TABLE adobe_route_exclusions (
 proxy_id BIGINT PRIMARY KEY REFERENCES proxies(id) ON DELETE CASCADE,
 proxy_updated_at TIMESTAMPTZ NOT NULL,
 reason VARCHAR(80) NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE adobe_route_observations (
 id BIGSERIAL PRIMARY KEY,
 proxy_id BIGINT NOT NULL,
 proxy_updated_at TIMESTAMPTZ NOT NULL,
 source_id VARCHAR(64) NOT NULL DEFAULT '',
 account_id BIGINT NOT NULL,
 credential_hash VARCHAR(64) NOT NULL,
 scope VARCHAR(200) NOT NULL,
 success BOOLEAN NOT NULL,
 status_code INTEGER NOT NULL,
 stage VARCHAR(20) NOT NULL,
 submission_rejected BOOLEAN NOT NULL DEFAULT false,
 started_at TIMESTAMPTZ NOT NULL,
 checked_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX adobe_route_observations_window ON adobe_route_observations(proxy_id,proxy_updated_at,scope,checked_at DESC);
CREATE INDEX adobe_route_observations_retention ON adobe_route_observations(checked_at);
CREATE TABLE adobe_route_health (
 proxy_id BIGINT NOT NULL REFERENCES proxies(id) ON DELETE CASCADE,
 proxy_updated_at TIMESTAMPTZ NOT NULL,
 scope VARCHAR(200) NOT NULL,
 failed_at TIMESTAMPTZ,
 success_started_at TIMESTAMPTZ,
 retry_after TIMESTAMPTZ,
 PRIMARY KEY(proxy_id,scope)
);
