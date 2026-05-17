#!/usr/bin/env bash
#
# install.sh — install a Go binary as a systemd service.
#
# Tuned for the BreedOS MVP binary by default, but accepts arbitrary
# binary and service names. The binary must be present next to this
# script (built with: cd mvp && go build -o ../<binary_name> .).
#
# Usage:
#   sudo ./install.sh install   [binary_name] [service_name]
#   sudo ./install.sh uninstall [service_name]
#        ./install.sh info      [service_name]
#        ./install.sh help

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT_NAME="$(basename "${BASH_SOURCE[0]}")"
DEFAULT_BINARY="breedos"
DEFAULT_LISTEN="0.0.0.0:8080"
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
${SCRIPT_NAME} — systemd-service installer for the BreedOS MVP binary
              (or any Go binary placed next to this script).

USAGE
  sudo ./${SCRIPT_NAME} install   [binary_name] [service_name]
  sudo ./${SCRIPT_NAME} uninstall [service_name]
       ./${SCRIPT_NAME} info      [service_name]
       ./${SCRIPT_NAME} help

DEFAULTS
  binary_name   = ${DEFAULT_BINARY}
  service_name  = <binary_name>
  listen addr   = ${DEFAULT_LISTEN}   (for breedos: passed as '-listen <addr>')
  working dir   = ${SCRIPT_DIR}      (this script's directory)
  unit file     = ${UNIT_DIR}/<service_name>.service

REQUIREMENTS
  - systemd-based Linux
  - sudo / root privileges for 'install' and 'uninstall'
  - binary must be present next to this script: ${SCRIPT_DIR}/<binary_name>
    Build it with:
        cd mvp && go build -o ../<binary_name> .

EXAMPLES
  sudo ./${SCRIPT_NAME} install                       # install breedos with defaults
  sudo ./${SCRIPT_NAME} install breedos               # explicit binary name
  sudo ./${SCRIPT_NAME} install breedos breedos-prod  # service named 'breedos-prod'
  sudo ./${SCRIPT_NAME} uninstall breedos             # remove the service
       ./${SCRIPT_NAME} info      breedos             # show unit file + status + recent logs
EOF
}

# ---- internals ------------------------------------------------------------

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

extract_listen_value() {
  local args="$1"
  echo "$args" | sed -n 's/.*-listen[= ]*\([^ ]*\).*/\1/p'
}

# ---- install --------------------------------------------------------------

cmd_install() {
  local binary_name="${1:-$DEFAULT_BINARY}"
  local service_name="${2:-$binary_name}"
  local binary_path="${SCRIPT_DIR}/${binary_name}"
  local unit_path="${UNIT_DIR}/${service_name}.service"

  require_root

  log "Binary:       ${binary_path}"
  log "Service:      ${service_name}.service"
  log "Script dir:   ${SCRIPT_DIR}"

  # --- pre-flight: binary present? ---
  if [[ ! -f "$binary_path" ]]; then
    err "Binary not found: ${binary_path}
Build it first, e.g.:
    cd mvp && go build -o ../${binary_name} ."
  fi
  if [[ ! -x "$binary_path" ]]; then
    err "Binary exists but is not executable: ${binary_path}
Fix with:  chmod +x ${binary_path}"
  fi

  # --- pre-flight: service already exists? ---
  if service_exists "$service_name"; then
    warn "Service '${service_name}.service' already exists."
    read -r -p "[install] Overwrite existing service? [y/N] " ans
    case "${ans,,}" in
      y|yes) ;;
      *) err "Aborted by user." ;;
    esac
    log "Stopping and disabling existing service before overwrite..."
    systemctl stop    "${service_name}.service" 2>/dev/null || true
    systemctl disable "${service_name}.service" 2>/dev/null || true
  fi

  # --- prompt: arguments ---
  local default_args=""
  [[ "$binary_name" == "$DEFAULT_BINARY" ]] && default_args="-listen ${DEFAULT_LISTEN}"

  printf '\n'
  log "Command-line arguments for the service (optional)."
  if [[ -n "$default_args" ]]; then
    log "  Default for breedos: ${default_args}"
    log "  Press Enter to accept, or type your own (e.g. '-listen 0.0.0.0:9090')."
  else
    log "  No default — press Enter for empty args, or type custom args."
  fi
  read -r -p "[install] Args: " user_args
  local run_args="${user_args:-$default_args}"

  # --- prompt: user ---
  local default_user="${SUDO_USER:-root}"
  printf '\n'
  log "Run the service as which user."
  log "  Default: ${default_user}"
  read -r -p "[install] Run as user: " run_user
  run_user="${run_user:-$default_user}"

  if ! id -u "$run_user" >/dev/null 2>&1; then
    err "User '${run_user}' does not exist on this system."
  fi

  # --- prompt: working directory ---
  printf '\n'
  log "Working directory for the service."
  log "  Default: ${SCRIPT_DIR}"
  read -r -p "[install] Working dir: " work_dir
  work_dir="${work_dir:-$SCRIPT_DIR}"

  if [[ ! -d "$work_dir" ]]; then
    err "Working directory does not exist: ${work_dir}"
  fi

  # --- description ---
  local description
  if [[ "$binary_name" == "$DEFAULT_BINARY" ]]; then
    description="BreedOS — Decision engine for selection strategies in CRISPR-enabled crop breeding"
  else
    description="${service_name} (managed by ${binary_name})"
  fi

  # --- write unit file ---
  printf '\n'
  log "Writing unit file: ${unit_path}"
  cat >"$unit_path" <<UNIT
[Unit]
Description=${description}
Documentation=https://github.com/NikolayUvarov/breedos
After=network.target

[Service]
Type=simple
User=${run_user}
WorkingDirectory=${work_dir}
ExecStart=${binary_path} ${run_args}
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

  sleep 1

  # --- summary ---
  printf '\n'
  ok "Installed and started: ${service_name}.service"
  printf '\n'

  local listen_val=""
  if [[ "$binary_name" == "$DEFAULT_BINARY" ]]; then
    if [[ "$run_args" == *"-listen"* ]]; then
      listen_val="$(extract_listen_value "$run_args")"
    else
      listen_val="${DEFAULT_LISTEN} (default)"
    fi
  fi

  cat <<EOF
CONFIGURATION
  Service name:      ${service_name}.service
  Unit file:         ${unit_path}
  Binary:            ${binary_path}
  Working directory: ${work_dir}
  Run as user:       ${run_user}
  Arguments:         ${run_args:-<none>}
$( [[ -n "$listen_val" ]] && printf '  Listen address:    %s\n' "$listen_val" )

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
  sudo ./${SCRIPT_NAME} uninstall ${service_name}

EOF

  systemctl --no-pager --lines=10 status "${service_name}.service" || true
}

# ---- uninstall ------------------------------------------------------------

cmd_uninstall() {
  local service_name="${1:-$DEFAULT_BINARY}"
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
  local service_name="${1:-$DEFAULT_BINARY}"
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
