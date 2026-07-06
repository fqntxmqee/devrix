#!/usr/bin/env bash
# check-d7-d3-prepend-boundary.sh — CI guard for D7→D3 API boundary (DM-20260706-008)
#
# Background (DM-20260706-007 / DM-20260706-008):
#   sess_1783333760211_6000 飞书卡片失败事件根因 = D7 编排层 4 个 LLM proposer
#   之前有 2 个绕过 messagesForLLMInvoke 边界,导致 AGENTS.md / UserContextPrepend
#   被吞,LLM 拿不到 D{N} → path 映射。
#
# 协议约定 (llm_invoke_boundary.go §11):
#   "All LLM InvokeStream call sites in this package must route through this
#    helper so prepend policy stays single-sourced"
#
# 本 guard 扫描 internal/layers/orchestration/sessionorchestrator/ 包内
# 所有 LLMInvoker.InvokeStream 调用点,确保每一个调用站点前都已调过
# messagesForLLMInvoke(msgs, prepared.UserContextPrepend)。
#
# 退出码:
#   0 — 所有 InvokeStream 调用站点都经过 messagesForLLMInvoke 包装
#   1 — 存在 InvokeStream 绕过 messagesForLLMInvoke 的调用点
#   2 — 关键文件 (messagesForLLMInvoke / llm_invoke_boundary.go) 缺失
#
# 用法:
#   bash scripts/check-d7-d3-prepend-boundary.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ORCH_DIR="$REPO_ROOT/internal/layers/orchestration/sessionorchestrator"

EXIT_CODE=0

echo "==> D7→D3 prepend boundary CI guard (DM-20260706-008)"
echo

# ===== 1. 关键文件必须存在 =====
echo "[1] 检查关键文件存在..."
KEY_FILES=(
  "$ORCH_DIR/llm_invoke_boundary.go"
)
for f in "${KEY_FILES[@]}"; do
  if [[ ! -f "$f" ]]; then
    echo "    ✗ FAIL: 关键文件缺失 → $f"
    EXIT_CODE=2
  else
    echo "    ✓ OK: $f 存在"
  fi
done
echo

# Allow-list: 已知合法的"非 proposer InvokeStream"调用点
#  - semantic_verifier_default.go: Verify 节点的 template-mimicry 检测,
#    不通过 D2 MaterializeForMUPS,设计上无需 UserContextPrepend
#  - 后续若新增 verifier / utility 类 LLM 调用,必须在此处添加 allow-list
#    或改为走 messagesForLLMInvoke 包装
ALLOW_LIST=(
  "semantic_verifier_default.go"
)

# ===== 2. 收集所有 LLMInvoker.InvokeStream 调用点 =====
echo "[2] 扫描 InvokeStream 调用站点..."
INVOKE_SITES_RAW=$(grep -rn --include='*.go' -E '\.LLM\.InvokeStream\(|\.LLM\.InvokeNonStream\(' "$ORCH_DIR" \
  | grep -v '_test.go' \
  | grep -v 'llm_invoke_boundary.go' \
  || true)

# 过滤掉 allow-list 中的文件
INVOKE_SITES=""
while IFS= read -r line; do
  [[ -z "$line" ]] && continue
  skip=0
  for allow in "${ALLOW_LIST[@]}"; do
    if echo "$line" | grep -q "$allow"; then
      echo "    [allow-list] 跳过: $line"
      skip=1
      break
    fi
  done
  if [[ $skip -eq 0 ]]; then
    INVOKE_SITES+="$line"$'\n'
  fi
done <<< "$INVOKE_SITES_RAW"

if [[ -z "$INVOKE_SITES" ]]; then
  echo "    ⚠ WARN: 未发现 InvokeStream 调用点 (可能 LLMInvoker 已迁移)"
  exit 0
fi

SITE_COUNT=$(echo "$INVOKE_SITES" | wc -l | tr -d ' ')
echo "    发现 $SITE_COUNT 个 InvokeStream 调用点"
echo

# ===== 3. 检查每个调用站点所在函数是否含 messagesForLLMInvoke =====
echo "[3] 检查每个调用点是否经过 messagesForLLMInvoke..."

BAD_SITES=""
while IFS= read -r site; do
  [[ -z "$site" ]] && continue

  # 解析 file:line
  FILE=$(echo "$site" | cut -d: -f1)
  LINE=$(echo "$site" | cut -d: -f2)

  # 在同一文件内,从 LINE 向上扫描 30 行,看是否出现 messagesForLLMInvoke(
  CONTEXT=$(sed -n "$((LINE > 30 ? LINE - 30 : 1)),$((LINE - 1))p" "$FILE" 2>/dev/null || true)

  if echo "$CONTEXT" | grep -qE 'messagesForLLMInvoke\('; then
    # 调用点前 30 行有 messagesForLLMInvoke → OK
    echo "    ✓ OK: $FILE:$LINE 上方有 messagesForLLMInvoke"
  else
    BAD_SITES+="$FILE:$LINE"$'\n'
    echo "    ✗ FAIL: $FILE:$LINE 上方 30 行内无 messagesForLLMInvoke 调用"
  fi
done <<< "$INVOKE_SITES"

echo

# ===== 4. 检查 messagesForLLMInvoke 自身仍存在并被引用 =====
echo "[4] 检查 messagesForLLMInvoke 至少被 2 处引用..."
REFERENCES=$(grep -rn --include='*.go' 'messagesForLLMInvoke(' "$ORCH_DIR" \
  | grep -v 'llm_invoke_boundary.go' \
  | grep -v '_test.go' \
  | wc -l | tr -d ' ')

if [[ "$REFERENCES" -ge 2 ]]; then
  echo "    ✓ OK: messagesForLLMInvoke 已被 $REFERENCES 处引用 (≥ 2)"
else
  echo "    ✗ FAIL: messagesForLLMInvoke 仅被 $REFERENCES 处引用 (< 2, 至少需要 Observe + Plan + Execute 3 处)"
  EXIT_CODE=1
fi
echo

# ===== 5. 汇总 =====
if [[ -z "$BAD_SITES" && $EXIT_CODE -eq 0 ]]; then
  echo "==> ✅ PASS: D7→D3 prepend boundary 全 InvokeStream 调用站点均经过 messagesForLLMInvoke"
  exit 0
elif [[ -n "$BAD_SITES" ]]; then
  echo "==> ❌ FAIL: 以下 InvokeStream 调用点未经过 messagesForLLMInvoke:"
  echo "$BAD_SITES" | sed 's/^/      /'
  EXIT_CODE=1
fi

exit $EXIT_CODE