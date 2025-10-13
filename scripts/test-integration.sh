#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export GOCACHE="${GOCACHE:-$(pwd)/.gocache}"
export GOMODCACHE="${GOMODCACHE:-$(pwd)/.gomodcache}"
export GOTMUXCC_INTEGRATION=${GOTMUXCC_INTEGRATION:-1}
GO=${GO:-go}
exec "$GO" test -tags integration ./...
