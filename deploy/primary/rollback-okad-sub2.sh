#!/usr/bin/env bash
# Restore code only. Never replace business databases with pre-release backups.
set -euo pipefail
release="${1:?Usage: rollback-okad-sub2.sh RELEASE [production|main]}"
mode="${2:-production}"
[[ "$release" =~ ^[a-zA-Z0-9._-]+$ ]] || exit 2
[[ "$mode" == production || "$mode" == main ]] || exit 2
manifest="/opt/adobe-relay/releases/$release/pre-release.json"
mapfile -t images < <(python3 - "$manifest" "$mode" <<'PY'
import json, re, sys
manifest = json.load(open(sys.argv[1]))
for project in ('sub2', 'okad'):
    value = manifest[sys.argv[2]][project]
    prefix = 'adobe-relay' if project == 'sub2' else 'adobe-team-cookie'
    assert re.fullmatch(re.escape(prefix) + r':[a-zA-Z0-9._-]+', value)
    print(value)
PY
)
[[ ${#images[@]} == 2 ]] || exit 1
for image in "${images[@]}"; do docker image inspect "$image" >/dev/null; done
bash /opt/adobe-relay/deploy/primary/release.sh "${images[0]}"
bash /opt/adobe-relay/deploy/primary/okad-release.sh "${images[1]}"
curl --fail --silent --show-error --max-time 30 https://creative.cheap/health
curl --fail --silent --show-error --max-time 30 https://creative.cheap/okad/api/health
echo "Restored $release ($mode) code; current business data preserved."
