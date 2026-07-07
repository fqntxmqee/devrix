# Design: D7 Proposer UserContextPrepend + ReportSummary 信息密度修复

## 1. 修复原则(三条)

### 1.1 D2→D3 API 边界强制

`proposer.InvokeStream` 必须经过 `messagesForLLMInvoke(msgs, prepared.UserContextPrepend)` 包装,
这是 D2-prepared context 到 D3-LLM request 的**唯一允许路径**。任何绕过此路径的
proposer 调用都视为协议违规(由 `scripts/check-d7-d3-prepend-boundary.sh` CI guard 强制)。

```
                  ┌─────────────────────────────────┐
                  │  D2 ContextEngine (prepared)    │
                  │  UserContextPrepend (AGENTS.md) │
                  │  Prior / Reputation / i18n etc  │
                  └────────────────┬────────────────┘
                                   │ 唯一允许路径
                                   ▼
                  ┌─────────────────────────────────┐
                  │  messagesForLLMInvoke(msgs,     │
                  │                      prepend)   │ ← boundary function
                  └────────────────┬────────────────┘
                                   │
                                   ▼
                  ┌─────────────────────────────────┐
                  │  D3 LLMGateway InvokeStream     │
                  │  system = wrapped system_prompt │
                  │  + AGENTS.md prepend            │
                  └─────────────────────────────────┘
```

`messagesForLLMInvoke` 是 D2 边界唯一对外暴露的 wrapper,由 D3 消费 prepared context。
proposer 直接调 `InvokeStream(req.LLMInvokeRequest)` 跳过 wrapper = D2 context 丢失。

### 1.2 UncertaintyReport 消费契约对齐 design.md §7.2

Plan 节点输入 = `UncertaintyReport{Observations, UncertaintyCoord, Anomalies, QuantizedIntent}`。
**实现不应把 4 维对象坍塌为 1 字符串**;序列化是必要的(LLM 文本消费),但**必须保留
CatBusiness Observations 的语义内容**,不能仅用 Numeric anomaly count 代替。

原契约(PR #449 前):
```
"observation_summary": "intent=explore; anomalies=2"
```
- 丢失:ObsUncertainty.Question("用户想问的是 X 还是 Y?")
- 丢失:ObsDeviation.Statement("实测 ≠ 预期,差 12%")
- 后果:Plan LLM 看不到用户问题,只能基于异常计数瞎猜

新契约(PR #449 后):
```
"observation_summary": "intent=explore; anomalies=2; q=用户想问的是 X 还是 Y?; dev=实测≠预期,差 12%"
```
- 保留:CatBusiness ObsUncertainty/ObsDeviation 语义内容
- 强度过滤:strength ≥ 0.7(避免噪声)
- 向后兼容:旧 `intent=<kind>` 永远保留(`parts = append(parts, "intent="+intentKind)` 仍第一段)

### 1.3 partition by Category 设计必须被尊重

CatBusiness 走 `BusinessObservations`(LLM-driven ObsUncertainty 的归宿),
CatSystem + ObsDeviation / 高强度 ObsUncertainty 走 `Anomalies`。
**任何下游消费者必须明确自己消费的 partition**,不能假设 "Anomalies = 全部 Observations"。

`uncertaintyReportSummary` 内部已正确 partition(PR #449 前也有),但**只统计了
Anomalies 的 Numeric count,忽略了 BusinessObservations 的语义内容**——这是
Bug B 的根因。

## 2. 修复实现路径

### 2.1 PR #449(DM-20260706-007)— Observe + Plan 修复

| 文件 | 改动 | 备注 |
|------|------|------|
| `deliverable_execute.go` | `uncertaintyReportSignature(anomalyCount)` → `uncertaintyReportSummary(report)` 签名扩展 + 扫描 Observations | Fix A |
| `llm_observation_proposer.go:55` | `InvokeStream(req)` → 走 `messagesForLLMInvoke(msgs, prepared.UserContextPrepend)` | Fix B |
| `strategic_plan_proposer.go:403` | `InvokeStream(req)` → 走 `messagesForLLMInvoke(msgs, prepared.UserContextPrepend)` | Fix B |
| `deliverable_execute_test.go` | +5 unit tests 覆盖 ObsUncertainty/ObsDeviation serialization + strength 阈值 | T01 |
| `llm_observation_proposer_test.go` | +1 unit test 覆盖 UserContextPrepend 注入 | T02 |
| `strategic_plan_proposer_usercontext_test.go` | NEW(+1 test) 覆盖 Plan proposer UserContextPrepend 注入 | T03 |

### 2.2 PR #450(DM-20260706-008)— Execute trace + CI guard

#### 2.2.1 Execute 节点 trace 验证(0 代码改动)

`workitem_executor.go:484` 早已 wired `messagesForLLMInvoke(messages, userContextPrepend)`,
早于 PR #449。trace 上 Execute 第一轮 LLM call 的 `messages_count=2` 确认 AGENTS.md 已注入
→ Execute LLM 自己完成 `d7 领域` → `internal/layers/orchestration/plan/` 的翻译,
Plan scope_in 字面路径被自愈。

Plan→Execute 帧 delta 由 `D7_Execute_PlanFrameDelta_Inject` span
(`injection_status=ok, chars=105, schema_hash=b41294769fc80a05`)和工作项执行模板合并
注入 system_prompt。`workitem_executor.go:240-257` binder 已 wired,7 个测试覆盖
(InjectPlanFrameDelta 6 测试 + Binder 1 测试)。**Execute 节点零代码改动,全协议自洽**。

#### 2.2.2 CI guard 防御回归

新增 `scripts/check-d7-d3-prepend-boundary.sh`:

```bash
#!/usr/bin/env bash
# 扫描 internal/layers/orchestration/sessionorchestrator/ 内所有
# LLMInvoker.InvokeStream / InvokeNonStream 调用点
# 验证每个调用站点所在函数前 30 行已调过 messagesForLLMInvoke
# 验证 messagesForLLMInvoke 至少被 2 处引用
# allow-list semantic_verifier_default.go (Verify 节点语义检测,设计上不需要 AGENTS.md)
set -euo pipefail

TARGET_DIR="internal/layers/orchestration/sessionorchestrator"
ALLOWLIST=("semantic_verifier_default.go")

# 1. find InvokeStream/InvokeNonStream call sites
call_sites=$(grep -rnE '\bInvokeStream\s*\(|\bInvokeNonStream\s*\(' "$TARGET_DIR" \
    --include="*.go" | grep -v _test.go || true)

# 2. for each call site, verify the surrounding function (within 30 lines above)
#    has called messagesForLLMInvoke at least once
for site in $call_sites; do
    file=$(echo "$site" | cut -d: -f1)
    line=$(echo "$site" | cut -d: -f2)
    # skip allowlist
    if printf '%s\n' "${ALLOWLIST[@]}" | grep -qxF "$(basename "$file")"; then
        continue
    fi
    # check 30 lines above the call site for messagesForLLMInvoke
    start=$((line - 30)); [ $start -lt 1 ] && start=1
    if ! sed -n "${start},$((line - 1))p" "$file" | grep -q 'messagesForLLMInvoke'; then
        echo "FAIL: $file:$line - no messagesForLLMInvoke in preceding 30 lines"
        exit 1
    fi
done

# 3. verify messagesForLLMInvoke referenced in ≥ 2 files
ref_count=$(grep -rl 'messagesForLLMInvoke' "$TARGET_DIR" --include="*.go" | wc -l | tr -d ' ')
[ "$ref_count" -ge 2 ] || { echo "FAIL: messagesForLLMInvoke only referenced in $ref_count files"; exit 1; }

echo "PASS: all InvokeStream call sites route through messagesForLLMInvoke"
```

执行结果(2026-07-06):✅ PASS,全 4 个 InvokeStream 调用点(proposer/Execute/Turn)均经过
messagesForLLMInvoke,1 个 allow-list 跳过(语义 verifier)。

### 2.3 PR #460(跨域 debt,2026-07-08)— IntentSegmenter 补 wired

PR #452(DM-20260707-001 PR-A2)引入 `LLMIntentSegmenter` 作为 D7 multi-intent observation
decompose 的 LLM-fallback proposer,但其 `Segment()` 方法直接调 `InvokeStream(req)` 绕过
`messagesForLLMInvoke`,触发 CI guard FAIL。Hotfix 修复:

#### 改动 1:SegmentRequest 加 UserContextPrepend 字段

```go
type SegmentRequest struct {
    SessionID string
    Message   string
    Prior     *learn.AdaptivePrior
    // UserContextPrepend (DM-20260706-008) is the runtime user-context
    // prepend (AGENTS.md / D{N}→path mapping) that the D2 contextengine
    // surfaces for the active session. The LLMIntentSegmenter routes its
    // messages through messagesForLLMInvoke with this map so the AGENTS.md
    // prepend reaches the LLM exactly like Observe/Plan proposers do.
    // Default nil = no prepend (legacy callers unaffected).
    UserContextPrepend map[string]string
}
```

#### 改动 2:Segment() 内部 msgs 走 messagesForLLMInvoke

```go
msgs := messagesForLLMInvoke([]types.Message{{
    SessionID: req.SessionID,
    Role:      types.MessageRoleUser,
    Content:   userPrompt,
}}, req.UserContextPrepend)
```

#### 改动 3:单元测试

`TestLLMIntentSegmenter_RoutesUserContextPrepend` 验证 stub 收到的最后一条 message
Content 含 `<system-reminder>` 包裹的 AGENTS.md 块,keys 含 `AGENTS.md` + `D7` mapping。

#### 改动 4:forward-compatible 零值

`req.UserContextPrepend == nil` 时 `messagesForLLMInvoke` 跳过 prepend 包装(已有逻辑),
legacy caller 不受影响。**纯增量**,无破坏性变更。

## 3. 全仓 LLM proposer 调用点(防御性最终清单)

| # | 调用点 | 文件:行 | 节点 | 状态 |
|---|--------|---------|------|------|
| 1 | `workitem_executor.go:485` | Execute 主路径 | ✅ 已 wired(早于 PR #449) |
| 2 | `turn_invoke.go:240` | D7 Turn sub-agent | ✅ 已 wired(早于 PR #449) |
| 3 | `llm_observation_proposer.go:55` | Observe proposer | ✅ PR #449 修复 |
| 4 | `strategic_plan_proposer.go:403` | Plan proposer | ✅ PR #449 修复 |
| 5 | `intent_segmenter.go:293` | IntentSegmenter (PR-A2 new) | ✅ PR #460 修复(跨域 debt) |
| 6 | `semantic_verifier_default.go:157` | Verify 节点语义 verifier | ✅ allow-list(无需 prepend) |

## 4. Trace 验证矩阵(2026-07-06)

| 节点 | 修复前 `messages_count` | 修复后 | 验证结果 |
|------|---------------------|--------|---------|
| Observe | 1(缺 prepend) | 2(AGENTS.md 在) | ✅ PR #449 闭环 |
| Plan | 1(缺 prepend) | 2(AGENTS.md 在) | ✅ PR #449 闭环 |
| Execute | 2(早 wired) | 2 | ✅ 自洽,无改动 |
| IntentSegmenter (PR-A2) | 1(缺 prepend) | 2(AGENTS.md 在) | ✅ PR #460 闭环 |

## 5. Failure modes & 降级

| Failure | Behaviour |
|---------|-----------|
| `req.UserContextPrepend == nil` | `messagesForLLMInvoke` 跳过 prepend 包装(纯 passthrough),legacy caller 不受影响 |
| `messagesForLLMInvoke` 引用次数 < 2 | CI guard FAIL,exit 1,merge blocked |
| Call site 前 30 行未调 `messagesForLLMInvoke` | CI guard 报告 `file:line`,exit 1 |
| `semantic_verifier_default.go` 误"修复" | CI guard allow-list 注释 + 文件头注释双向锁定 |

## 6. Relationship to PR-A2 (DM-20260707-001)

PR #452(IntentSegmenter)是 DM-20260707-001 multi-intent decompose 的 PR-A2 步骤,
它**没有读 DM-20260706-008 的 contract**(因为 DM-20260706-008 是 hotfix,
未走 S1-S5 OpenSpec 流程,文档后补)。**这是 ship-then-archive 模式的代价**:
late shipping change 的 contract 不会被同期 shipping 的 sibling change 感知。

PR #460 是契约一致性 hotfix,代表:
- **跨域 wire 一致性**:新加的 InvokeStream call site 必须走 messagesForLLMInvoke
- **CI guard 价值**:PR #452 merge 时 CI guard 立刻 FAIL,触发人工修复 PR #460
- **正向反馈**:ship-then-archive 不是无序,CI guard 充当合同警察

## 7. Verification

- 7 unit tests PASS(deliverable_execute_test.go +5 / llm_observation_proposer_test.go +1 / strategic_plan_proposer_usercontext_test.go +1)
- 1 cross-change debt test PASS(TestLLMIntentSegmenter_RoutesUserContextPrepend)
- 26/26 orchestration packages `go test -race ./...` PASS
- `bash scripts/check-d7-d3-prepend-boundary.sh` PASS(6/6 InvokeStream call sites 通过,1 allow-list)
- `go vet ./...` 0 warning
- Trace:Observe / Plan / Execute / IntentSegmenter 全部 messages_count=2(AGENTS.md 在)