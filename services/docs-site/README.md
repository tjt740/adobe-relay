# Registered-user API documentation

Independent documentation service for `https://docs.creative.cheap`, hosted on the existing primary server. It uses the existing Sub2 email/password and 2FA endpoints. Every HTML/Markdown document request is checked against `/api/v1/auth/me`; anonymous users, pending 2FA sessions, revoked/expired tokens, inactive/deleted users and authentication outages cannot read the document. Current login CAPTCHA settings are disabled; enabling a CAPTCHA on Sub2 will fail closed until the corresponding widget is integrated here.

The browser receives a random, host-only Secure/HttpOnly/SameSite=Strict session cookie. Sub2 tokens remain in bounded process memory; passwords are forwarded once and are neither stored nor logged. Sessions last at most one hour (or the upstream access-token lifetime), are bound to IP/user agent, and are invalidated by logout or service restart. The existing upstream login/rate limits continue to apply, with an additional nginx limit. No registration endpoint, admin API proxy, database connection, or image-generation action is exposed by this service.

`guide.md` is a build-time copy of `docs/ADOBE_IMAGE_CLIENT_GUIDE.md`; `build.sh` refreshes it, runs the race tests and vet, then builds the Linux binary. Do not publish a public static copy: both HTML and Markdown are protected by the server. Public assets contain only presentation code. No analytics or third-party scripts.

## Build and install

Run `./build.sh /absolute/output/docs-server`. Upload this binary and the deployment files into an isolated server staging directory, verify SHA256, then run `python3 install.py install <binary-sha256>`. The installer holds the existing release lock, installs only the docs systemd service and separate nginx hostname, and checks existing container identities/restarts and configuration fingerprints. It rejects repeat installation rather than overwriting an unknown later version. The Go service binds to loopback, runs with a dynamic unprivileged user, 96 MiB maximum memory, and one CPU maximum. Sub2 and Okad are not restarted or changed.

DNS must resolve `docs.creative.cheap` to `173.234.15.150`. Issue its separate certificate with the server's existing certbot installation and webroot `/var/www/creative-docs-acme`. Run `python3 install.py https` only once that certificate exists. Existing certbot renewal covers this additional certificate; verify the configured timer and renewal entry. Never expose the login form over HTTP.

`verify-live.py` performs one login with the existing server-admin credential in memory, checks the protected document/download, then logs out in a finally block. It writes only a sanitized receipt. It makes no Adobe requests and does not change user/account data beyond normal Sub2 login/activity/session records. Do not automatically repeat an uncertain login attempt. Local tests also cover ordinary user identity, 2FA, invalid credentials, revocation, inactivity, malformed responses, authentication outages, CSRF, expiry and concurrent reads.

## Update / rollback

Rebuild from the exact committed source. Under the existing release lock, preserve the previous binary/symlink and nginx/service configuration, verify source and artifact hashes and current service state, atomically point `current` at the new release, restart only `creative-docs`, and verify access gates and health. Restart intentionally signs docs visitors out. Reverting only this binary uses the same procedure; do not roll back any Sub2 data, application or other service.

To remove an initial installation, first verify no later update exists, then disable/stop `creative-docs`, remove only its three `creative-docs` nginx configs (including the log-format file) and proxy snippet, run `nginx -t`, and reload nginx. Keep audit evidence and certificate renewal records until explicitly reviewed.
