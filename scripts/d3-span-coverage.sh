#!/usr/bin/env bash
# d3-span-coverage.sh — DM-20260629-003 PR-6 (T33) Span Evidence Coverage Guard
#
# 目的: 检查 t-registry 中所有 D3-* T 行 (§2~§8) 是否都有 Span Evidence 字段
#       (非空, 非 "—" 标记)。守门阈值: ≥ 80%。
#
# 用法:
#   ./scripts/d3-span-coverage.sh           # 检查 (exit 0/1)
#   ./scripts/d3-span-coverage.sh --strict  # 严格模式, 缺任一即 fail
#
# 退出码:
#   0 = PASS (≥ 80% 映射)
#   1 = FAIL (映射率 < 80% 或缺关键 T 行)
#   2 = ERROR (文件未找到等)

set -euo pipefail

T_REGISTRY="openspec/specs/d3-llm-gateway/t-registry.md"
MIN_COVERAGE=80
MISSING_TMP=$(mktemp)
trap "rm -f $MISSING_TMP" EXIT

if [[ ! -f "$T_REGISTRY" ]]; then
  echo "ERROR: $T_REGISTRY not found" >&2
  exit 2
fi

# 1. 提取所有 T 表 (从 §2 D3-S1 到 §8 D3-X, 排除 §9 Legacy Archive + §10 Statistics + §11 关联文档)
T_BLOCK=$(awk '/^## 2\. D3-S1/,/^## 9\. Legacy Archive/' "$T_REGISTRY")

TOTAL=0
MAPPED=0

# 2. 解析整个 T_BLOCK: 找每个表头, 找 Span Evidence 列号, 解析该表所有 T 行
#    用 awk 一遍处理 (避免 subshell 状态丢失)
RESULT=$(echo "$T_BLOCK" | awk -v missing_file="$MISSING_TMP" '
  BEGIN {
    total = 0
    mapped = 0
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
  # T 数据行 (D3-S{N}-A{XX}-T{NN} 或 D3-X-A{XX}-T{NN} 或 D3-EC-T{NN}, 可能被 **bold**)
  in_table && (/^\| \*\*D3-[A-Z0-9]+-A[0-9]+-T[0-9]+\*\* \|/ || /^\| D3-[A-Z0-9]+-A[0-9]+-T[0-9]+ \|/ || /^\| D3-EC-T[0-9]+ \|/) {
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
    print total " " mapped
  }
')

read -r TOTAL MAPPED <<< "$RESULT"

if [[ $TOTAL -gt 0 ]]; then
  COVERAGE=$((MAPPED * 100 / TOTAL))
else
  COVERAGE=0
fi

EXPLICIT_DASH=$((TOTAL - MAPPED))
# 排除显式 '—' 标记的 T (注入模式 / 启动期 / 编译期 — 与 D2 / D7 v9.0.0 处理一致)
EXPECTED=$((TOTAL - EXPLICIT_DASH))
if [[ $EXPECTED -gt 0 ]]; then
  COVERAGE=$((MAPPED * 100 / EXPECTED))
else
  COVERAGE=0
fi

echo "=== D3 Span Evidence Coverage (DM-20260629-003 PR-6 T33) ==="
echo "T-registry: $T_REGISTRY"
echo "D3-domain T total: $TOTAL"
echo "  - Mapped (real span/metric/event name): $MAPPED"
echo "  - Explicit '—' (injection/startup/compile, not gap): $EXPLICIT_DASH"
echo "  - Expected (must-have Span Evidence): $EXPECTED"
echo "Coverage: $MAPPED / $EXPECTED = ${COVERAGE}%"
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
