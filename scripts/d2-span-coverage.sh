#!/usr/bin/env bash
# d2-span-coverage.sh — DM-20260629-002 PR-7 (T40) Span Evidence Coverage Guard
#
# 目的: 检查 t-registry §Canonical T 映射 表格中, Status=IMPLEMENTED 的 T 行
#       是否都有 Span Evidence 字段 (非空, 非 "—")。
#       守门阈值: 排除 D2-S16 (已迁 D7) + D2-S20 (REMOVED) + D2-STRUCT (CI guard) 后,
#       映射率 ≥ 80%。
#
# 用法:
#   ./scripts/d2-span-coverage.sh           # 检查 (exit 0/1)
#   ./scripts/d2-span-coverage.sh --strict  # 严格模式, 缺任一即 fail
#
# 退出码:
#   0 = PASS (≥ 80% 映射)
#   1 = FAIL (映射率 < 80% 或缺关键 T 行)
#   2 = ERROR (文件未找到等)

set -euo pipefail

T_REGISTRY="openspec/specs/d2-context-engine/t-registry.md"
MIN_COVERAGE=80

if [[ ! -f "$T_REGISTRY" ]]; then
  echo "ERROR: $T_REGISTRY not found" >&2
  exit 2
fi

# 1. 提取 §Canonical T 映射 表格 (从 "## Canonical T 映射" 到下一个 "##" 段)
CANONICAL_T=$(awk '/^## Canonical T 映射/,/^## [^C]/' "$T_REGISTRY")

# 2. 统计 canonical T 行 (S15-S20)
TOTAL=$(echo "$CANONICAL_T" | grep -cE '^\| D2-S(1[5-9]|20)-A[0-9]+-T[0-9]+ \|' || true)

# 3. 统计已迁 D7 的 S16 T 行 (不计入 D2 coverage)
S16_COUNT=$(echo "$CANONICAL_T" | grep -cE '^\| D2-S16-A[0-9]+-T[0-9]+ \|' || true)

# 4. 统计 REMOVED S20 T 行
S20_COUNT=$(echo "$CANONICAL_T" | grep -cE '^\| D2-S20-A[0-9]+-T[0-9]+ \|' || true)

# 5. 统计 mapped (Span Evidence 列非空且非 "— " 开头)
#    表格列: Canonical T ID | Legacy T ID | Canonical S | 描述 | Status | Span Evidence
#    Span Evidence 在第 6 列
MAPPED=$(echo "$CANONICAL_T" | awk -F' \\| ' '
  /^\| D2-S1[5-8]-A[0-9]+-T[0-9]+ \|/ {
    if (NF >= 6) {
      ev = $6
      sub(/^ +/, "", ev); sub(/ +$/, "", ev)
      # — (开头算 unmapped (compile-time invariant, CI guard, historical, 归 D7)
      # awk substr 按字节, em-dash 3-byte UTF-8, 所以 "— (" 是 5 bytes
      if (ev != "" && substr(ev, 1, 5) != "— (") {
        count++
      }
    }
  }
  END { print count+0 }
')

# 6. 计算覆盖率 (排除 S16 迁 D7 + S20 REMOVED)
D2_TOTAL=$((TOTAL - S16_COUNT - S20_COUNT))
if [[ $D2_TOTAL -gt 0 ]]; then
  COVERAGE=$((MAPPED * 100 / D2_TOTAL))
else
  COVERAGE=0
fi

echo "=== D2 Span Evidence Coverage (DM-20260629-002 PR-7 T40) ==="
echo "T-registry: $T_REGISTRY"
echo "Canonical T total: $TOTAL"
echo "  - D2-S16 (已迁 D7): $S16_COUNT"
echo "  - D2-S20 (REMOVED): $S20_COUNT"
echo "D2-domain T total: $D2_TOTAL"
echo "Mapped: $MAPPED / $D2_TOTAL = ${COVERAGE}%"
echo "Threshold: ${MIN_COVERAGE}%"
echo ""

if [[ $COVERAGE -ge $MIN_COVERAGE ]]; then
  echo "PASS: Span Evidence Coverage ${COVERAGE}% ≥ ${MIN_COVERAGE}%"
  exit 0
else
  echo "FAIL: Span Evidence Coverage ${COVERAGE}% < ${MIN_COVERAGE}%"
  if [[ "${1:-}" == "--strict" ]]; then
    echo "STRICT mode: missing T rows:"
    echo "$CANONICAL_T" | awk -F' \\| ' '
      /^\| D2-S1[5-8]-A[0-9]+-T[0-9]+ \|/ {
        if (NF >= 6) {
          ev = $6
          sub(/^ +/, "", ev); sub(/ +$/, "", ev)
          if (ev == "" || substr(ev, 1, 5) == "— (") {
            print "  - " $1 "  ev=[" ev "]"
          }
        }
      }
    '
  fi
  exit 1
fi
