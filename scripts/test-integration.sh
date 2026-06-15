#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> integration tests"
go test -tags="integration,d1,d2,d3,d4,d5,d7,cross" ./tests/integration/... -race -timeout 300s "$@"
