#!/usr/bin/env bash
set -euo pipefail
# Install as /usr/local/sbin/sub2api-cert-renew and run as root.
/opt/sub2api-certbot/bin/certbot renew \
  --cert-name sub2api-ip --non-interactive --no-random-sleep-on-renew --quiet "$@"
nginx -t
systemctl reload nginx
