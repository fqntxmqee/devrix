#!/usr/bin/env bash
# check-orch-rename.sh — D6 guard/ Orchestration* → Guard* 重命名 CI guard
#
# DM-20260621-011 PR-B: 在 guard/ 包内, Orchestration* 与 orch_* 标识符仅允许出现在
# type alias 定义点 + 文档注释中. 任何调用方代码 (非定义点) 使用旧名, 都应当被
# 改成 Guard* / guard_*. 后续 v2.5.0 将彻底删除 alias, 本 guard 提前阻断漏网.
#
# 注意: 本脚本**不**扫描 D7 orchestration 层 (那是不同域, 不在 PR-B scope).
# D7 的 `Orchestration*` 名称 (IOrchestrationEntry, SetOrchestrationEntry,
# InitOrchestration 等) 是 D7 编排层的语义命名, 不属于本 PR 范围.
#
# 退出码:
#   0 — 所有检查通过
#   1 — guard/ 包内出现非预期的 Orchestration* / orch_* 使用
#   2 — bridge 文件重新出现 (eval/bridge.go / orchestration/bridge.go)
#
# 用法:
#   bash scripts/check-orch-rename.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
GUARD_DIR="$REPO_ROOT/internal/layers/evolution/guard"
METRICS_FILE="$GUARD_DIR/metrics.go"

EXIT_CODE=0

echo "==> D6 orch→guard rename CI guard (DM-20260621-011 PR-B)"
echo

# ===== 1. bridge.go 必须不存在 =====
echo "[1] 检查 bridge 文件残留..."
BRIDGE_FILES=(
  "$REPO_ROOT/internal/layers/evolution/eval/bridge.go"
  "$REPO_ROOT/internal/layers/evolution/orchestration/bridge.go"
)
for f in "${BRIDGE_FILES[@]}"; do
  if [[ -f "$f" ]]; then
    echo "    ✗ FAIL: bridge 文件残留 → $f"
    EXIT_CODE=2
  else
    echo "    ✓ OK: $f 已删除"
  fi
done
echo

# ===== 2. guard/ 内 Orchestration* 仅允许出现在 alias 定义点 =====
# 允许的 alias 定义点 (allow-list by file:line pattern):
#   - guard/config.go:    type OrchestrationConfig = ...
#   - guard/validator.go: type RuntimeOrchestrationValidator = ... + func NewRuntimeOrchestrationValidator
#   - guard/observer.go:  type OrchestrationObserver = ... + func NewOrchestrationObserver
# 任何其他位置出现 Orchestration* 代码 (非 Deprecated 注释) 即 fail.
echo "[2] 扫描 guard/ 内 Orchestration* 使用..."

# 提取 guard/ 内非注释行 (排除 Deprecated / go:deprecated / // 开头的纯注释行)
# 然后筛选出包含 Orchestration* 的代码行.
GUARD_ORCH_LINES=$(grep -rn --include='*.go' -E 'Orchestration[A-Za-z]*' "$GUARD_DIR" \
  | awk -F: '
    {
      line = $0
      sub(/^[^:]+:[^:]+:/, "", line)
      # 跳过纯注释行
      if (line ~ /^[[:space:]]*\/\//) next
      if (line ~ /^[[:space:]]*\*/) next
      # 跳过 Deprecated: 和 go:deprecated 注释
      if (line ~ /Deprecated:/) next
      if (line ~ /go:deprecated/) next
      print $0
    }
  ')

# Allow-list: 已知 alias 定义点.
# 这些行号是稳定的 alias 定义, 不需要每次手改.
ALLOW_PATTERNS=(
  "config.go:.*type OrchestrationConfig = "
  "validator.go:.*type RuntimeOrchestrationValidator = "
  "validator.go:.*func NewRuntimeOrchestrationValidator"
  "observer.go:.*type OrchestrationObserver = "
  "observer.go:.*func NewOrchestrationObserver"
  # 允许引用 shared/config.OrchestrationConfig (PR-B 不动 shared/config 底层类型):
  "config.go:.*config\.OrchestrationConfig"
  "validator_test.go:.*config\.DefaultOrchestrationConfig"
)

UNEXPECTED=""
while IFS= read -r line; do
  [[ -z "$line" ]] && continue
  matched=0
  for pat in "${ALLOW_PATTERNS[@]}"; do
    if echo "$line" | grep -qE "$pat"; then
      matched=1
      break
    fi
  done
  if [[ $matched -eq 0 ]]; then
    UNEXPECTED+="$line"$'\n'
  fi
done <<< "$GUARD_ORCH_LINES"

if [[ -z "$UNEXPECTED" ]]; then
  echo "    ✓ OK: guard/ 内 Orchestration* 仅出现在 alias 定义点"
else
  echo "    ✗ FAIL: guard/ 内以下位置出现非预期 Orchestration* 代码使用:"
  echo "$UNEXPECTED" | sed 's/^/      /'
  EXIT_CODE=1
fi
echo

# ===== 3. metrics.go 内 orch_* 指标注册必须使用 guard_* =====
echo "[3] 检查 metrics.go 指标命名..."
REGISTERED_OLD=$(grep -E 'meter\.(Int64Counter|Float64Histogram|Int64UpDownCounter)\("orch_' "$METRICS_FILE" || true)
if [[ -z "$REGISTERED_OLD" ]]; then
  echo "    ✓ OK: metrics.go 不再以 orch_* 名称注册指标"
else
  echo "    ✗ FAIL: metrics.go 仍以旧名注册指标:"
  echo "$REGISTERED_OLD" | sed 's/^/      /'
  EXIT_CODE=1
fi

REGISTERED_NEW=$(grep -c -E 'meter\.(Int64Counter|Float64Histogram|Int64UpDownCounter)\("guard_' "$METRICS_FILE" || true)
if [[ "$REGISTERED_NEW" -ge 6 ]]; then
  echo "    ✓ OK: metrics.go 已注册 $REGISTERED_NEW 个 guard_* 指标 (≥ 6)"
else
  echo "    ✗ FAIL: metrics.go guard_* 指标数 = $REGISTERED_NEW, 期望 ≥ 6"
  EXIT_CODE=1
fi
echo

# ===== 4. 全仓 grep (非 guard/ 目录): guard package 调用方 =====
# 仅扫描本应调用 guard.Orchestration*/orch_* 但未迁移的代码.
# 排除:
#   - openspec/ 文档 (历史归档保留)
#   - internal/shared/config/ (shared/config.OrchestrationConfig 是底层类型, 不在 PR-B scope)
#   - internal/layers/evolution/guard/ (alias 定义点)
#   - tests/integration/d6/ (rename 测试自身)
#   - scripts/ (本脚本自身)
#   - internal/layers/orchestration/ (D7 编排层, 不同域)
#   - tests/testutil/orchestration_entry.go (D7 testutil)
#   - internal/layers/communication/capture/ (D7 gateway SetOrchestrationEntry, 不同域)
#   - internal/layers/observability/ (D7 telemetry spans, 不同域)
#   - internal/layers/contextengine/legacy/ (D7 legacy 错误信息, 提到 InitOrchestration)
#   - internal/bootstrap/ (D7 bootstrap.InitOrchestration, 不同域)
#   - internal/shared/contracts/ (D7 IOrchestrationEntry, 不同域)
# 重点扫描: cmd/devrix/main.go 中是否仍调用 guard.Orchestration* 旧 API.
echo "[4] 扫描 cmd/ 与 guard/ 之外的 D6-related 调用方..."

# 限定搜索: guard.NewOrchestrationObserver | guard.NewRuntimeOrchestrationValidator
# | guard.OrchestrationObserver | guard.RuntimeOrchestrationValidator | guard.OrchestrationConfig
# | guard.orchMetrics | guard.initOrchMetrics
GUARD_API_OLD=$(grep -rn --include='*.go' -E 'guard\.(NewOrchestrationObserver|NewRuntimeOrchestrationValidator|OrchestrationObserver|RuntimeOrchestrationValidator|OrchestrationConfig|orchMetrics|initOrchMetrics)' "$REPO_ROOT" \
  | grep -v 'internal/layers/evolution/guard/' \
  | grep -v 'tests/integration/d6/' \
  | awk -F: '
    {
      line = $0
      sub(/^[^:]+:[^:]+:/, "", line)
      if (line ~ /^[[:space:]]*\/\//) next
      if (line ~ /Deprecated:|go:deprecated/) next
      print $0
    }
  ' || true)

if [[ -z "$GUARD_API_OLD" ]]; then
  echo "    ✓ OK: 无非 guard/ 内部代码使用 guard.Orchestration* 旧 API"
else
  echo "    ✗ FAIL: 以下位置仍使用 guard.Orchestration* 旧 API (应迁移至 Guard*):"
  echo "$GUARD_API_OLD" | sed 's/^/      /'
  EXIT_CODE=1
fi
echo

# ===== 5. 全仓 orch_* 指标名 (非 metrics.go alias) =====
echo "[5] 扫描全仓 orch_* 指标名硬编码引用..."
ORCH_METRIC_USAGE=$(grep -rn --include='*.go' -E '"orch_[a-z_]+"' "$REPO_ROOT" \
  | grep -v 'internal/layers/evolution/guard/metrics.go' \
  | grep -v 'tests/integration/d6/' || true)

if [[ -z "$ORCH_METRIC_USAGE" ]]; then
  echo "    ✓ OK: 全仓无 orch_* 指标名硬编码引用"
else
  echo "    ✗ FAIL: 以下位置硬编码 orch_* 指标名 (应改为 guard_*):"
  echo "$ORCH_METRIC_USAGE" | sed 's/^/      /'
  EXIT_CODE=1
fi
echo

if [[ $EXIT_CODE -eq 0 ]]; then
  echo "==> ✅ PASS: orch→guard rename CI guard 全绿"
else
  echo "==> ❌ FAIL: orch→guard rename CI guard 存在违规 (exit=$EXIT_CODE)"
fi

exit $EXIT_CODE
