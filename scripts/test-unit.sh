#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export GOCACHE="${GOCACHE:-$(pwd)/.gocache}"
export GOMODCACHE="${GOMODCACHE:-$(pwd)/.gomodcache}"
GO=${GO:-go}
exec "$GO" test ./...
