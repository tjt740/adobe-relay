#!/usr/bin/env bash
# Network-free synthetic workload. Run only with at least 13 GiB available.
set -euo pipefail
test_binary=${1:?static Linux middleware test binary}
output=${2:?audit output directory}
mkdir -p "$output"
container=
cleanup() {
  if [ -n "$container" ]; then docker rm -f "$container" >/dev/null 2>&1 || true; fi
}
trap cleanup EXIT
available=$(awk '/MemAvailable/ {print $2}' /proc/meminfo)
(( available > 13 * 1024 * 1024 )) || { echo 'Insufficient host headroom for isolated replay'; exit 1; }
for scenario in small-8g large-8g compressed-8g small-12g large-12g compressed-12g; do
  mode=${scenario%-*}
  memory=${scenario##*-}
  go_memory=6144MiB
  if [ "$memory" = 12g ]; then go_memory=9216MiB; fi
  container=$(docker create --network none --memory "$memory" --memory-swap "$memory" --cpus 2 \
    --entrypoint /memory.test \
    --mount "type=bind,src=$test_binary,dst=/memory.test,readonly" \
    -e "GOMEMLIMIT=$go_memory" -e "ADOBE_ADAPTIVE_REPLAY=$mode" \
    postgres:18-alpine -test.run '^TestImageMemoryAdaptiveCgroupReplay$' -test.timeout 2m -test.v)
  docker start -a "$container" > "$output/$scenario.log" 2>&1 || true
  cat "$output/$scenario.log"
  read -r code oom < <(docker inspect -f '{{.State.ExitCode}} {{.State.OOMKilled}}' "$container")
  printf 'memory_replay_container scenario=%s exit=%s oom_killed=%s\n' "$scenario" "$code" "$oom" | tee "$output/$scenario.result"
  test "$code" = 0 && test "$oom" = false
  docker rm "$container" >/dev/null
  container=
done
