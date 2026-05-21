#!/usr/bin/env bash
#
# deploy_breedos.sh — build the breedos MVP binary and ship it to a
# remote host as <binary>.UPDATE. A running breedos service (v0.7.1+)
# detects the .UPDATE file via its self-update watcher, runs
# `--self-check`, swaps the running binary atomically, and exits so
# systemd restarts the new version.
#
# Usage:
#   ./deploy_breedos.sh user@host:/absolute/path/to/engine/
#   BREEDOS_DEPLOY_TARGET=user@host:/abs/path/ ./deploy_breedos.sh
#   BREEDOS_BINARY=other-binary ./deploy_breedos.sh user@host:/path/
#
# Notes:
# - The target must end with `/` (a directory; the script appends
#   `<BINARY_NAME>.UPDATE` for the remote filename).
# - The local binary is built with mvp/build.sh (CGO_ENABLED=0, static,
#   stripped). Local `--self-check` is run before scp so that broken
#   builds never reach the remote host.
# - First-time deploy to a host running v0.7.0 or earlier must still
#   be done manually (the older binary does not understand .UPDATE).
# - This script does not run anything on the remote host. The running
#   service on the remote does the swap on its own.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY_NAME="${BREEDOS_BINARY:-breedos}"
TARGET="${1:-${BREEDOS_DEPLOY_TARGET:-}}"

if [[ -z "$TARGET" ]]; then
  cat >&2 <<EOF
deploy_breedos.sh — build + scp the breedos binary to a remote host as <binary>.UPDATE

USAGE
  $(basename "$0") user@host:/absolute/path/to/engine/
  BREEDOS_DEPLOY_TARGET=user@host:/abs/path/ $(basename "$0")
  BREEDOS_BINARY=other-binary $(basename "$0") user@host:/path/

REQUIREMENTS
  - SSH access to the target host (key-based recommended; the script
    invokes scp directly and inherits your shell's SSH agent).
  - mvp/build.sh present next to this script.
  - The remote breedos service must be v0.7.1 or later (which knows
    about the .UPDATE self-update contract). For older targets, deploy
    the binary manually with install.sh first.

EXAMPLE
  $(basename "$0") backup@example.com:/home/backup/unred/hosts/www.breedos.org/engine/
EOF
  exit 2
fi

case "$TARGET" in
  */) ;;
  *)
    echo "[deploy] ERROR: TARGET must end with '/'  (got: $TARGET)" >&2
    exit 2
    ;;
esac

LOCAL_BUILD_DIR="$(mktemp -d "/tmp/breedos-deploy.XXXXXX")"
LOCAL_BUILD="${LOCAL_BUILD_DIR}/${BINARY_NAME}"
cleanup() { rm -rf "$LOCAL_BUILD_DIR"; }
trap cleanup EXIT

cd "$SCRIPT_DIR"

echo "[deploy] Building portable static binary -> ${LOCAL_BUILD}"
./mvp/build.sh "$LOCAL_BUILD"

echo "[deploy] Verifying local --self-check"
result="$("$LOCAL_BUILD" --self-check 2>&1 || true)"
if [[ "$(echo "$result" | tr -d '[:space:]')" != "OK" ]]; then
  echo "[deploy] ERROR: local binary --self-check did not return OK" >&2
  echo "[deploy] output: ${result}" >&2
  exit 3
fi
echo "[deploy] local --self-check: OK"

REMOTE_FILE="${TARGET}${BINARY_NAME}.UPDATE"
echo "[deploy] scp ${LOCAL_BUILD} -> ${REMOTE_FILE}"
scp "$LOCAL_BUILD" "$REMOTE_FILE"

cat <<EOF

[deploy] Upload complete. The running breedos service on the remote
host will detect the .UPDATE file within ~60 seconds, run --self-check
on the candidate, swap the binary atomically, and exit so systemd
restarts the new version.

To monitor:
    ssh <host> 'sudo journalctl -u breedos -f'

If the candidate fails --self-check on the remote, the .UPDATE file
will be left in place for inspection and the running service will
continue serving the previous binary.
EOF
