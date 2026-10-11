#!/usr/bin/env bash
# Update only Okad application code; keep live configuration and data.
set -euo pipefail
umask 077
image="${1:?Usage: okad-release.sh adobe-team-cookie:TAG}"
[[ "$image" =~ ^adobe-team-cookie:[a-zA-Z0-9._-]+$ ]] || exit 2
root=/opt/adobe-team-cookie
exec 9>"$root/.release.lock"
flock -w 300 9
compose=(docker compose --project-directory "$root" -f "$root/compose.yml")
docker image inspect "$image" >/dev/null
app_id=$("${compose[@]}" ps -q app)
[[ -n "$app_id" ]] || { echo 'An existing Okad container is required.' >&2; exit 1; }
stamp=$(date -u +%Y%m%dT%H%M%SZ)
backup_dir="$root/backups/$stamp-${image##*:}"
mkdir -p "$backup_dir"
cp -p "$root/.env" "$backup_dir/environment"
cp -p "$root/compose.yml" "$backup_dir/compose.yml"
# The container may contain previously applied frontend patches. Capture them,
# not just its original image tag. The live database is a separate bind mount.
previous="adobe-team-cookie:rollback-$stamp"
docker commit --pause=false "$app_id" "$previous" >/dev/null
printf '%s\n' "$previous" > "$backup_dir/previous-image"
docker exec -i "$app_id" python - <<'PY'
import sqlite3
source = sqlite3.connect('file:/app/data/app.db?mode=ro', uri=True)
dest = sqlite3.connect('/tmp/okad-release-backup.db')
source.backup(dest)
assert dest.execute('PRAGMA integrity_check').fetchone()[0] == 'ok'
dest.close()
source.close()
PY
docker cp "$app_id:/tmp/okad-release-backup.db" "$backup_dir/app.db" >/dev/null
docker exec "$app_id" rm /tmp/okad-release-backup.db
tar -C "$root" --exclude=data/app.db --exclude='data/app.db-*' -czf "$backup_dir/data-config.tar.gz" data
echo "Okad backup verified: $backup_dir"
set_image() {
  python3 - "$root/compose.yml" "$1" <<'PY'
from pathlib import Path
import re, sys
p = Path(sys.argv[1])
updated, count = re.subn(r'(?m)^(    image:)\s*adobe-team-cookie:[^\s]+[ \t]*$',
                         lambda m: m[1] + ' ' + sys.argv[2], p.read_text())
assert count == 1, 'Expected one Okad image in existing compose configuration'
p.write_text(updated)
PY
}
set_image "$image"
if "${compose[@]}" up -d --no-build --no-deps --wait --wait-timeout 180 app &&
   curl --fail --silent --show-error --max-time 15 http://127.0.0.1:9500/api/health; then
  echo "Released $image on the primary server."
else
  echo 'Okad release failed; restoring previous runtime code.' >&2
  set_image "$previous"
  "${compose[@]}" up -d --no-build --no-deps --wait --wait-timeout 120 app || true
  exit 1
fi
