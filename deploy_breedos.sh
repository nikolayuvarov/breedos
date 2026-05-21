#!/usr/bin/env bash
#
# deploy_breedos.sh — build the breedos MVP binary and ship it to a
# remote host as <binary>.UPDATE. A running breedos service (v0.7.1+)
# detects the .UPDATE file via its self-update watcher, runs
# `--self-check`, swaps the running binary atomically, and exits so
# systemd restarts the new version.
#
# Configuration:
#   Defaults live in a local `.env` file next to this script (gitignored).
#   See `.env.example` for the expected variables.
#
# Usage:
#   ./deploy_breedos.sh                                   # use .env defaults
#   ./deploy_breedos.sh user@host:/absolute/path/to/engine/
#   BREEDOS_DEPLOY_TARGET=user@host:/abs/path/ ./deploy_breedos.sh
#   BREEDOS_BINARY=other-binary ./deploy_breedos.sh user@host:/path/

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${SCRIPT_DIR}/.env"
ENV_EXAMPLE="${SCRIPT_DIR}/.env.example"

# Load defaults from .env if present. Variables set in the current
# environment take precedence over .env (we only fill the gaps).
if [[ -f "$ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
fi

BINARY_NAME="${BREEDOS_BINARY:-breedos}"

print_help() {
  cat <<EOF
deploy_breedos.sh — build + scp the breedos binary to a remote host as <binary>.UPDATE

USAGE
  $(basename "$0")                                      # use defaults from .env
  $(basename "$0") user@host:/absolute/path/to/engine/  # override target
  BREEDOS_DEPLOY_TARGET=user@host:/abs/path/ $(basename "$0")
  BREEDOS_BINARY=other-binary $(basename "$0") user@host:/path/

CONFIGURATION
  Defaults are read from a local \`.env\` file next to this script.
  The .env file is gitignored — each operator keeps their own.

  Expected variables (see .env.example for a template):
    BREEDOS_DEPLOY_TARGET   user@host:/absolute/path/to/engine/   (required)
    BREEDOS_BINARY          breedos                               (optional, default: breedos)

  To create it from the template:
    cp ${ENV_EXAMPLE} ${ENV_FILE}
    \$EDITOR ${ENV_FILE}

OVERRIDE PRECEDENCE (highest first)
  1. Positional argument: \$1
  2. Environment variable: BREEDOS_DEPLOY_TARGET (from current shell or .env)

REQUIREMENTS
  - SSH access to the target host (key-based recommended; the script
    invokes scp directly and inherits your shell's SSH agent).
  - mvp/build.sh present next to this script.
  - The remote breedos service must be v0.7.1 or later (which knows
    about the .UPDATE self-update contract). For older targets, deploy
    the binary manually with install.sh first.

EXAMPLES
  $(basename "$0")
  $(basename "$0") backup@example.com:/home/backup/unred/hosts/www.breedos.org/engine/
EOF
}

case "${1:-}" in
  -h|--help|help)
    print_help
    exit 0
    ;;
esac

TARGET="${1:-${BREEDOS_DEPLOY_TARGET:-}}"

if [[ -z "$TARGET" ]]; then
  echo "[deploy] ERROR: no deploy target configured." >&2
  echo "" >&2
  if [[ ! -f "$ENV_FILE" ]]; then
    echo "[deploy] No .env file found at: ${ENV_FILE}" >&2
    if [[ -f "$ENV_EXAMPLE" ]]; then
      echo "[deploy] Create one from the template:" >&2
      echo "[deploy]     cp ${ENV_EXAMPLE} ${ENV_FILE}" >&2
      echo "[deploy]     \$EDITOR ${ENV_FILE}" >&2
    fi
    echo "" >&2
  else
    echo "[deploy] .env exists but does not set BREEDOS_DEPLOY_TARGET." >&2
    echo "" >&2
  fi
  echo "[deploy] Or pass the target as a positional argument:" >&2
  echo "[deploy]     $(basename "$0") user@host:/absolute/path/to/engine/" >&2
  echo "" >&2
  echo "[deploy] Run with --help for full documentation." >&2
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
