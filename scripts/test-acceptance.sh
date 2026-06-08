#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> acceptance tests (P0)"
go test -tags="acceptance,d1,d2,d4" ./tests/acceptance/p0/... -timeout 300s "$@"
