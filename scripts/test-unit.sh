#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> unit tests (internal packages)"
go test ./internal/... -race -timeout 120s "$@"

echo "==> security tests"
go test ./tests/security/... -race -timeout 60s "$@"
