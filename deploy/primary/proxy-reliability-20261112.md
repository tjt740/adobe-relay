# Proxy reliability and 403 diagnostics — 2026-11-12

Status: prepared, not yet deployed. Follow-up evidence will identify the exact application and verification results.

## Scope

Keep the live 1536 MiB Mihomo limit and preserve its process/connections. Proxies 1124, 1160 and 1953 were placed inactive with persistent route exclusions at 05:10:44 Beijing; all had zero account bindings and zero test leases. No listener was removed. Audit: `/opt/adobe-relay/releases/proxy-reliability-20261112/quarantine-applied.json`.

Health checks complete certificate verification, TLS and HTTP through the selected proxy. A failed primary Adobe API check triggers a second Adobe host check; healthy routes incur no second request. A 403 proves an HTTP denial, not a broken transport. Both targets must fail transport before the route is marked unhealthy. Existing queue/concurrency/deadlines remain.

A read-only host sampler publishes bounded, credential-free cgroup memory and restart/OOM snapshots on the existing app data mount. The app reads this file without Docker socket access. New site-only alert rules cover sustained 70% / 85% memory, new OOM/restart signals and missing/stale monitoring. Existing SMTP/alert settings remain unchanged. Missing data cannot resolve a memory alarm.

403 diagnostics identify the exact submitted bearer/session by one-way combined and separate fingerprints, declared JWT expiry, actual attempt/account, proxy configuration fingerprint, upstream request/code and response hash. No raw credentials, prompts, signed URLs or request-time IP lookup is added. Generic access_error remains undetermined; explicit entitlement/quota denials retain their distinct classification. Existing auth retry and accepted/unknown job non-replay protections remain.

The six prior controls (three idle accounts × two proxies) all returned images. They do not establish the historical 403 cause because original failing credentials/exit identity were not preserved across all controls. See `mihomo-memory-20261112.md` and the server `adobe-403-control-20261112` audit.

## Deployment and verification

Use the exact checked image and existing rolling drain tooling, natural drain only, preservation checks, rollback tag and current database backup. No migration, pricing, cookie-policy, scheduler budget, Dockerfile or startup change is planned. Retain current SS/SS2022 work from main. Do not classify prepared or failed gates as production success. No email will be sent by this task.

Lowering memory or removing these exclusions is not an automatic rollback: inspect current traffic and evidence first. Code rollback never restores old database/config snapshots. Capacity and health checks reduce known failure modes; they cannot guarantee every possible 502 is eliminated.
