#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> e2e smoke tests"
go test -tags=smoke ./tests/e2e/... -timeout 120s "$@"
