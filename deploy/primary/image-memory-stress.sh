#!/usr/bin/env bash
# Isolated synthetic replay. Never points at an application, DB or Adobe.
set -euo pipefail
test_binary=${1:?path to a static Linux middleware test binary}
base_image=${2:-postgres:18-alpine}
test_binary=$(cd "$(dirname "$test_binary")" && pwd)/$(basename "$test_binary")
work=$(mktemp -d)
container=
cleanup() {
  if [ -n "$container" ]; then docker rm -f "$container" >/dev/null 2>&1 || true; fi
  rm -rf "$work"
}
trap cleanup EXIT
for scenario in legacy-2g guarded-2g guarded-8g; do
  mode=${scenario%-*}
  memory=${scenario##*-}
  go_memory=1536MiB
  if [ "$memory" = 8g ]; then go_memory=6144MiB; fi
  container=$(docker create --network none --memory "$memory" --memory-swap "$memory" --cpus 2 \
    --entrypoint /memory.test \
    --mount "type=bind,src=$test_binary,dst=/memory.test,readonly" \
    -e "GOMEMLIMIT=$go_memory" -e "ADOBE_MEMORY_REPLAY=$mode" \
    "$base_image" -test.run '^TestImageMemoryCgroupReplay$' -test.timeout 3m -test.v)
  docker start -a "$container" > "$work/$scenario.log" 2>&1 || true
  cat "$work/$scenario.log"
  read -r code oom < <(docker inspect -f '{{.State.ExitCode}} {{.State.OOMKilled}}' "$container")
  printf 'memory_replay_container scenario=%s exit=%s oom_killed=%s\n' "$scenario" "$code" "$oom"
  if [ "$mode" = legacy ]; then
    test "$code" = 137 && test "$oom" = true
  else
    test "$code" = 0 && test "$oom" = false
  fi
  docker rm "$container" >/dev/null
  container=
done
