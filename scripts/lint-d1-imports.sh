#!/usr/bin/env bash
# D1 import boundary lint (DM-20260628-003 Phase 4).
# Fails when D1 presentation packages import D4/D7 implementation paths.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "==> D1 import boundary tests"
go test ./internal/layers/communication/capture/ -run TestD1Capture_NoForbiddenImports -count=1
go test ./internal/layers/communication/channel/adapters/ -run TestD1ChannelAdapters_NoForbiddenImports -count=1

DIRS=(
  internal/layers/communication/thinking
  internal/layers/communication/taskprogress
  internal/layers/communication/conclusion
  internal/layers/communication/delivery
)

echo "==> D1 presentation package import scan"
failed=0
for dir in "${DIRS[@]}"; do
  [[ -d "$dir" ]] || continue
  while IFS= read -r -d '' file; do
    if grep -q 'internal/layers/multiagent' "$file" 2>/dev/null; then
      echo "FORBIDDEN: $file imports multiagent"
      failed=1
    fi
    if grep -q 'internal/layers/orchestration/' "$file" 2>/dev/null; then
      echo "FORBIDDEN: $file imports orchestration"
      failed=1
    fi
  done < <(find "$dir" -name '*.go' ! -name '*_test.go' -print0)
done

if [[ "$failed" -ne 0 ]]; then
  echo "D1 import lint failed"
  exit 1
fi

echo "D1 import lint passed"
