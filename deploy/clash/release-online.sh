#!/usr/bin/env bash
# Run from an existing deploy/online installation after loading the release image.
set -euo pipefail
umask 077
cd "$(dirname "$0")/../.."
image="${1:?Usage: release-online.sh adobe-relay:COMMIT}"
[[ "$image" =~ ^adobe-relay:[a-zA-Z0-9._-]+$ ]] || exit 2
env_file=deploy/online/.env
base=(docker compose -f deploy/online/compose.yml --env-file "$env_file")
compose=(docker compose -f deploy/online/compose.yml -f deploy/clash/compose.yml --env-file "$env_file")
docker image inspect "$image" >/dev/null
docker image inspect metacubex/mihomo:v1.19.31 >/dev/null
app_id=$("${base[@]}" ps -q app)
postgres_id=$("${base[@]}" ps -q postgres)
[[ -n "$app_id" && -n "$postgres_id" ]] || { echo 'An existing running app and database are required.' >&2; exit 1; }
old_image=$(docker inspect "$app_id" --format '{{.Config.Image}}')
backup_dir="deploy/clash/backups.local/$(date -u +%Y%m%dT%H%M%SZ)-${image##*:}"
mkdir -p "$backup_dir"
cp "$env_file" "$backup_dir/environment"
cp deploy/online/compose.yml "$backup_dir/compose.yml"
docker exec "$postgres_id" pg_dump -U sub2api -d sub2api -Fc > "$backup_dir/database.dump"
docker exec -i "$postgres_id" pg_restore --list < "$backup_dir/database.dump" >/dev/null
docker exec "$app_id" tar -C /app -czf - data > "$backup_dir/app-data.tar.gz"
printf '%s\n' "$old_image" > "$backup_dir/previous-image"
echo "Backup verified: $backup_dir"
python3 deploy/clash/init_env.py "$env_file"
set_image() {
  python3 - "$env_file" "$1" <<'PY'
from pathlib import Path
import sys
p = Path(sys.argv[1])
lines = p.read_text().splitlines()
replacement = 'SUB2API_IMAGE=' + sys.argv[2]
if any(x.startswith('SUB2API_IMAGE=') for x in lines):
    lines = [replacement if x.startswith('SUB2API_IMAGE=') else x for x in lines]
else:
    lines.append(replacement)
p.write_text('\n'.join(lines) + '\n')
p.chmod(0o600)
PY
}
set_image "$image"
# Update only the app and its dedicated runtime; preserve database/cache containers.
if "${compose[@]}" up -d --no-build --no-deps --wait --wait-timeout 240 mihomo app &&
   "${compose[@]}" exec -T app wget -q -T 10 -O /dev/null http://127.0.0.1:8080/health &&
   "${compose[@]}" exec -T mihomo sh -ec 'wget -q -T 10 --header="Authorization: Bearer $CLASH_CONTROLLER_SECRET" -O /dev/null http://127.0.0.1:9090/version'; then
  echo "Released $image; application and Clash runtime are healthy."
else
  echo 'Release verification failed; restoring the previous application image.' >&2
  set_image "$old_image"
  "${compose[@]}" up -d --no-build --no-deps --wait --wait-timeout 120 app || true
  exit 1
fi
