#!/usr/bin/env bash
# Property Rights Audit — scans for task semantics outside D7 WorkTree (AC25).
set -euo pipefail

ROOT="${1:-.}"
VIOLATIONS=0

echo "=== Property Rights Audit (devrix-unified-work-tree AC25) ==="
echo "Root: $ROOT"
echo

echo "--- D2 sc.Todos direct writes (allowed: todo_tool.go projection path) ---"
while IFS= read -r line; do
  file="${line%%:*}"
  if [[ "$file" == *todo_tool.go ]]; then
    continue
  fi
  echo "VIOLATION: sc.Todos write outside projection: $line"
  VIOLATIONS=$((VIOLATIONS + 1))
done < <(rg -n 'sc\.Todos\s*=' "$ROOT/internal/layers/contextengine" 2>/dev/null || true)

echo
echo "--- New *Registry / *Manager types outside orchestration (AC23) ---"
while IFS= read -r line; do
  file="${line%%:*}"
  if [[ "$file" == *orchestration/runregistry/* ]] || [[ "$file" == *orchestration/workmodel/* ]]; then
    continue
  fi
  if [[ "$file" == *nested/background.go* ]] || [[ "$file" == *multiagent/* ]]; then
    continue
  fi
  echo "WARN: task-related registry outside D7: $line"
  VIOLATIONS=$((VIOLATIONS + 1))
done < <(rg -n 'type [A-Za-z]*(Registry|Manager) struct' "$ROOT/internal/layers/contextengine" 2>/dev/null || true)

echo
echo "--- Summary ---"
if [[ "$VIOLATIONS" -eq 0 ]]; then
  echo "PASS: no property-rights violations detected (baseline clean)"
  exit 0
fi
echo "FOUND: $VIOLATIONS issue(s) — review required"
exit 0
