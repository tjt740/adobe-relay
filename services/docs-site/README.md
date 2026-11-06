# Registered-user API documentation

Independent documentation service for `https://creative.cheap/docs`, hosted on the existing primary server. It uses the existing Sub2 email/password and 2FA endpoints. Every HTML/Markdown document request is checked against `/api/v1/auth/me`; anonymous users, pending 2FA sessions, revoked/expired tokens, inactive/deleted users and authentication outages cannot read the document. Current login CAPTCHA settings are disabled; enabling a CAPTCHA on Sub2 will fail closed until the corresponding widget is integrated here.

The browser receives a random, host-only Secure/HttpOnly/SameSite=Strict session cookie. Sub2 tokens remain in bounded process memory; passwords are forwarded once and are neither stored nor logged. Sessions last at most one hour (or the upstream access-token lifetime), are bound to IP/user agent, and are invalidated by logout or service restart. The existing upstream login/rate limits continue to apply, with an additional nginx limit. No registration endpoint, admin API proxy, database connection, or image-generation action is exposed by this service.

`guide.md` is a build-time copy of `docs/ADOBE_IMAGE_CLIENT_GUIDE.md`; `build.sh` refreshes it, runs the race tests and vet, then builds the Linux binary. Do not publish a public static copy: both HTML and Markdown are protected by the server. Public assets contain only presentation code. No analytics or third-party scripts.

## Build and install

Run `./build.sh /absolute/output/docs-server`. Upload this binary and the deployment files into an isolated server staging directory, verify SHA256, then run `python3 install.py <binary-sha256>`. The installer holds the existing release lock, installs only the docs systemd service, then adds one include to the current creative.cheap HTTPS server. It snapshots and checks all existing container identities/restarts and configuration fingerprints; the existing nginx file must be byte-for-byte identical after removing that one include. Repeat installs are rejected rather than overwriting an unknown later version.

The service binds to loopback, runs with a dynamic unprivileged user, 96 MiB maximum memory, and one CPU maximum. Sub2 and Okad are not restarted or changed. The existing domain/certificate are reused. `/docs`, its assets, login forms and downloads stay under `/docs/`. The previous `/docs/batch-image` alias redirects to the existing `/batch-image` page. www requests are redirected to the canonical creative.cheap hostname before the login form. No DNS change or new certificate is required.

`verify-live.py` performs one login with the existing server-admin credential in memory, checks the protected document/download, then logs out in a finally block. It writes only a sanitized receipt. It makes no Adobe requests and does not change user/account data beyond normal Sub2 login/activity/session records. Do not automatically repeat an uncertain login attempt. Local tests also cover ordinary user identity, 2FA, invalid credentials, revocation, inactivity, malformed responses, authentication outages, CSRF, expiry and concurrent reads.

## Update / rollback

Rebuild from the exact committed source. Under the existing release lock, preserve the previous binary/symlink and nginx/service configuration, verify source and artifact hashes and current service state, atomically point `current` at the new release, restart only `creative-docs`, and verify access gates and health. Restart intentionally signs docs visitors out. Reverting only this binary uses the same procedure; do not roll back any Sub2 data, application or other service.

To remove an initial installation, first verify no later update exists. Under the release lock remove only the recorded docs include from the current creative.cheap HTTPS server (do not restore a stale whole file), remove `00-creative-docs-limits.conf` and the two docs snippets, run `nginx -t`, and reload nginx. Then disable/stop `creative-docs`. Keep audit evidence and current Sub2/Okad data/configuration.

## Browser form regression

Use `Referrer-Policy: same-origin`. `no-referrer` makes native form POSTs send `Origin: null` under the Fetch standard and conflicts with the exact-origin CSRF gate. Keep the gate: missing/null/foreign origins remain rejected. Old open forms receive a fresh login page prompting reload.

`browser-check.mjs` starts an isolated TLS fixture with simulated Sub2 users. Run with an installed Playwright module (`PLAYWRIGHT_MODULE_PATH` optional); `DOCS_CHROME_CHANNEL=chrome DOCS_BROWSERS=chromium` uses an existing Chrome installation. It reproduces the old policy's 403, then tests native administrator/user/2FA login, Markdown download, logout and desktop/mobile widths. The fixture is test-only and is excluded from the deployed binary. Screenshot/output directory defaults to ignored `.browser-output`.

For a binary-only update, the primary workflow's `update` phase requires the exact currently installed binary SHA256. It holds the existing release lock, records prior version/configuration, atomically updates only the docs binary and restarts only docs. All document sessions expire at restart; Sub2/Okad and nginx are preserved. Health failure restores only the previous docs binary.

## Document presentation

Green navigation and table headings, soft teal inline parameters, muted green supported states and restrained amber limitations form a coordinated palette. Code examples use mint, sage, blue-grey and warm sand on a dark green-grey surface. Active navigation follows the reading position. Bash, JSON and Python examples use local text-node highlighting with no external scripts or HTML injection; copy buttons use the untouched source string. Unknown languages remain plain text. Wide tables and examples scroll within their own containers.

The browser check compares all rendered examples and table cells to the server HTML, checks exact clipboard payloads for all 18 examples, verifies section search/current navigation, and captures login, document, capability-table and code views at 1440/390 widths. These are Chrome viewport checks, not physical-device or other-browser proof.
