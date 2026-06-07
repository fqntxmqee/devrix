#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> integration tests"
go test -tags=integration ./tests/integration/... -race -timeout 300s "$@"
