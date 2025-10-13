#!/usr/bin/env bash
export GOCACHE=$(pwd)/.gocache
export GOMODCACHE=$(pwd)/.gomodcache
GOTMUXCC_INTEGRATION=1 scripts/test-unit.sh
