#!/bin/bash
# Local Quaker checks.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

bash -n qk quaker

if command -v go > /dev/null 2>&1; then
    GOCACHE="${GOCACHE:-/private/tmp/quaker-gocache}" \
    GOMODCACHE="${GOMODCACHE:-/private/tmp/quaker-gomodcache}" \
        go test ./...
else
    echo "Go not found; skipping Go tests."
fi
