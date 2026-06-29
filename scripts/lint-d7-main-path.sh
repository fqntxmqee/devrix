#!/usr/bin/env bash
# D7 main-path architecture lint (TurnLoop + WorkTree + MUPS).
# Ensures user-message ingress never regresses to retired FastPath/OrchestratePath.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "==> D7 main-path architecture tests"
go test ./internal/layers/orchestration/sessionorchestrator/ \
  -run 'TestD7MainPath_' -count=1

echo "==> D7 retired ingress file scan"
failed=0
for f in \
  internal/layers/orchestration/sessionorchestrator/fastpath.go \
  internal/layers/orchestration/sessionorchestrator/orchestrate_path.go
do
  if [[ -f "$f" ]]; then
    echo "FORBIDDEN: retired ingress file present: $f"
    failed=1
  fi
done

echo "==> D7 ProcessMessage ingress scan"
orch="$ROOT/internal/layers/orchestration/sessionorchestrator/orchestrator.go"
if ! grep -q 'RunSessionTurnLoop' "$orch"; then
  echo "MISSING: orchestrator.go must call RunSessionTurnLoop for user messages"
  failed=1
fi
if grep -qE 'FastPath\.Run|OrchestratePath\.Run' "$orch"; then
  echo "FORBIDDEN: orchestrator.go references retired FastPath/OrchestratePath"
  failed=1
fi

echo "==> D7 bootstrap wiring scan"
wire="$ROOT/internal/bootstrap/wire_coordinator.go"
if ! grep -q 'WithItemPipelineRunner' "$wire"; then
  echo "MISSING: wire_coordinator.go must wire WithItemPipelineRunner"
  failed=1
fi
if ! grep -q 'WithTaskManager' "$wire"; then
  echo "MISSING: wire_coordinator.go must wire WithTaskManager (WorkTree)"
  failed=1
fi

if [[ "$failed" -ne 0 ]]; then
  echo "D7 main-path lint failed"
  exit 1
fi

echo "D7 main-path lint passed"
