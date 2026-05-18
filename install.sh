#!/usr/bin/env bash
#
# install.sh — install a Go binary as a systemd service.
#
# Generic installer. No binary-specific defaults — the binary is
# responsible for its own runtime defaults. Empty args means empty
# args (no flags passed to ExecStart).
#
# Build a portable binary first (no glibc version pinning):
#     ./mvp/build.sh                                                 (recommended)
#     # or:  cd mvp && CGO_ENABLED=0 go build -o ../<binary_name> .
#
# Usage:
#   sudo ./install.sh install    [flags]
#   sudo ./install.sh uninstall  [flags]
#        ./install.sh info       [flags]
#        ./install.sh help

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT_NAME="$(basename "${BASH_SOURCE[0]}")"
DEFAULT_BINARY="breedos"
UNIT_DIR="/etc/systemd/system"

# ---- pretty output --------------------------------------------------------

c_blue=$'\033[1;34m'
c_yellow=$'\033[1;33m'
c_red=$'\033[1;31m'
c_green=$'\033[1;32m'
c_reset=$'\033[0m'

log()  { printf '%s[install]%s %s\n' "$c_blue"   "$c_reset" "$*"; }
warn() { printf '%s[install]%s %s\n' "$c_yellow" "$c_reset" "$*" >&2; }
err()  { printf '%s[install]%s %s\n' "$c_red"    "$c_reset" "$*" >&2; exit 1; }
ok()   { printf '%s[install]%s %s\n' "$c_green"  "$c_reset" "$*"; }

# ---- help -----------------------------------------------------------------

usage() {
  cat <<EOF
${SCRIPT_NAME} — generic systemd-service installer for a Go binary placed
              next to this script. No binary-specific defaults: the binary
              uses its own runtime defaults when no args are given.

USAGE
  sudo ./${SCRIPT_NAME} install    [flags]
  sudo ./${SCRIPT_NAME} uninstall  [flags]
       ./${SCRIPT_NAME} info       [flags]
       ./${SCRIPT_NAME} help

INSTALL FLAGS
  -b, --binary       NAME       Binary file name (default: ${DEFAULT_BINARY})
                                Looked up next to this script: ${SCRIPT_DIR}/<NAME>
  -s, --service      NAME       systemd service name (default: <binary>)
  -a, --args         "STRING"   Command-line arguments passed to the binary
                                (default: empty — binary uses its own defaults)
  -u, --user         NAME       User to run the service as (default: \$SUDO_USER or root)
  -w, --workdir      PATH       Working directory (default: script's directory)
  -d, --description  "STRING"   systemd unit Description= (default: '<service> service')
  -y, --yes, --non-interactive  Skip prompts, use provided flags / defaults only
  -f, --force                   Overwrite existing service without prompting

UNINSTALL FLAGS / INFO FLAGS
  -s, --service NAME            Service name (default: ${DEFAULT_BINARY})

DEFAULTS
  binary       = ${DEFAULT_BINARY}
  service      = <binary>
  args         = (empty — binary handles its own defaults)
  user         = \$SUDO_USER or root
  workdir      = ${SCRIPT_DIR}
  description  = <service> service
  unit file    = ${UNIT_DIR}/<service>.service

REQUIREMENTS
  - systemd-based Linux
  - sudo / root privileges for 'install' and 'uninstall'
  - binary present next to this script: ${SCRIPT_DIR}/<binary>
    Build a portable binary (no glibc version pinning):
        ./mvp/build.sh
    or:
        cd mvp && CGO_ENABLED=0 go build -o ../<binary> .

EXAMPLES
  sudo ./${SCRIPT_NAME} install                                   # interactive, all defaults
  sudo ./${SCRIPT_NAME} install -u backup                          # different run user, rest interactive
  sudo ./${SCRIPT_NAME} install -y -u backup                       # non-interactive, backup user, no args
  sudo ./${SCRIPT_NAME} install -y -a "-flag value -other"         # non-interactive with custom args
  sudo ./${SCRIPT_NAME} install -s breedos-prod -y                 # service 'breedos-prod', non-interactive
  sudo ./${SCRIPT_NAME} uninstall -s breedos                       # remove the service
       ./${SCRIPT_NAME} info      -s breedos                       # show unit file + status + recent logs
EOF
}

# ---- helpers --------------------------------------------------------------

require_root() {
  if [[ $EUID -ne 0 ]]; then
    err "This action requires root privileges. Re-run with sudo."
  fi
}

service_exists() {
  local sn="$1"
  systemctl list-unit-files --type=service 2>/dev/null \
    | awk '{print $1}' \
    | grep -qx "${sn}.service"
}

# Try to run the binary briefly to surface dynamic-loader errors
# (e.g. glibc version mismatch) BEFORE installing it as a service.
preflight_binary() {
  local binary_path="$1"
  local binary_name="$2"
  local out rc

  # Try '-h' — Go flag package prints help to stderr and exits 2 on -h.
  # Loader failures usually exit with 127 or print "GLIBC" / "symbol not found".
  out="$("$binary_path" -h </dev/null 2>&1 || true)"
  rc=$?

  if echo "$out" | grep -qiE 'GLIBC|symbol .* not found|version .* not found|cannot (open|load) shared object|exec format error'; then
    err "Binary loader / runtime check failed for:
    ${binary_path}

Error output:
$(echo "$out" | sed 's/^/    /')

This is typically a glibc version mismatch — the binary was built against
a newer glibc than this system has. Rebuild as a portable static binary:

    ./mvp/build.sh
  or:
    cd mvp && CGO_ENABLED=0 go build -o ../${binary_name} .

CGO_ENABLED=0 produces a fully static Go binary with no glibc dependency."
  fi

  # Quick second check by running the binary with a tiny timeout
  if command -v timeout >/dev/null 2>&1; then
    out="$(timeout 0.3 "$binary_path" </dev/null 2>&1 || true)"
    if echo "$out" | grep -qiE 'GLIBC|symbol .* not found|cannot (open|load) shared object'; then
      err "Binary loader check failed:
$(echo "$out" | sed 's/^/    /')

Rebuild as a static binary:
    ./mvp/build.sh"
    fi
  fi
}

# After 'systemctl start', wait briefly and verify the service is actually
# running (not crash-looping). systemd's 'start' returns success even if the
# process exits immediately, so we must check Active state explicitly.
verify_service_running() {
  local service_name="$1"
  local max_wait="${2:-5}"
  local i=0
  local state

  while (( i < max_wait )); do
    state="$(systemctl is-active "${service_name}.service" 2>/dev/null || true)"
    if [[ "$state" == "active" ]]; then
      return 0
    fi
    if [[ "$state" == "failed" ]]; then
      return 1
    fi
    sleep 1
    ((i++)) || true
  done

  state="$(systemctl is-active "${service_name}.service" 2>/dev/null || true)"
  [[ "$state" == "active" ]]
}

# ---- install --------------------------------------------------------------

cmd_install() {
  # parse flags
  local binary_name=""
  local service_name=""
  local user_args=""
  local args_set=0
  local run_user=""
  local work_dir=""
  local description=""
  local non_interactive=0
  local force=0

  while [[ $# -gt 0 ]]; do
    case "$1" in
      -b|--binary)       binary_name="$2"; shift 2 ;;
      -s|--service)      service_name="$2"; shift 2 ;;
      -a|--args)         user_args="$2"; args_set=1; shift 2 ;;
      -u|--user)         run_user="$2"; shift 2 ;;
      -w|--workdir)      work_dir="$2"; shift 2 ;;
      -d|--description)  description="$2"; shift 2 ;;
      -y|--yes|--non-interactive) non_interactive=1; shift ;;
      -f|--force)        force=1; shift ;;
      -h|--help)         usage; exit 0 ;;
      --)                shift; break ;;
      -*)                err "Unknown flag: $1   (use './${SCRIPT_NAME} help' for usage)" ;;
      *)                 err "Unexpected positional argument: '$1'. This script uses flags (see './${SCRIPT_NAME} help')." ;;
    esac
  done

  binary_name="${binary_name:-$DEFAULT_BINARY}"
  service_name="${service_name:-$binary_name}"
  local binary_path="${SCRIPT_DIR}/${binary_name}"
  local unit_path="${UNIT_DIR}/${service_name}.service"

  require_root

  log "Binary:       ${binary_path}"
  log "Service:      ${service_name}.service"
  log "Script dir:   ${SCRIPT_DIR}"

  # --- pre-flight: binary present and executable ---
  if [[ ! -f "$binary_path" ]]; then
    err "Binary not found: ${binary_path}
Build a portable binary first:
    ./mvp/build.sh
  or:
    cd mvp && CGO_ENABLED=0 go build -o ../${binary_name} ."
  fi
  if [[ ! -x "$binary_path" ]]; then
    err "Binary exists but is not executable: ${binary_path}
Fix with:  chmod +x ${binary_path}"
  fi

  # --- pre-flight: binary actually runs (no glibc mismatch, etc.) ---
  preflight_binary "$binary_path" "$binary_name"

  # --- pre-flight: service already exists? ---
  if service_exists "$service_name"; then
    if (( force )); then
      log "Service '${service_name}.service' exists; --force given, overwriting."
    elif (( non_interactive )); then
      err "Service '${service_name}.service' already exists.
Use --force to overwrite (or --service NAME with a different name)."
    else
      warn "Service '${service_name}.service' already exists."
      read -r -p "[install] Overwrite existing service? [y/N] " ans
      case "${ans,,}" in
        y|yes) ;;
        *) err "Aborted by user." ;;
      esac
    fi
    log "Stopping and disabling existing service before overwrite..."
    systemctl stop    "${service_name}.service" 2>/dev/null || true
    systemctl disable "${service_name}.service" 2>/dev/null || true
  fi

  # --- resolve args (flag → empty if not set; empty means binary uses own defaults) ---
  if (( ! args_set && ! non_interactive )); then
    printf '\n'
    log "Command-line arguments for the service (optional)."
    log "  Leave empty if the binary handles its own defaults."
    read -r -p "[install] Args: " user_args
  fi

  # --- resolve user ---
  local default_user="${SUDO_USER:-root}"
  if [[ -z "$run_user" ]]; then
    if (( non_interactive )); then
      run_user="$default_user"
    else
      printf '\n'
      log "Run the service as which user."
      log "  Default: ${default_user}"
      read -r -p "[install] Run as user: " run_user
      run_user="${run_user:-$default_user}"
    fi
  fi
  if ! id -u "$run_user" >/dev/null 2>&1; then
    err "User '${run_user}' does not exist on this system."
  fi

  # --- resolve working directory ---
  if [[ -z "$work_dir" ]]; then
    if (( non_interactive )); then
      work_dir="$SCRIPT_DIR"
    else
      printf '\n'
      log "Working directory for the service."
      log "  Default: ${SCRIPT_DIR}"
      read -r -p "[install] Working dir: " work_dir
      work_dir="${work_dir:-$SCRIPT_DIR}"
    fi
  fi
  if [[ ! -d "$work_dir" ]]; then
    err "Working directory does not exist: ${work_dir}"
  fi

  # --- resolve description (flag → generic) ---
  if [[ -z "$description" ]]; then
    description="${service_name} service"
  fi

  # --- write unit file ---
  printf '\n'
  log "Writing unit file: ${unit_path}"
  cat >"$unit_path" <<UNIT
[Unit]
Description=${description}
After=network.target

[Service]
Type=simple
User=${run_user}
WorkingDirectory=${work_dir}
ExecStart=${binary_path} ${user_args}
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
UNIT

  chmod 0644 "$unit_path"

  systemctl daemon-reload
  systemctl enable "${service_name}.service" >/dev/null
  log "Starting ${service_name}.service ..."
  systemctl start "${service_name}.service"

  # --- verify it actually came up ---
  if verify_service_running "$service_name" 5; then
    ok "Service is active: ${service_name}.service"
  else
    warn "Service did NOT reach 'active' state after start."
    warn "Recent logs:"
    journalctl -u "${service_name}" -n 25 --no-pager >&2 2>/dev/null || true
    err "Install completed but service is not running. Check the unit file:
    ${unit_path}
and the logs above. Common causes:
  - port already in use (check: sudo ss -tlnp | grep ':<port>')
  - binary loader error (rebuild with CGO_ENABLED=0 / ./mvp/build.sh)
  - working directory or binary not readable by user '${run_user}'"
  fi

  # --- summary ---
  printf '\n'
  cat <<EOF
CONFIGURATION
  Service name:      ${service_name}.service
  Unit file:         ${unit_path}
  Binary:            ${binary_path}
  Working directory: ${work_dir}
  Run as user:       ${run_user}
  Arguments:         ${user_args:-<empty — binary uses its own defaults>}
  Description:       ${description}

MANAGE THE SERVICE
  sudo systemctl status   ${service_name}
  sudo systemctl restart  ${service_name}
  sudo systemctl stop     ${service_name}
  sudo systemctl start    ${service_name}
  sudo systemctl disable  ${service_name}    # disable autostart
  sudo systemctl enable   ${service_name}    # enable autostart

VIEW LOGS
  sudo journalctl -u ${service_name} -f                       # follow live
  sudo journalctl -u ${service_name} -n 200 --no-pager        # last 200 lines
  sudo journalctl -u ${service_name} --since "1 hour ago"     # last hour

UNINSTALL
  sudo ./${SCRIPT_NAME} uninstall --service ${service_name}

EOF

  systemctl --no-pager --lines=10 status "${service_name}.service" || true
}

# ---- uninstall ------------------------------------------------------------

cmd_uninstall() {
  local service_name=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      -s|--service) service_name="$2"; shift 2 ;;
      -h|--help)    usage; exit 0 ;;
      --)           shift; break ;;
      -*)           err "Unknown flag: $1" ;;
      *)            err "Unexpected positional argument: '$1'. Use --service NAME." ;;
    esac
  done

  service_name="${service_name:-$DEFAULT_BINARY}"
  local unit_path="${UNIT_DIR}/${service_name}.service"

  require_root

  if [[ ! -f "$unit_path" ]]; then
    err "Service file not found: ${unit_path}
Service '${service_name}' is not installed system-wide (or installed elsewhere)."
  fi

  log "Stopping ${service_name}.service ..."
  systemctl stop "${service_name}.service" 2>/dev/null || true
  log "Disabling ${service_name}.service ..."
  systemctl disable "${service_name}.service" 2>/dev/null || true
  log "Removing ${unit_path} ..."
  rm -f "$unit_path"
  systemctl daemon-reload
  systemctl reset-failed "${service_name}.service" 2>/dev/null || true

  ok "Uninstalled: ${service_name}.service"
}

# ---- info -----------------------------------------------------------------

cmd_info() {
  local service_name=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      -s|--service) service_name="$2"; shift 2 ;;
      -h|--help)    usage; exit 0 ;;
      --)           shift; break ;;
      -*)           err "Unknown flag: $1" ;;
      *)            err "Unexpected positional argument: '$1'. Use --service NAME." ;;
    esac
  done

  service_name="${service_name:-$DEFAULT_BINARY}"
  local unit_path="${UNIT_DIR}/${service_name}.service"

  if [[ ! -f "$unit_path" ]]; then
    err "Service file not found: ${unit_path}
Service '${service_name}' is not installed system-wide."
  fi

  printf '\n'
  log "Service unit:  ${unit_path}"
  printf '\n--- unit file ---\n'
  cat "$unit_path"
  printf '\n'
  log "Current status:"
  systemctl --no-pager --lines=10 status "${service_name}.service" || true
  printf '\n'
  log "Recent logs (last 20 lines):"
  journalctl -u "${service_name}" -n 20 --no-pager 2>/dev/null || true
}

# ---- dispatch -------------------------------------------------------------

main() {
  local action="${1:-help}"
  shift || true

  case "$action" in
    install)          cmd_install   "$@" ;;
    uninstall|remove) cmd_uninstall "$@" ;;
    info|status)      cmd_info      "$@" ;;
    help|-h|--help|"") usage ;;
    *)
      warn "Unknown action: ${action}"
      usage
      exit 1
      ;;
  esac
}

main "$@"
