#!/usr/bin/env bash
# Fast deterministic eval for CI / local smoke.
# Uses mock judge and stratified sampling to stay under ~90s.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

DATASET="${EVAL_DATASET:-openspec/eval-datasets/v1/dataset.yaml}"
BASELINE="${EVAL_BASELINE:-openspec/eval-datasets/v1/baseline.yaml}"
MAX_ITEMS="${EVAL_MAX_ITEMS:-20}"
OUTPUT="${EVAL_OUTPUT:-}"

args=(
  run ./cmd/devrix
  eval run
  --dataset "$DATASET"
  --baseline "$BASELINE"
  --max-items "$MAX_ITEMS"
  --mock-judge
  --summary
  --gate
)

if [[ -n "$OUTPUT" ]]; then
  args+=(--output "$OUTPUT")
fi

go "${args[@]}"
