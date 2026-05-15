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

if [[ -z "$PREFIX" || "$PREFIX" == "/" ]]; then
    echo "Refusing unsafe install prefix: $PREFIX" >&2
    exit 1
fi

if [[ -z "$APP_DIR" || "$APP_DIR" == "/" ]]; then
    echo "Refusing unsafe app directory: $APP_DIR" >&2
    exit 1
fi

if [[ "$PREFIX" == -* || "$APP_DIR" == -* ]]; then
    echo "Install paths must not start with '-'." >&2
    exit 1
fi

mkdir -p "$PREFIX" "$APP_DIR"

APP_DIR_REAL="$(cd "$APP_DIR" && pwd)"
if [[ "$APP_DIR_REAL" == "$SCRIPT_DIR" ]]; then
    echo "Refusing to install over the source checkout." >&2
    exit 1
fi

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
