#!/usr/bin/env bash
# d4-span-coverage.sh — DM-20260629-004 PR-6 (T27) Span Evidence Coverage Guard
#
# 目的: 检查 t-registry 中所有 D4-* T 行 (除 §Legacy Archive 与 §Revision History)
#       是否都有 Span Evidence 字段（非空、非 "—" 标记）。守门阈值: ≥ 80% effective。
#
# 口径:
#   Effective Coverage = Mapped / (Total - ExplicitDash)
#   - Mapped: Span Evidence 字段包含 OpD4_S4_* 或 EventAgent* / EventPermissionRequired
#   - ExplicitDash: Span Evidence 为 "—" 或以 "— (xxx)" 开头（注入模式/启动期/跨域 owns）
#
# 用法:
#   ./scripts/d4-span-coverage.sh           # 检查 (exit 0/1)
#   ./scripts/d4-span-coverage.sh --strict  # 严格模式, 缺任一即 fail
#
# 退出码:
#   0 = PASS (≥ 80% effective)
#   1 = FAIL (有效覆盖率 < 80%)
#   2 = ERROR (文件未找到等)

set -euo pipefail

T_REGISTRY="openspec/specs/d4-multi-agent/t-registry.md"
MIN_COVERAGE=80
MISSING_TMP=$(mktemp)
trap "rm -f $MISSING_TMP" EXIT

if [[ ! -f "$T_REGISTRY" ]]; then
  echo "ERROR: $T_REGISTRY not found" >&2
  exit 2
fi

# 1. 提取所有 T 表：从 §D4-S1 至 §Legacy Archive（排除 §Legacy Archive + §Statistics + §T-Without-Span Tracker + §Revision History）
T_BLOCK=$(awk '/^## D4-S1:/,/^## §Legacy Archive/' "$T_REGISTRY")

TOTAL=0
MAPPED=0
EXPLICIT_DASH=0

# 2. 解析整个 T_BLOCK: 找每个表头, 找 Span Evidence 列号, 解析该表所有 T 行
RESULT=$(echo "$T_BLOCK" | awk -v missing_file="$MISSING_TMP" '
  BEGIN {
    total = 0
    mapped = 0
    explicit = 0
    in_table = 0
    ev_col = 0
  }
  # 检测表头行 (含 "Span Evidence")
  /^\| T ID .*\| Span Evidence \|/ {
    in_table = 1
    # 数 Span Evidence 在第几列 (| 分隔, 第 1 个字段是空)
    n = split($0, parts, / \| /)
    for (i = 1; i <= n; i++) {
      gsub(/^ +| +$/, "", parts[i])
      if (parts[i] == "Span Evidence") { ev_col = i; break }
    }
    next
  }
  # separator row (|---|---|)
  /^\|[-\| ]+\|$/ { next }
  # T 数据行 (D4-S{N}-A{XX}-T{NN} 或 D4-X-A{XX}-T{NN} 或 D4-X-T{NN}, 可能被 **bold**)
  in_table && (/^\| \*\*D4-[A-Z0-9]+-A?[0-9]*-T[0-9]+\*\* \|/ || /^\| D4-[A-Z0-9]+-A?[0-9]*-T[0-9]+ \|/) {
    if (ev_col == 0) next
    # 切分当前行
    n = split($0, parts, / \| /)
    if (n < ev_col) next
    ev = parts[ev_col]
    gsub(/^ +| +$/, "", ev)
    # 去掉 backticks
    gsub(/`/, "", ev)
    total++
    # mapped = ev 非空 且 不以 "—" 开头
    if (ev != "" && substr(ev, 1, 3) != "—") {
      mapped++
    } else {
      explicit++
      # 记录 T ID (第 1 个字段)
      tid = parts[1]
      gsub(/^ +| +$/, "", tid)
      print "  - " tid " ev=[" ev "]" >> missing_file
    }
  }
  # 表外行 (空行/正文) 结束当前表
  in_table && /^[^-]*$/ && !/^\| / && !/^$/ {
    in_table = 0
    ev_col = 0
  }
  END {
    print total " " mapped " " explicit
  }
')

read -r TOTAL MAPPED EXPLICIT_DASH <<< "$RESULT"

# 计算 effective coverage
EXPECTED=$((TOTAL - EXPLICIT_DASH))
if [[ $EXPECTED -gt 0 ]]; then
  COVERAGE=$((MAPPED * 100 / EXPECTED))
else
  COVERAGE=0
fi

echo "=== D4 Span Evidence Coverage (DM-20260629-004 PR-6 T27) ==="
echo "T-registry: $T_REGISTRY"
echo "D4-domain T total: $TOTAL"
echo "  - Mapped (real OpD4_S4_* / EventAgent* / EventPermissionRequired): $MAPPED"
echo "  - Explicit '—' (sub-process / cross-domain / config / factory, not gap): $EXPLICIT_DASH"
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