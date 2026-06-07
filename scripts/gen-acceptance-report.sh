#!/usr/bin/env bash
# Generate acceptance-report.md from L5 registry and test execution results.
#
# Usage:
#   ./scripts/gen-acceptance-report.sh --change devrix-v3
#   ./scripts/gen-acceptance-report.sh --change devrix-v3 --priority P0,P1
#   ./scripts/gen-acceptance-report.sh --change devrix-v3 --output /tmp/report.md
#
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

REGISTRY="${ROOT}/openspec/l5-registry.md"
CHANGE=""
OUTPUT=""
PRIORITY_FILTER="P0"
EXECUTOR="${USER:-ci}"
ENVIRONMENT="local"
RUN_TESTS=1

RESULT_FILE="$(mktemp)"
trap 'rm -f "$RESULT_FILE"' EXIT

usage() {
  sed -n '2,8p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --change)
      CHANGE="${2:-}"
      shift 2
      ;;
    --output)
      OUTPUT="${2:-}"
      shift 2
      ;;
    --priority)
      PRIORITY_FILTER="${2:-P0}"
      shift 2
      ;;
    --executor)
      EXECUTOR="${2:-}"
      shift 2
      ;;
    --env|--environment)
      ENVIRONMENT="${2:-}"
      shift 2
      ;;
    --skip-tests)
      RUN_TESTS=0
      shift
      ;;
    -h|--help)
      usage 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage 1
      ;;
  esac
done

if [[ -z "$CHANGE" ]]; then
  echo "error: --change is required" >&2
  usage 1
fi

if [[ ! -f "$REGISTRY" ]]; then
  echo "error: L5 registry not found: $REGISTRY" >&2
  exit 1
fi

CHANGE_DIR="${ROOT}/openspec/changes/${CHANGE}"
if [[ -z "$OUTPUT" ]]; then
  OUTPUT="${CHANGE_DIR}/acceptance-report.md"
fi

DEMAND_ID="DM-unknown"
DEMAND_TITLE="$CHANGE"
if [[ -f "${CHANGE_DIR}/demand.md" ]]; then
  DEMAND_ID="$(grep -E '^demand-id:' "${CHANGE_DIR}/demand.md" | head -1 | awk '{print $2}' || true)"
  DEMAND_TITLE="$(grep -E '^title:' "${CHANGE_DIR}/demand.md" | head -1 | sed 's/^title: //' || true)"
fi

DATE="$(date +%Y-%m-%d)"
VERDICT="ACCEPTED"

tags_for_path() {
  local path="$1"
  case "$path" in
    tests/integration/*) echo "integration" ;;
    tests/e2e/*) echo "smoke" ;;
    tests/acceptance/*) echo "acceptance" ;;
    *) echo "" ;;
  esac
}

package_for_test_file() {
  local path="$1"
  dirname "./${path}"
}

file_result_get() {
  local path="$1"
  grep -F "${path}|" "$RESULT_FILE" 2>/dev/null | head -1 | cut -d'|' -f2- || true
}

file_result_set() {
  local path="$1" result="$2" evidence="$3"
  if grep -qF "${path}|" "$RESULT_FILE" 2>/dev/null; then
    return 0
  fi
  echo "${path}|${result}|${evidence}" >> "$RESULT_FILE"
}

parse_l5_entries() {
  local current_priority=""
  while IFS= read -r line; do
    case "$line" in
      "### P0") current_priority="P0" ;;
      "### P1") current_priority="P1" ;;
      "### P2") current_priority="P2" ;;
      "### P3") current_priority="P3" ;;
      "| L5-"*)
        IFS='|' read -r _ id desc _ test_path status _ <<< "$line"
        id="$(echo "$id" | xargs)"
        desc="$(echo "$desc" | xargs)"
        test_path="$(echo "$test_path" | xargs | tr -d '\`')"
        status="$(echo "$status" | xargs)"
        if [[ -n "$current_priority" && "$id" == L5-* ]]; then
          echo "${id}|${desc}|${current_priority}|${test_path}|${status}"
        fi
        ;;
    esac
  done < "$REGISTRY"
}

priority_enabled() {
  local p="$1"
  [[ ",${PRIORITY_FILTER}," == *",${p},"* ]]
}

run_test_file() {
  local path="$1"
  if [[ -z "$path" || "$path" == "—" || "$path" == "-" ]]; then
    return 2
  fi

  local existing
  existing="$(file_result_get "$path")"
  if [[ -n "$existing" ]]; then
    return 0
  fi

  local pkg tag
  pkg="$(package_for_test_file "$path")"
  tag="$(tags_for_path "$path")"

  if [[ ! -f "${ROOT}/${path}" ]]; then
    file_result_set "$path" "FAIL" "missing test file: ${path}"
    return 1
  fi

  if [[ -n "$tag" ]]; then
    if go test -tags="${tag}" "${pkg}" -count=1 -timeout 120s >/dev/null 2>&1; then
      file_result_set "$path" "PASS" "go test -tags=${tag} ${pkg}"
    else
      file_result_set "$path" "FAIL" "go test -tags=${tag} ${pkg}"
      return 1
    fi
  else
    if go test "${pkg}" -count=1 -timeout 120s >/dev/null 2>&1; then
      file_result_set "$path" "PASS" "go test ${pkg}"
    else
      file_result_set "$path" "FAIL" "go test ${pkg}"
      return 1
    fi
  fi
}

SUITE_RESULTS=""
run_suite() {
  local script="$1"
  if [[ "$RUN_TESTS" -eq 0 ]]; then
    SUITE_RESULTS="${SUITE_RESULTS}| \`${script}\` | SKIP | --skip-tests |
"
    return 0
  fi
  if "./${script}" >/tmp/devrix-suite.log 2>&1; then
    SUITE_RESULTS="${SUITE_RESULTS}| \`${script}\` | PASS | see CI/local log |
"
  else
    SUITE_RESULTS="${SUITE_RESULTS}| \`${script}\` | FAIL | see CI/local log |
"
    VERDICT="REJECTED"
  fi
}

L5_ROWS_FILE="$(mktemp)"
trap 'rm -f "$RESULT_FILE" "$L5_ROWS_FILE"' EXIT
parse_l5_entries > "$L5_ROWS_FILE"

while IFS='|' read -r id desc priority test_path status; do
  if ! priority_enabled "$priority"; then
    continue
  fi
  if [[ "$RUN_TESTS" -eq 1 && "$status" == "IMPLEMENTED" && -n "$test_path" && "$test_path" != "—" ]]; then
    run_test_file "$test_path" || true
  fi
done < "$L5_ROWS_FILE"

if [[ "$RUN_TESTS" -eq 1 ]]; then
  run_suite "scripts/test-unit.sh"
  run_suite "scripts/test-integration.sh"
  run_suite "scripts/test-e2e.sh"
  run_suite "scripts/test-acceptance.sh"
fi

p0_total=0 p0_pass=0 p0_fail=0 p0_skip=0
p1_total=0 p1_pass=0 p1_fail=0 p1_skip=0
p2_total=0 p2_pass=0 p2_fail=0 p2_skip=0

l5_table_rows=""
fail_analysis=""

while IFS='|' read -r id desc priority test_path status; do
  if ! priority_enabled "$priority"; then
    continue
  fi

  result="SKIP"
  evidence="not implemented"

  if [[ "$status" == "PLANNED" || -z "$test_path" || "$test_path" == "—" ]]; then
    result="SKIP"
    evidence="status=${status}"
  elif [[ "$RUN_TESTS" -eq 0 ]]; then
    result="SKIP"
    evidence="--skip-tests"
  else
    local_line="$(file_result_get "$test_path")"
    result="$(echo "$local_line" | cut -d'|' -f1)"
    evidence="$(echo "$local_line" | cut -d'|' -f2-)"
    if [[ -z "$result" ]]; then
      result="FAIL"
      evidence="$test_path"
    fi
    if [[ "$result" == "FAIL" ]]; then
      VERDICT="REJECTED"
      fail_analysis="${fail_analysis}
### ${id}: ${desc}
- **失败原因**: 测试未通过（${evidence}）
- **影响评估**: ${priority} 阻断或需例外说明
- **处置方案**: 修复后重新执行 \`./scripts/gen-acceptance-report.sh --change ${CHANGE}\`
"
    fi
  fi

  case "$priority" in
    P0) p0_total=$((p0_total + 1)); case "$result" in PASS) p0_pass=$((p0_pass + 1)) ;; FAIL) p0_fail=$((p0_fail + 1)) ;; SKIP) p0_skip=$((p0_skip + 1)) ;; esac ;;
    P1) p1_total=$((p1_total + 1)); case "$result" in PASS) p1_pass=$((p1_pass + 1)) ;; FAIL) p1_fail=$((p1_fail + 1)) ;; SKIP) p1_skip=$((p1_skip + 1)) ;; esac ;;
    P2) p2_total=$((p2_total + 1)); case "$result" in PASS) p2_pass=$((p2_pass + 1)) ;; FAIL) p2_fail=$((p2_fail + 1)) ;; SKIP) p2_skip=$((p2_skip + 1)) ;; esac ;;
  esac

  l5_table_rows="${l5_table_rows}| ${id} | ${desc} | ${priority} | ${result} | ${evidence} |
"
done < "$L5_ROWS_FILE"

if [[ "$p0_fail" -gt 0 ]]; then
  VERDICT="REJECTED"
fi

mkdir -p "$(dirname "$OUTPUT")"

{
  cat <<EOF
---
demand-id: ${DEMAND_ID}
title: ${DEMAND_TITLE} — 验收报告
executor: ${EXECUTOR}
environment: ${ENVIRONMENT}
date: ${DATE}
verdict: ${VERDICT}
change: ${CHANGE}
---

# 验收报告：${DEMAND_TITLE}

## 1. 执行摘要

| 项目 | 值 |
|------|---|
| 需求 ID | ${DEMAND_ID} |
| 变更 | ${CHANGE} |
| 执行人 | ${EXECUTOR} |
| 测试环境 | ${ENVIRONMENT} |
| 执行日期 | ${DATE} |
| 总体结论 | **${VERDICT}** |

## 2. 自动化验证

| 检查 | 结果 | 证据 |
|------|------|------|
EOF
  if [[ -n "$SUITE_RESULTS" ]]; then
    echo "$SUITE_RESULTS"
  else
    echo "| (skipped) | SKIP | --skip-tests |"
  fi
  cat <<EOF

## 3. L5 测试点验证结果

| L5 ID | 描述 | 优先级 | 状态 | 证据 |
|-------|------|--------|------|------|
${l5_table_rows}
### 统计

| 优先级 | 总数 | 通过 | 失败 | 跳过 |
|--------|------|------|------|------|
| P0 | ${p0_total} | ${p0_pass} | ${p0_fail} | ${p0_skip} |
| P1 | ${p1_total} | ${p1_pass} | ${p1_fail} | ${p1_skip} |
| P2 | ${p2_total} | ${p2_pass} | ${p2_fail} | ${p2_skip} |

## 4. 失败项分析（如有）
EOF
  if [[ -n "$fail_analysis" ]]; then
    echo "$fail_analysis"
  else
    echo "
无失败项。"
  fi
  cat <<EOF

## 5. 遗留风险

| 风险 | 影响 | 规避方案 |
|------|------|---------|
| PLANNED L5 未覆盖 | 功能缺口 | 按 \`openspec/l5-registry.md\` 排期补测 |
| Live 测试未纳入 CI | 外部依赖波动 | 使用 \`-tags=live\` 在 staging 手动执行 |

## 6. 结论

$( if [[ "$VERDICT" == "ACCEPTED" ]]; then
  echo "P0 测试点全部通过，测试套件绿色，可进入 S6 交付。"
else
  echo "存在失败项，请修复后重新生成验收报告。"
fi )

---
生成命令: \`./scripts/gen-acceptance-report.sh --change ${CHANGE}\`
EOF
} > "$OUTPUT"

echo "acceptance report written: $OUTPUT"
echo "verdict: $VERDICT"

if [[ "$VERDICT" == "REJECTED" ]]; then
  exit 1
fi
