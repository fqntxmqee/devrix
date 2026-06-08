#!/usr/bin/env bash
# Run segmented tests for architecture domain D2, D3, or D4.
# Usage: ./scripts/test-domain.sh {d2|d3|d4} [--live] [--unit-only] [--cover] [go test args...]
set -euo pipefail

cd "$(dirname "$0")/.."

DOMAIN="${1:-}"
shift || true

LIVE=false
UNIT_ONLY=false
COVER=false
EXTRA_ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --live)
      LIVE=true
      shift
      ;;
    --unit-only)
      UNIT_ONLY=true
      shift
      ;;
    --cover)
      COVER=true
      shift
      ;;
    *)
      EXTRA_ARGS+=("$1")
      shift
      ;;
  esac
done

if [[ -z "$DOMAIN" || ! "$DOMAIN" =~ ^d[234]$ ]]; then
  echo "usage: $0 {d2|d3|d4} [--live] [--unit-only] [--cover] [go test args...]" >&2
  exit 2
fi

case "$DOMAIN" in
  d2)
    UNIT_PKGS=(./internal/layers/contextengine/...)
    LAYER_PATH="internal/layers/contextengine"
    INTEGRATION_FILTER="integration,d2,cross"
    ACCEPTANCE_FILTER="acceptance,d2"
    PERF_FILTER="performance,d2"
    ;;
  d3)
    UNIT_PKGS=(./internal/layers/llmgateway/...)
    LAYER_PATH="internal/layers/llmgateway"
    INTEGRATION_FILTER="integration,d3,cross"
    ACCEPTANCE_FILTER=""
    PERF_FILTER=""
    ;;
  d4)
    UNIT_PKGS=(./internal/layers/multiagent/...)
    LAYER_PATH="internal/layers/multiagent"
    INTEGRATION_FILTER="integration,d4,cross"
    ACCEPTANCE_FILTER="acceptance,d4"
    PERF_FILTER=""
    ;;
esac

COVER_ARGS=()
if $COVER; then
  PROFILE="/tmp/devrix_${DOMAIN}_unit.out"
  COVER_ARGS=(-coverprofile="$PROFILE" -covermode=atomic)
fi

echo "==> domain ${DOMAIN} unit tests"
go test "${UNIT_PKGS[@]}" -race -timeout 120s "${COVER_ARGS[@]}" "${EXTRA_ARGS[@]:-}"

if $COVER; then
  echo "==> domain ${DOMAIN} unit coverage"
  go tool cover -func="$PROFILE" | tail -1
fi

if $UNIT_ONLY; then
  exit 0
fi

echo "==> domain ${DOMAIN} integration tests"
INT_ARGS=(-race -timeout 300s)
if $COVER; then
  INT_PROFILE="/tmp/devrix_${DOMAIN}_integration.out"
  INT_ARGS+=(-coverprofile="$INT_PROFILE" -covermode=atomic -coverpkg="./${LAYER_PATH}/...")
fi
go test -tags="${INTEGRATION_FILTER}" ./tests/integration/... "${INT_ARGS[@]}" "${EXTRA_ARGS[@]:-}"

if $COVER; then
  echo "==> domain ${DOMAIN} integration coverage (${LAYER_PATH})"
  go tool cover -func="$INT_PROFILE" | tail -1
fi

if [[ -n "$ACCEPTANCE_FILTER" ]]; then
  echo "==> domain ${DOMAIN} acceptance tests"
  go test -tags="${ACCEPTANCE_FILTER}" ./tests/acceptance/p0/... -timeout 300s "${EXTRA_ARGS[@]:-}"
fi

if [[ "$DOMAIN" == "d2" && -n "$PERF_FILTER" ]]; then
  echo "==> domain ${DOMAIN} performance tests"
  go test -tags="${PERF_FILTER}" ./tests/performance/... -timeout 300s "${EXTRA_ARGS[@]:-}"
fi

if [[ "$DOMAIN" == "d4" ]]; then
  echo "==> domain ${DOMAIN} e2e smoke tests"
  go test -tags="smoke,d4" ./tests/e2e/... -timeout 120s "${EXTRA_ARGS[@]:-}"
fi

if $LIVE && [[ "$DOMAIN" == "d3" ]]; then
  echo "==> domain ${DOMAIN} live integration tests (requires API credentials)"
  go test -tags="integration,d3,live" ./tests/integration/... -timeout 600s "${EXTRA_ARGS[@]:-}"
fi
