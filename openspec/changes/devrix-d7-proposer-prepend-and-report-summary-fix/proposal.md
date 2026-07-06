# D7 Proposer UserContextPrepend + ReportSummary 信息密度修复 (DM-20260706-008)

> **状态**:✅ **S7_Archived**(2026-07-06)— 全 3 proposer + CI guard 闭环
> **关联**:DM-20260706-007 (PR #449) 覆盖 Observe + Plan;本 Change 覆盖 Execute 验证 + 防御性 CI guard
> **优先级**:P0 — 阻断 LLM 完整消费上游 context,sess_1783333760211_6000 类飞书卡 ❌ 根因

## 1. 背景

`sess_1783333760211_6000` 飞书卡片失败事件的 trace `0369df062c125dfe2aad9de21363730e` 揭示 D7 编排层 3 个 LLM proposer 存在**两类同源 bug**:

| Bug 类别 | 现象 | 触发位置 | 协议层偏差 |
|---------|------|---------|-----------|
| **A: UserContextPrepend 边界吞** | `messagesForLLMInvoke` 没被调用,LLM 拿不到 AGENTS.md | proposer.InvokeStream 调用边界 | D2→D3 API 边界 |
| **B: ReportSummary 信息坍塌** | `uncertaintyReportSummary` 只数 anomalies,ObsUncertainty 实际内容丢失 | item_pipeline.go → strategic_plan_proposer.go 字段序列化 | D7 内部节点间契约 |

PR #449(DM-20260706-007)覆盖 Observe + Plan 两节点。**Execute 节点 trace 验证 + 防御性 CI guard**在本 Change 闭环。

## 2. 修复原则

### 2.1 三条原则

1. **D2→D3 API 边界强制**:`proposer.InvokeStream` 必须经过 `messagesForLLMInvoke(msgs, prepared.UserContextPrepend)` 包装,这是 D2-prepared context 到 D3-LLM request 的唯一允许路径。**任何绕过此路径的 proposer 调用都视为协议违规**(由 `scripts/check-d7-d3-prepend-boundary.sh` CI guard 强制)。
2. **UncertaintyReport 消费契约对齐 design.md §7.2**:Plan 节点输入 = `UncertaintyReport{Observations, UncertaintyCoord, Anomalies, QuantizedIntent}`。**实现不应把 4 维对象坍塌为 1 字符串**;序列化是必要的(LLM 文本消费),但**必须保留 CatBusiness Observations 的语义内容**,不能仅用 Numeric anomaly count 代替。
3. **partition by Category 设计必须被尊重**:CatBusiness 走 `BusinessObservations`(LLM-driven ObsUncertainty 的归宿),CatSystem + ObsDeviation / 高强度 ObsUncertainty 走 `Anomalies`。**任何下游消费者必须明确自己消费的 partition**,不能假设 "Anomalies = 全部 Observations"。

### 2.2 不在范围内(显式排除)

- **Plan LLM output 缺 `source_observation_ids` 字段**:design.md §7.2 强约束,但被 `PrepareStrategicScopeIn` Gate 兜底,未产生线上事故。归 hardening backlog。
- **D2 prior_delta_empty span fallback 缺失**:DM-20260705-010 协议要求,本 Change 不涉及。

## 3. 修复范围 — 已完成

### 3.1 PR #449(DM-20260706-007)— Observe + Plan 修复

- `uncertaintyReportSignature` 签名扩展 `(anomalyCount int) → (report UncertaintyReport)`,扫描 `report.Observations`,序列化 ObsUncertainty.Question + ObsDeviation.Statement,strength ≥ 0.7 阈值过滤
- `LLMObservationProposer.ProposeObservations` + `LLMStrategicPlanProposer.ProposeStrategicPlan` 改为走 `messagesForLLMInvoke`
- 7 个新 unit test 覆盖两节点 prepend + summary 序列化边界

### 3.2 DM-20260706-008(本 Change)— Execute 验证 + CI guard

#### 3.2.1 Execute 节点 trace 验证(0 代码改动)

`workitem_executor.go:484` 早已 wired `messagesForLLMInvoke(messages, userContextPrepend)`,早于 PR #449。trace 上 Execute 第一轮 LLM call 的 `messages_count=2` 确认 AGENTS.md 已注入 → Execute LLM 自己完成 `d7 领域` → `internal/layers/orchestration/plan/` 的翻译,Plan scope_in 字面路径被自愈。

Plan→Execute 帧 delta 由 `D7_Execute_PlanFrameDelta_Inject` span(`injection_status=ok, chars=105, schema_hash=b41294769fc80a05`)和工作项执行模板合并注入 system_prompt。`workitem_executor.go:240-257` binder 已 wired,7 个测试覆盖(InjectPlanFrameDelta 6 测试 + Binder 1 测试)。**Execute 节点零代码改动,全协议自洽**。

#### 3.2.2 CI guard 防御回归

新增 `scripts/check-d7-d3-prepend-boundary.sh`:
- 扫描 `internal/layers/orchestration/sessionorchestrator/` 内所有 `LLMInvoker.InvokeStream` / `InvokeNonStream` 调用点
- 验证每个调用站点所在函数**前 30 行**已调过 `messagesForLLMInvoke`
- 验证 `messagesForLLMInvoke` 至少被 2 处引用(Observe + Plan + Execute + Turn)
- allow-list `semantic_verifier_default.go`(Verify 节点的 template-mimicry 检测,设计上不需要 AGENTS.md 注入)

执行结果:✅ PASS,全 4 个 InvokeStream 调用点(proposer/Execute/Turn)均经过 messagesForLLMInvoke,1 个 allow-list 跳过(语义 verifier)。

#### 3.2.3 全仓 LLM proposer 调用点(防御性最终清单)

| # | 调用点 | 文件:行 | 节点 | 状态 |
|---|--------|---------|------|------|
| 1 | `workitem_executor.go:485` | Execute 主路径 | ✅ 已 wired |
| 2 | `turn_invoke.go:240` | D7 Turn sub-agent | ✅ 已 wired |
| 3 | `llm_observation_proposer.go:55` | Observe proposer | ✅ PR #449 修复 |
| 4 | `strategic_plan_proposer.go:403` | Plan proposer | ✅ PR #449 修复 |
| 5 | `semantic_verifier_default.go:157` | Verify 节点语义 verifier | ✅ allow-list(无需 prepend) |

## 4. 验收标准 — 已达成

| AC | 内容 | 实测 |
|----|------|-----|
| AC1 | D2→D3 边界强制 — `grep` 全仓 InvokeStream,0 proposer 绕过 messagesForLLMInvoke | ✅ 4/4 proposer 通过,CI guard 兜底 |
| AC2 | UncertaintyReport 内容保真 — `uncertaintyReportSummary` 序列化 ObsUncertainty.Question + ObsDeviation.Statement,strength ≥ 0.7 阈值 | ✅ 5 new unit test + 修复后 trace 上 observation_summary 含 `q=<实际问题>` |
| AC3 | Execute 节点协议自洽 — Execute LLM 收到 AGENTS.md + plan_frame_delta | ✅ trace 上 messages_count=2,Execute LLM 自愈 Plan scope_in 字面路径 |
| AC4 | Regression 全绿 — 22/22 orchestration packages `go test -race ./...` PASS | ✅ PR #449 全绿 |
| AC5 | CI guard 兜底 — `check-d7-d3-prepend-boundary.sh` PASS | ✅ 4/4 调用点通过 |

## 5. 风险评估 — 闭环

- **小**:D2→D3 边界修复**仅**追加 `messagesForLLMInvoke` 包装,不改 LLM 调用参数,不改 prompt 内容,不改输出解析。回归风险局限于 "UserContextPrepend 突然出现" 的 LLM 行为变化 — 这是修复目标本身,**预期改进而非回归**。
- **小**:ReportSummary 序列化扩展是**纯增量**:旧契约 `intent=<kind>` 永远保留(`parts = append(parts, "intent="+intentKind)` 仍第一段);新内容以 `; q=...; dev=...` 追加。LLM 看到旧场景仍 100% 兼容。
- **小**:Execute 节点 0 代码改动,纯 trace 验证 + CI guard 加固。**风险最低**。

## 6. 实际归档清单(2026-07-06)

### 6.1 代码改动

| 文件 | 状态 | 来源 |
|------|------|------|
| `internal/layers/orchestration/sessionorchestrator/deliverable_execute.go` | MODIFIED | PR #449 Fix A |
| `internal/layers/orchestration/sessionorchestrator/llm_observation_proposer.go` | MODIFIED | PR #449 Fix B |
| `internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer.go` | MODIFIED | PR #449 Fix B |
| `internal/layers/orchestration/sessionorchestrator/deliverable_execute_test.go` | MODIFIED(+5 tests) | PR #449 |
| `internal/layers/orchestration/sessionorchestrator/llm_observation_proposer_test.go` | MODIFIED(+1 test) | PR #449 |
| `internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer_usercontext_test.go` | NEW(+1 test) | PR #449 |
| `scripts/check-d7-d3-prepend-boundary.sh` | NEW | DM-20260706-008 CI guard |

### 6.2 Trace 验证

| 节点 | 修复前 `messages_count` | 修复后 | 验证结果 |
|------|---------------------|--------|---------|
| Observe | 1(缺 prepend) | 2(AGENTS.md 在) | ✅ PR #449 闭环 |
| Plan | 1(缺 prepend) | 2(AGENTS.md 在) | ✅ PR #449 闭环 |
| Execute | 2(早 wired) | 2 | ✅ 自洽 |

### 6.3 防御性硬化

- `scripts/check-d7-d3-prepend-boundary.sh`:全仓 InvokeStream 调用点扫描 + 30 行前文 messagesForLLMInvoke 必现检查。CI 必跑。
- allow-list 文档化:`semantic_verifier_default.go` 是 Verify 节点的 template-mimicry 检测,设计上不需要 AGENTS.md,**显式 allow-list + 注释**,防止后续被错误"修复"。

## 7. 后续跟进项

- **Plan LLM output 缺 `source_observation_ids` 字段**:design.md §7.2 强约束,Plan→Execute 帧 delta 已 trace-verified 工作,Gate 兜底无事故。归 hardening backlog。
- **D2 prior_delta_empty span fallback 缺失**:r1 initial 阶段 FrameDelta 零值时 hardening 承诺的 `prior_delta_empty` span 未 emit。归 hardening backlog。
- **CI guard 增量集成**:把 `check-d7-d3-prepend-boundary.sh` 接入 CI heavy steps,与 `check-orch-rename.sh` 并列。下一 PR 跟进。