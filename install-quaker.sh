#!/bin/bash
# Install Quaker from a checked-out source tree.

set -euo pipefail

PREFIX="${PREFIX:-$HOME/.local/bin}"
APP_DIR="${APP_DIR:-$HOME/.local/share/quaker}"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --prefix)
            PREFIX="${2:-}"
            shift 2
            ;;
        --app-dir)
            APP_DIR="${2:-}"
            shift 2
            ;;
        -h | --help)
            echo "Usage: ./install-quaker.sh [--prefix DIR] [--app-dir DIR]"
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
    esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

mkdir -p "$PREFIX" "$APP_DIR"

if command -v go > /dev/null 2>&1; then
    (cd "$SCRIPT_DIR" && make build)
fi

for item in qk quaker bin cmd go.mod LICENSE NOTICE.md QUAKER.md README.md; do
    if [[ -e "$SCRIPT_DIR/$item" ]]; then
        rm -rf "$APP_DIR/$item"
        cp -R "$SCRIPT_DIR/$item" "$APP_DIR/$item"
    fi
done

chmod +x "$APP_DIR/qk" "$APP_DIR/quaker" 2> /dev/null || true
ln -sf "$APP_DIR/qk" "$PREFIX/qk"
ln -sf "$APP_DIR/quaker" "$PREFIX/quaker"

cat <<EOF
Quaker installed.

Binary: $PREFIX/qk
Alias:  $PREFIX/quaker
State:  ~/.quaker

Quaker is an independent macOS maintenance tool with local memory, policy rules, and safe automation.
EOF
