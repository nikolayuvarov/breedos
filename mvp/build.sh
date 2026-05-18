#!/usr/bin/env bash
#
# build.sh — build a portable static BreedOS binary.
#
# CGO_ENABLED=0 produces a pure-Go binary with NO glibc dependency,
# so it runs on any Linux with a compatible kernel ABI — not just on
# systems whose glibc matches the build host. -trimpath and
# -ldflags='-s -w' strip debug info and absolute paths for smaller,
# reproducible artifacts.
#
# Usage:
#   ./build.sh                # writes ../breedos  (next to install.sh)
#   ./build.sh ./breedos      # writes mvp/breedos
#   ./build.sh /tmp/breedos   # arbitrary path
#
# Why this matters:
# A default `go build` links to the build host's glibc via cgo
# (net/os/user packages). The resulting binary requires the same or
# newer glibc on the target system, and fails with:
#     /lib/x86_64-linux-gnu/libc.so.6: version `GLIBC_2.34' not found
# on older targets. CGO_ENABLED=0 avoids that.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

OUTPUT="${1:-../breedos}"

# Resolve output to an absolute path for the log line
OUTPUT_DIR="$(cd "$(dirname "$OUTPUT")" && pwd)"
OUTPUT_NAME="$(basename "$OUTPUT")"
OUTPUT_ABS="${OUTPUT_DIR}/${OUTPUT_NAME}"

c_blue=$'\033[1;34m'
c_green=$'\033[1;32m'
c_reset=$'\033[0m'

printf '%s[build]%s Source:  %s\n' "$c_blue" "$c_reset" "$SCRIPT_DIR"
printf '%s[build]%s Output:  %s\n' "$c_blue" "$c_reset" "$OUTPUT_ABS"
printf '%s[build]%s Mode:    CGO_ENABLED=0, -trimpath, -ldflags='"'"'-s -w'"'"' (static, stripped)\n' "$c_blue" "$c_reset"
echo ""

CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "${OUTPUT_ABS}" .

echo ""
printf '%s[build]%s Done.\n' "$c_green" "$c_reset"
if command -v file >/dev/null 2>&1; then
  printf '%s[build]%s %s\n' "$c_blue" "$c_reset" "$(file "${OUTPUT_ABS}")"
fi
printf '%s[build]%s %s\n' "$c_blue" "$c_reset" "$(ls -la "${OUTPUT_ABS}")"
