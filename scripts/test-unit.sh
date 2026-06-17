#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

cover_args=()
if [[ "${CI_COVERAGE:-}" == "1" ]]; then
  cover_args=(-coverprofile=coverage.out -covermode=atomic)
fi

echo "==> unit tests (internal packages)"
go test ./internal/... -race -timeout 120s "${cover_args[@]}" "$@"

echo "==> security tests"
go test ./tests/security/... -race -timeout 60s "$@"
