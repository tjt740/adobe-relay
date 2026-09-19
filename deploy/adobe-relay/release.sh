#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
image="${1:?Usage: release.sh adobe-relay:TAG}"
[[ "$image" =~ ^adobe-relay:[a-zA-Z0-9._-]+$ ]] || exit 2
python3 deploy/adobe-relay/init_env.py
env_file=deploy/adobe-relay/.env
old_image=$(sed -n 's/^SUB2API_IMAGE=//p' "$env_file")
compose=(docker compose -f deploy/adobe-relay/compose.yml --env-file "$env_file")
set_image() {
  python3 - "$env_file" "$1" <<'PY'
from pathlib import Path
import sys
p = Path(sys.argv[1])
lines = p.read_text().splitlines()
p.write_text('\n'.join('SUB2API_IMAGE=' + sys.argv[2] if x.startswith('SUB2API_IMAGE=') else x for x in lines) + '\n')
p.chmod(0o600)
PY
}
docker image inspect "$image" >/dev/null
set_image "$image"
if "${compose[@]}" up -d --no-build --wait --wait-timeout 240; then
  curl --fail --retry 5 --retry-delay 3 http://127.0.0.1:9500/health
else
  set_image "$old_image"
  if docker image inspect "$old_image" >/dev/null 2>&1; then
    "${compose[@]}" up -d --no-build --wait --wait-timeout 120 || true
  fi
  exit 1
fi
