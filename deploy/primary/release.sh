#!/usr/bin/env bash
# Run on the primary server after loading the image.
set -euo pipefail
umask 077
image="${1:?Usage: release.sh adobe-relay:COMMIT}"
[[ "$image" =~ ^adobe-relay:[a-zA-Z0-9._-]+$ ]] || exit 2
root=/opt/adobe-relay
env_file="$root/shared/.env"
exec 9>"$root/.release.lock"
flock -w 300 9
docker image inspect "$image" >/dev/null
app_id=$(adobe-relay ps -q app)
postgres_id=$(adobe-relay ps -q postgres)
[[ -n "$app_id" && -n "$postgres_id" ]] || { echo 'An existing app and database are required.' >&2; exit 1; }
old_image=$(docker inspect "$app_id" --format '{{.Config.Image}}')
backup_dir="$root/backups/$(date -u +%Y%m%dT%H%M%SZ)-${image##*:}"
mkdir -p "$backup_dir"
cp -p "$env_file" "$backup_dir/environment"
cp "$root/shared/"{compose.yml,clash.yml,resources.yml} "$backup_dir/"
docker exec "$postgres_id" pg_dump -U sub2api -d sub2api -Fc > "$backup_dir/database.dump"
docker exec -i "$postgres_id" pg_restore --list < "$backup_dir/database.dump" >/dev/null
docker exec "$app_id" tar -C /app -czf - data > "$backup_dir/app-data.tar.gz"
printf '%s\n' "$old_image" > "$backup_dir/previous-image"
echo "Backup verified: $backup_dir"
set_image() {
  python3 - "$env_file" "$1" <<'PY'
from pathlib import Path
import sys
p = Path(sys.argv[1])
lines = p.read_text().splitlines()
assert any(line.startswith('SUB2API_IMAGE=') for line in lines), 'SUB2API_IMAGE is missing'
p.write_text('\n'.join('SUB2API_IMAGE=' + sys.argv[2] if line.startswith('SUB2API_IMAGE=') else line for line in lines) + '\n')
p.chmod(0o600)
PY
}
set_image "$image"
if adobe-relay up -d --no-build --no-deps --wait --wait-timeout 240 app &&
   curl --fail --silent --show-error --max-time 15 http://127.0.0.1:6666/health; then
  echo "Released $image on the primary server."
else
  echo 'Release failed; restoring the previous application image.' >&2
  set_image "$old_image"
  adobe-relay up -d --no-build --no-deps --wait --wait-timeout 120 app || true
  exit 1
fi
