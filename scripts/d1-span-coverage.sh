#!/usr/bin/env bash
# d1-span-coverage.sh — DM-20260629-005 PR-4 #2 registry-sync (T15) Span Evidence Coverage Guard
#
# 目的: 检查 D1 t-registry 中所有 T 行 (排除 §T-Without-Span Tracker / §Statistics / §Revision History)
#       是否都有 Span Evidence 字段。守门阈值: ≥ 80% effective。
#
# 口径:
#   Effective Coverage = Mapped / (Total - ExplicitDash - LegacyDefault)
#   - Mapped: Span Evidence 字段含 `d1.*` / `eventbus.*` / `adapter.*` 字面量
#   - ExplicitDash: Span Evidence 以 `—（` 开头 (注入 / 启动 / 编译 / 边界)
#   - LegacyDefault: Span Evidence 含 "legacy span 已退役" (登记至 span-registry.md §Legacy)
#
# 用法:
#   ./scripts/d1-span-coverage.sh           # 检查 (exit 0/1)
#   ./scripts/d1-span-coverage.sh --strict  # 严格模式, 列出未映射的 T 行
#
# 退出码:
#   0 = PASS (≥ 80% effective)
#   1 = FAIL (有效覆盖率 < 80%)
#   2 = ERROR (文件未找到等)

set -euo pipefail

T_REGISTRY="openspec/specs/d1-communication/t-registry.md"
MIN_COVERAGE=80
MISSING_TMP=$(mktemp)
trap "rm -f $MISSING_TMP" EXIT

if [[ ! -f "$T_REGISTRY" ]]; then
  echo "ERROR: $T_REGISTRY not found" >&2
  exit 2
fi

# 1. 提取 §Canonical T 至 §T-Without-Span Tracker 之间所有 T 行
T_BLOCK=$(awk '/^## Canonical T/,/^## T-Without-Span Tracker/' "$T_REGISTRY")

RESULT=$(echo "$T_BLOCK" | awk -v missing_file="$MISSING_TMP" '
  BEGIN {
    total = 0
    mapped = 0
    explicit = 0
    legacy = 0
    in_table = 0
    ev_col = 0
  }
  # 检测表头行 (含 "Span Evidence")
  /^\| T ID .*\| Span Evidence \|/ {
    in_table = 1
    n = split($0, parts, / \| /)
    for (i = 1; i <= n; i++) {
      gsub(/^ *\|? *| *\|? *$/, "", parts[i])
      if (parts[i] == "Span Evidence") { ev_col = i; break }
    }
    next
  }
  # separator row
  /^\|[-\| ]+\|$/ { next }
  # T 数据行 (D1-S*  或  D1-RF-T*  或  **D1-*-T*** bold)
  in_table && (/^\| \*\*D1-[A-Z0-9\-]+T[0-9]+\*\* \|/ || /^\| D1-[A-Z0-9\-]+T[0-9]+ \|/) {
    if (ev_col == 0) next
    n = split($0, parts, / \| /)
    if (n < ev_col) next
    ev = parts[ev_col]
    gsub(/^ *\|? *| *\|? *$/, "", ev)
    gsub(/`/, "", ev)
    total++
    tid = parts[1]
    gsub(/^ *\|? *| *\|? *$/, "", tid)
    # legacy span 已退役 → 不计入分母
    if (ev ~ /legacy span 已退役/) {
      legacy++
    } else if (ev ~ /^—/) {
      explicit++
      print "  - " tid " ev=[" ev "]" >> missing_file
    } else if (ev == "") {
      explicit++
      print "  - " tid " ev=[EMPTY]" >> missing_file
    } else {
      mapped++
    }
  }
  END {
    print total " " mapped " " explicit " " legacy
  }
')

read -r TOTAL MAPPED EXPLICIT_DASH LEGACY <<< "$RESULT"

EXPECTED=$((TOTAL - EXPLICIT_DASH - LEGACY))
if [[ $EXPECTED -gt 0 ]]; then
  COVERAGE=$((MAPPED * 100 / EXPECTED))
else
  COVERAGE=0
fi

echo "=== D1 Span Evidence Coverage (DM-20260629-005 PR-4 T15) ==="
echo "T-registry: $T_REGISTRY"
echo "D1-domain T total: $TOTAL"
echo "  - Mapped (real d1.* / eventbus.* / adapter.* span refs): $MAPPED"
echo "  - Explicit '—' (sub-process / cross-domain / startup / boundary, not gap): $EXPLICIT_DASH"
echo "  - Legacy default '—' (registered span-registry.md §Legacy, excluded from denominator): $LEGACY"
echo "  - Expected (must-have Span Evidence): $EXPECTED"
echo "Effective Coverage: $MAPPED / $EXPECTED = ${COVERAGE}%"
echo "Threshold: ${MIN_COVERAGE}%"
echo ""

if [[ $COVERAGE -ge $MIN_COVERAGE ]]; then
  echo "PASS: Span Evidence Coverage ${COVERAGE}% ≥ ${MIN_COVERAGE}%"
  exit 0
else
  echo "FAIL: Span Evidence Coverage ${COVERAGE}% < ${MIN_COVERAGE}%"
  if [[ "${1:-}" == "--strict" ]] && [[ -s "$MISSING_TMP" ]]; then
    echo "STRICT mode: T rows with explicit '—' or empty Span Evidence:"
    cat "$MISSING_TMP"
  fi
  exit 1
fi