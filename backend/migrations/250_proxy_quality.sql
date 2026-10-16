-- Rolling network evidence only; no account, credential or scheduling rewrites.
ALTER TABLE proxy_failover_health ADD COLUMN IF NOT EXISTS latency_ewma_ms BIGINT NOT NULL DEFAULT 0;
ALTER TABLE proxy_failover_health ADD COLUMN IF NOT EXISTS slow_checks INTEGER NOT NULL DEFAULT 0;
ALTER TABLE proxy_failover_health ADD COLUMN IF NOT EXISTS success_permille INTEGER NOT NULL DEFAULT 0;
