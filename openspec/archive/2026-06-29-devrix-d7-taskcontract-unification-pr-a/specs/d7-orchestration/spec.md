# D7 Orchestration Spec Delta — TaskContract 统一 PR-A (v6.0.x → v7.0.0)

**Change ID:** devrix-d7-taskcontract-unification-pr-a
**Demand ID:** DM-20260629-007
**Delta Type:** ADDED（v6.0.x → v7.0.0 演进起点 — 第一阶段 PR-A scope）
**Parent Change:** `devrix-d7-taskcontract-unification` (DM-20260629-006, S6_Archived DESIGN ONLY)
**SOT:** `internal/layers/orchestration/interfaces/`（NEW） + `mups/{execute,learn}` + `workmodel/` + `decisionplanning/`

**Scope:** L1 接口层 + L2 字段语义层 + L4 spec 同步 = 6 AC（占 23 AC 总量的 26%）。L3 防御运行时（AC11/AC12/AC13/AC14/AC15）和 L4 治理收口（AC6/AC7/AC8/AC9/AC10/AC16/AC18/AC19/AC20/AC21/AC22/AC23）由 PR-B / PR-C 实施，本文件不涉及。

---

## 1. 修改总览（PR-A scope）

| 内容 | 文件 | 类型 | 行为变化 |
|------|------|------|----------|
| 1. `interfaces.TaskSpec` struct + 4+2 字段 + builder | `interfaces/task_spec.go` (NEW) | NEW (Pure types) | L1：Plan/Channel/WorkItem 三处创建点统一通过该 type 构造 |
| 2. `interfaces.TaskReport` struct + 5+2 字段 + 5 子类型 + builder | `interfaces/task_report.go` (NEW) | NEW (Pure types) | L1：Channel.Execute 出口 + Learn 节点入口统一通过该 type |
| 3. `interfaces.DissentEntry` + `interfaces.Blockage` + `interfaces.Resource` 子类型 | `interfaces/task_report.go` (NEW) | NEW | L2：5+2 字段运行时语义（PR-A scope 子集）|
| 4. `interfaces.NewTaskSpec` / `NewTaskReport` 构造器 + `With*` 不可变 API + `AppendDissent` | `interfaces/{task_spec,task_report}.go` (NEW) | NEW | 遵循 `coding.md §9` 不可变约定 |
| 5. 5 个 ORCH_* SentinelError（PR-A 范围基础）| `interfaces/errors.go` (NEW) + `orchtypes/errors.go` (MODIFIED) | NEW | Code + Message + Remediation 三元组 |
| 6. `ChannelRequest` 新增 `Spec *TaskSpec` 嵌入字段（additive）| `mups/execute/channel.go` (MODIFIED) | L1 additive | Channel.Execute 入参强类型化（不破坏老路径）|
| 7. `LearnRequest` 新增 `Report *TaskReport` 嵌入字段（additive）| `mups/learn/learner.go` (MODIFIED) | L1 additive | Learn 节点入参强类型化（不破坏老路径）|
| 8. 全量结果 → `TaskReport.Dissent` 字段填充（ExplorationChannel top-3）| `mups/execute/exploration.go` (MODIFIED) | L2 NEW behavior | 少数派方案 + summary hash 沉淀 |
| 9. Verifier 拒绝原因 → `TaskReport.Blockage` 结构化（3 类 kind）| `decisionplanning/filter.go` (MODIFIED) | L2 NEW behavior | missing / infeasible / required_external 分类 |
| 10. ContextBudget Phase B metric → `TaskReport.Resource` 抽取 | `decisionplanning/decomposer.go` (MODIFIED) | L2 NEW behavior | per-Plan token/time/step 度量 |
| 11. `WorkItem` 创建路径返回 `*interfaces.TaskSpec` | `workmodel/workitem.go` (MODIFIED) | L1 | 三处创建点收敛（首处）|
| 12. `ChildDownlink` 嵌入 TaskSpec 引用 | `workmodel/child_downlink.go` (MODIFIED) | L1 | TraceID 全链路贯穿 |
| 13. `Decomposer` 分解产出 `*interfaces.TaskSpec` + Resource 抽取 | `decisionplanning/decomposer.go` (MODIFIED) | L1+L2 | 分解器出参强类型化 |
| 14. `SessionOrchestrator` / `RunTurn` 主循环消费 TaskSpec + 产出 TaskReport | `sessionorchestrator/{orchestrator,turn_orchestrator}.go` (MODIFIED) | L1+L2 | 5 节点全链路 |
| 15. `WaveScheduler.dispatchOne` 接收 TaskSpec | `wavescheduler/scheduler.go` (MODIFIED) | L1 | 调度入参强类型化 |
| 16. 5 个 span emit helper | `sessionorchestrator/spans.go` (MODIFIED) | L4 | d7.s20.* × 2 + d7.s21.* × 3 |
| 17. **Spec 文档同步**（6 文件）| `openspec/specs/d7-orchestration/{spec,d7-domain,a-registry,f-registry,t-registry,span-registry}.md` (MODIFIED) | L4 | spec 同步是 PR-A 提交前置 |

---

## 2. ADDED Requirements（PR-A scope 3 Requirements + 12 Scenarios）

### Requirement: TaskSpec struct + builder + 3 创建点统一 ✅ PLANNED

`internal/layers/orchestration/interfaces/` 包 MUST 定义 `TaskSpec` struct（4+2 字段），并被 Plan / Channel / WorkItem 三处创建点统一通过 `interfaces.NewTaskSpec()` 构造。

**Priority:** P0
**T:** D7-S20-A01-T01 + D7-S20-A01-T02 + D7-S20-A01-T03
**Design:** `design.md §③.1 + §⑥.2`

#### Scenario: TaskSpec construction with auto-generated TraceID

- GIVEN caller invokes `interfaces.NewTaskSpec("fix the bug")`
- WHEN construction succeeds
- THEN `TaskSpec.Goal == "fix the bug"`
- AND `TaskSpec.TraceID` matches regex `^ts_[a-f0-9]{8}$`
- AND `TaskSpec.CostBudget` defaults to zero-value
- AND `TaskSpec.HardConstraints` is empty (not nil)

#### Scenario: TaskSpec construction rejects empty Goal

- GIVEN caller invokes `interfaces.NewTaskSpec("")`
- WHEN construction runs
- THEN `interfaces.NewTaskSpec` returns `nil, interfaces.ErrTaskSpecGoalEmpty`
- AND error code is `ORCH_TASK_SPEC_GOAL_EMPTY`

#### Scenario: TaskSpec Validate all fields

- GIVEN a TaskSpec with `Goal="x"`, `HardConstraints=[{k, v, true}]`, `ConvergenceBudget={MaxDepth: 3, FallbackPolicy: Pessimistic}`, `TraceID="ts_abcdef12"`, `CostBudget={Tokens: 1000}`
- WHEN `spec.Validate()` is called
- THEN no error is returned
- AND all fields are preserved unchanged

#### Scenario: TaskSpec With* immutable builder

- GIVEN `s1 := NewTaskSpec("a"); s2 := s1.WithConstraint(Constraint{Key: "k", Value: "v", Required: true})`
- WHEN comparing
- THEN `s1.HardConstraints` is empty
- AND `s2.HardConstraints` has 1 element
- AND `s1 != s2` (different pointers)

<!-- T: D7-S20-A01-T01 -->
<!-- T: D7-S20-A01-T02 -->

#### Scenario: TaskSpec 3 创建点统一（Plan / Channel / WorkItem）

- GIVEN `mups/execute/channel.go::NewChannelRequest` is called
- WHEN request is constructed
- THEN internal call to `interfaces.NewTaskSpec(directive)` happens
- AND `request.Spec *interfaces.TaskSpec` is non-nil

- AND GIVEN `workmodel/workitem.go::NewWorkItem` is called
- WHEN workitem is constructed
- THEN internal call to `interfaces.NewTaskSpec(goal)` happens
- AND returned `WorkItem.Spec *interfaces.TaskSpec` is non-nil

- AND GIVEN `decisionplanning/decomposer.go::Decompose` is called
- WHEN task graph is synthesized
- THEN return type is `*interfaces.TaskSpec`
- AND spec.Goal equals decomposed top-level goal

<!-- T: D7-S20-A01-T03 -->

---

### Requirement: TaskReport struct + 5 子类型 + Channel.Execute 出口 + Learn 节点入口 ✅ PLANNED

`internal/layers/orchestration/interfaces/` 包 MUST 定义 `TaskReport` struct（5+2 字段 + 5 子类型），并被 Channel.Execute 出口（commit / scenario / protocol / exploration）+ Learn 节点入口（mups/learn/learner.go）统一通过 `interfaces.NewTaskReport()` 构造。

**Priority:** P0
**T:** D7-S20-A02-T01 + D7-S20-A02-T02 + D7-S20-A02-T03
**Design:** `design.md §③.2 + §⑥.2`

#### Scenario: TaskReport construction with TraceID inheritance

- GIVEN `spec := NewTaskSpec("a")` with `TraceID="ts_abcdef12"`
- WHEN `report := NewTaskReport(spec.TraceID)` is called
- THEN `report.TraceID == "ts_abcdef12"`
- AND `report.Result.Kind == ResultKindPending`
- AND `report.Dissent` is empty (not nil)
- AND `report.Blockage` is empty (not nil)
- AND `report.Resource.TokensUsed == 0`

#### Scenario: TaskReport construction rejects empty TraceID

- GIVEN caller invokes `interfaces.NewTaskReport("")`
- WHEN construction runs
- THEN `interfaces.NewTaskReport` returns `nil, interfaces.ErrTaskReportTraceIDEmpty`
- AND error code is `ORCH_TASK_REPORT_TRACE_ID_EMPTY`

#### Scenario: TaskReport With* immutable builder

- GIVEN `r1 := NewTaskReport(id); r2 := r1.WithResult(Result{Kind: Pass})`
- WHEN comparing
- THEN `r1.Result.Kind == ResultKindPending`
- AND `r2.Result.Kind == ResultKindPass`
- AND `r1 != r2` (different pointers)

<!-- T: D7-S20-A02-T01 -->
<!-- T: D7-S20-A02-T02 -->

#### Scenario: TaskReport Channel.Execute 出口统一

- GIVEN `mups/execute/commit.go::Execute` runs successfully
- WHEN returning from execution
- THEN return type is `*interfaces.TaskReport`
- AND report.TraceID matches spec.TraceID

- AND GIVEN `mups/execute/scenario.go::Execute` runs (parallel voting)
- WHEN returning from execution
- THEN return type is `*interfaces.TaskReport`
- AND report.Result.Kind reflects aggregated scenario voting

- AND GIVEN `mups/execute/protocol.go::Execute` runs (multi-step)
- WHEN returning from execution
- THEN return type is `*interfaces.TaskReport`
- AND report.Evidence contains all step outputs

#### Scenario: TaskReport Learn 节点入口统一

- GIVEN `mups/learn/learner.go::Learn` is called
- WHEN learn node consumes
- THEN parameter type is `*interfaces.TaskReport`
- AND `LearnRequest.Report *interfaces.TaskReport` is non-nil (additive 字段)
- AND `BayesianUpdate(report.TraceID, report.Result.Confidence)` is invoked

<!-- T: D7-S20-A02-T03 -->

---

### Requirement: Dissent / Blockage / Resource 字段填充规则 ✅ PLANNED

`TaskReport.Dissent` (top-3 截断) + `TaskReport.Blockage` (3 类 kind 分类) + `TaskReport.Resource` (per-Plan 度量抽取) 三字段 MUST 按既定规则填充，并在 Learn 节点 + 5 层 CB 消费。

**Priority:** P0
**T:** D7-S21-A01-T01 + D7-S21-A02-T01 + D7-S21-A03-T01
**Design:** `design.md §③.3 + §③.4 + §③.5`

#### Scenario: Dissent top-3 truncation in ExplorationChannel

- GIVEN `mups/execute/exploration.go` runs 5 candidate plans
- WHEN all candidates complete with `Result.Kind == Indeterminate`
- THEN `report.Dissent` has exactly 3 entries (top-3 minority by score)
- AND each entry has `Source`, `Decision`, `Reason`, `Summary` (hash), `Timestamp` populated
- AND 4th and 5th candidates are dropped with a log warning

#### Scenario: Dissent empty when Result.Kind == Pass

- GIVEN ExplorationChannel completes with `Result.Kind == Pass`
- WHEN report is constructed
- THEN `report.Dissent` is empty
- AND no log warning is emitted

#### Scenario: Dissent dedup at Learn 节点

- GIVEN same `DissentEntry` is `AppendDissent`-ed twice (same Source + Decision + Reason hash)
- WHEN Learn node consumes report
- THEN SkillMemory.SOP has 1 entry (deduped by hash)
- AND `report.Dissent` still has 2 entries (Learn node dedups, not interface)

<!-- T: D7-S21-A01-T01 -->

#### Scenario: Blockage 3 类 kind 分类

- GIVEN Verifier rejects with category "missing_input"
- WHEN `decisionplanning/filter.go::blockFromVerifierRejection` runs
- THEN `Blockage.Kind == BlockageMissing`
- AND `Blockage.Description` contains verifier message
- AND `Blockage.Source` is verifier ID

- AND GIVEN Verifier rejects with category "infeasible_path"
- THEN `Blockage.Kind == BlockageInfeasible`

- AND GIVEN Verifier rejects with category "required_external"
- THEN `Blockage.Kind == BlockageRequiredExternal`

<!-- T: D7-S21-A02-T01 -->

#### Scenario: Resource 字段从 ContextBudget 抽取

- GIVEN `decisionplanning/decomposer.go::resourceFromBudget(spec, sessionCtx)` runs
- WHEN ContextBudget has `TokensUsed=500`, `TokensBudget=1000`, `Elapsed=100ms`, `StepCount=3`, `ToolCalls=2`
- THEN `Resource.TokensUsed == 500`
- AND `Resource.TokensBudget == spec.CostBudget.Tokens` (1000)
- AND `Resource.TimeElapsed == 100ms`
- AND `Resource.StepCount == 3`
- AND `Resource.ToolInvocations == 2`

#### Scenario: Resource 字段单位一致性 + 不可变

- GIVEN Resource has `TokensUsed < 0` or `TimeElapsed < 0` or `StepCount < 0`
- WHEN `WithResource` is called
- THEN `interfaces.ErrResourceInvalid` is returned
- AND error code is `ORCH_RESOURCE_INVALID`

- AND GIVEN `r1 := NewTaskReport(id); r2 := r1.WithResource(res)`
- WHEN comparing
- THEN `r1.Resource == zero-value`
- AND `r2.Resource == res`
- AND `r1 != r2` (different pointers)

<!-- T: D7-S21-A03-T01 -->

---

## 3. 行为不变保证（PR-A scope）

- **ChannelRequest / LearnRequest additive 字段**：PR-A 仅新增嵌入字段 `Spec *TaskSpec` / `Report *TaskReport`，老字段全部保留 → 现有 22/22 orchestration packages 0 编译失败
- **Pure types 防 cycle**：`interfaces` 包 0 import D7 任何子包，layout guard 静态检查通过
- **22/22 orchestration packages `-race` PASS 100%**（不退化）
- **LP-1/LP-2/LP-5 100% 兼容**：TaskReport 仅作 Learn 节点入参增强，5 节点管道完整保留
- **PR-A 不做 L3 防御运行时**：AC11 Pessimistic + AC13 CoW + AC12 Rule-based + AC14 Similarity + AC15 Hard Evidence 全部留 PR-B/PR-C

---

## 4. 跨域边界影响（PR-A scope）

| 域 | 影响 | 备注 |
|----|------|------|
| D1 communication | 无变化（ProcessMessage 入口未改）| PR-A 不触 D1 |
| D2 context engine | Resource 字段从 ContextBudget Phase B 抽取 | 复用现有 metric |
| D3 LLM gateway | 无变化 | PR-A 不触 D3 |
| D4 multi-agent | Worker 接收 TaskSpec | L1 强类型化 |
| D5 observability | 5 个新 span + 4 个新 metric | L4 spec 同步（span-registry.md）|
| D6 evolution | 无变化（advisory 校验留 PR-B boundary_test）| PR-A 不触 D6 |
| D7 orchestration | 主战场（14 文件修改）| interfaces 包 + 6 D7 子包 |

---

## 5. Out of Scope（明确划线 — PR-B / PR-C 实施）

- ❌ L3 防御运行时 5 个 AC（AC11 Pessimistic + AC12 Rule-based + AC13 CoW + AC14 Similarity + AC15 Hard Evidence）
- ❌ Migration Plan（AC16 → PR-B type alias）
- ❌ Cross-Domain Boundary test（AC21 → PR-B boundary_test.go）
- ❌ Feature Flag 灰度（AC22 → PR-B）
- ❌ Error Code 完整 11 个（PR-A 仅 5 基础）
- ❌ Convergence span（AC6 → PR-C）
- ❌ AdaptiveThreshold wiring（AC7 → PR-C）
- ❌ Layout guard 静态检查实施（AC8 → PR-C）
- ❌ Coverage ≥ 80% 治理（AC18 → PR-C）
- ❌ Performance Budget 治理（AC19 → PR-C）
- ❌ Security Classification（AC20 → PR-C）
- ❌ interfaces/{hard_evidence, mvp_artifact, version_chain, dissent, blockage}.go 等子类型文件扩展
- ❌ interfaces/{benchmark, security, boundary}_test.go 测试文件

---

## 6. 关联变更

- **前置**：`devrix-d7-taskcontract-unification` (DM-20260629-006, DESIGN ONLY archive) — 父 DESIGN 648 行六段式
- **前置**：`devrix-d7-dsaft-restructuring` (DM-20260629-001) — v6.0.x 维护收官
- **前置**：`devrix-d7-six-s-simplification` (DM-20260626-001) — 14 S → 6 S
- **后续**：PR-B（DM-20260629-008 候选）落地 L3 低风险 + L4 基础
- **后续**：PR-C（DM-20260629-009 候选）落地 L3 高风险 + L4 收口

---

## 7. T 编号重映射说明

⚠️ **T 编号与父 DESIGN §2.2 不一致**：父 DESIGN 写 `D7-S16/17/18/19`，本 Change 重映射为 `D7-S20/21/22/23`（因为 `D7-S16` 已被 `devrix-d7-layer-subcontext` 占用）。详细决策见 `proposal.md §3.4 Decision 1`。