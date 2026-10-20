#!/usr/bin/env bash
# Run on the release runner after verifying the archive's SHA-256 sidecar.
# Keep the archive compressed over SSH; loading an image does not replace app.
set -euo pipefail
archive="${1:?Usage: load-image.sh ARCHIVE root@173.234.15.150 [SSH_OPTIONS...]}"
target="${2:?A primary-server SSH target is required}"
shift 2
[[ "$target" == root@173.234.15.150 ]] || {
  echo 'Only the configured primary server is a release target.' >&2
  exit 2
}
[[ -f "$archive" && -r "$archive" ]] || {
  echo 'The verified image archive is not readable.' >&2
  exit 2
}
# Both gzip and docker failures must reach the caller, including a corrupt
# stream that docker consumes before gzip detects the invalid trailer.
ssh "$@" "$target" 'bash -o pipefail -c "gzip -dc | docker load"' < "$archive"
