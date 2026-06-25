# Delta: Domain D7 (Orchestration)

**Change ID:** devrix-d7-orchestration-domain → current
**Affects:** orchestration, contextengine/tasks, gateway entry, query loop
**Version:** 6.0.0
**Status:** Active
**Last Updated:** 2026-06-26

---

## Current State Summary

D7 编排域 **MUPS v4.3 5 节点管道 + v5 EscapeEngine 全闭环**（2026-06-23 至 2026-06-25 落地）：物理路径与 14 S 层 1:1 对齐——S1 `workmodel/`（WorkItem 唯一权威，v4.3 起 Task flat-view 全删）+ S2 `sessionorchestrator/` + `turn/` + S3 `wavescheduler/` + S4 `executionflow/{hub,workplan,imsink,bridge}/` + S5 `decisionplanning/` + S6 横切硬化层 + S7 5 节点门面 + S8 `observe/` + S9 `execute/` + S10 `verify/` + S11 `learn/` + S12 跨域闭环集成 + S13 Verify Auto-Close + S14 `escape/`。`coordinator/` 与 `hubspoke/` 保留 1-release type-alias shim；`shared/types/` 承载跨域 Artifact 类型（PR-C1）。MUPS 5 节点管道（Observe → Plan → Execute → Verify → Learn）通过 DependencyContract 串联，LP-1 Bayesian reputation 闭环（Phase 6）已 wired 至 SessionOrchestrator。EscapeEngine + ResumeSession（Phase v5）5 层 CircuitBreaker L0..L5 + 3 决策路由 A/B/C。t-registry 180/180 IMPLEMENTED（2026-06-25 v4.9.0）。

### S 层博弈角色（切法 A — 按用户价值流）

> **基于 `devrix-d7-sa-refine` (DM-20260614-008) + MUPS v4.3 5 节点扩展**

| S 层 | 博弈角色 | North Star |
|------|---------|------------|
| D7-S1 | **State Authority**（非博弈角色） | **WorkItem** 持久化与状态机（v4.3 post-cleanup，Task flat-view 已删）；产"事实"而非"决策" |
| D7-S2 | **Screening Mechanism** + **Turn Leader (Stackelberg)** | 用户消息统一入口 + Turn 主循环；S2 = Meta-Orchestrator 跨 S3/S4/S5 |
| D7-S3 | **Mechanism Designer** | 多任务并行执行，冲突避免，上下文隔离 |
| D7-S4 | **Costly Signaler** | 执行进度透明，WorkPlan 可追溯 |
| D7-S5 | **Information Producer** | 把用户 goal 转化为可执行的任务结构 |
| D7-S6 | **Discipline Keeper**（横切硬化） | metric 命名 spec/code 对齐 + 并发安全（DM-20260622-001）|
| D7-S7 | **Pipeline Coordinator** | 5 节点管道门面 + 依赖契约编排 |
| D7-S8 | **Information Quantizer** | Observe 节点：4 类 Observation + UncertaintyReport + UncertaintyCoord（Phase 2）|
| D7-S9 | **Mechanism Designer** | Execute 节点：4 类 Artifact + 4 Channel + C2/W8 1:1 映射（Phase 3）|
| D7-S10 | **Certifier** | Verify 节点：4 态 Verdict + 14 ExitReason（Phase 4）|
| D7-S11 | **Memory Curator** | Learn 节点：4 LearningClass + ReputationEvidence Bayesian（Phase 5）|
| D7-S12 | **Closed-Loop Operator** | Observe-Learner 跨域闭环 + 3 层 fail-safe + LP-1 round-trip（Phase 6）|
| D7-S13 | **Auto Closer** | Verify Auto-Close + 4 规则 + sessionSpan 6 prior attributes（Phase 7）|
| D7-S14 | **Escape Operator** | EscapeEngine 5 层 CircuitBreaker + ResumeSession 3 决策路由 + sessionSpan 3 resume attributes（Phase v5）|

## IMPLEMENTED (现行代码)

### Requirement: D7-S3 Wave Scheduler

`WaveScheduler` MUST provide DAG scheduling with 5-slot WorkerPool, ConflictGuard, and three ContextPolicy modes.

**Package:** `internal/layers/orchestration/wavescheduler/`
**DSAFT:** D7-S3-A01 ScheduleWave, D7-S3-A02 ResolveWorkerContext, D7-S3-A03 GuardConflict

#### Scenario: Peak concurrency capped at 5

- GIVEN 6 ready subagent tasks and 1 cursor task
- WHEN WaveScheduler runs continuous dispatch
- THEN peak running workers ≤ 5
- AND cursor=1, claude_code=1, subagent=3 caps are respected

#### Scenario: Upstream context receives artifact only

- GIVEN TaskNode with `context_policy=upstream`
- WHEN ContextResolver.Resolve runs
- THEN SystemPrompt includes upstream artifact summary
- AND Messages do NOT contain Leader conversation history

#### Scenario: Fresh context is directive-only

- GIVEN TaskNode with `context_policy=fresh`
- WHEN ContextResolver.Resolve runs
- THEN Messages contain only the user directive message

---

### Requirement: D7-S4 Execution Flow Hub

`Hub` MUST implement `contracts.ExecutionFlowHub` with WorkPlan aggregation and dual-channel fan-out.

**Package:** `internal/layers/orchestration/executionflow/hub/hub.go`

#### Scenario: WorkPlan snapshot includes flows

- GIVEN FlowStarted applied to workplan.Service
- WHEN Hub.Snapshot is called
- THEN ExecutionFlows include status `running` and RecentEvents

#### Scenario: Task projection in snapshot

- GIVEN link_tasks enabled and TaskManager has session tasks
- WHEN Hub.Snapshot is called
- THEN TaskSnapshots include id, subject, status, owner

#### Scenario: IM worker progress emission

- GIVEN im_progress enabled and IMSink wired
- WHEN FlowStarted is published
- THEN GatewaySink emits worker_progress EngineEvent with render=worker_tree

---

### Requirement: D7-S1 Task Manager (已迁入 D7)

`TaskManager` MUST provide in-memory + optional disk-backed Task CRUD per session with state machine guards.

**Package:** `internal/layers/orchestration/workmodel/task_manager.go`
**DSAFT:** D7-S1-A02

#### Scenario: Disk persistence on v2 mode

- GIVEN `tasks.mode=v2` and valid store_dir
- WHEN Task is created and process restarts
- THEN TaskManager.EnsureSession reloads tasks from disk

#### Scenario: Illegal transition rejection

- GIVEN a Task in completed status
- WHEN UpdateStatus to pending is attempted
- THEN ErrIllegalTransition is returned
- AND status is unchanged

---

### Requirement: D7-S5 Plan Mode (已迁入 D7)

PlanMode MUST support `/plan` workflow with read-only PlanAgent exploration.

**Package:** `internal/layers/orchestration/workmodel/plan_mode.go`
**DSAFT:** D7-S1-A04

#### Scenario: Plan mode lifecycle

- GIVEN inactive PlanMode
- WHEN Enter is called with goal
- THEN state becomes active
- AND PlanAgent executes in read-only mode

---

## PARTIAL (实现不完整)

### Requirement: D7-S1 Unified Work Model

**状态:** v1.2 全部完成 ✅。

| 已有 | 状态 |
|------|------|
| Task CRUD + 依赖（workmodel/task_manager.go） | ✅ |
| DiskStore 持久化（workmodel/disk_store.go） | ✅ |
| Plan 实体 + CreateWorkPlan（coordinator/workmodel.go） | ✅ |
| v1.2 LocalWorkModel + BackgroundProvider | ✅ |
| TaskStatus 状态机守卫（IsLegalTransition） | ✅ |
| BackgroundTask 注册与 QueryWorkPlan 可见 | ✅ |

---

### Requirement: D7-S5 Decision Layer

**状态:** v1.1 全部完成。

| 已有 | 状态 |
|------|------|
| ClassifyIntent 规则版（classifier.go） | ✅ |
| ClassifyIntent LLM fallback（classifier_fallback.go） | ✅ |
| SynthesizeTaskGraph 规则版（decomposer.go） | ✅ |
| SynthesizeTaskGraph LLM-based 拆解（decomposer.go） | ✅ |
| SelectExecutor D2/D4 路由（executor.go） | ✅ |

---

## PLANNED (D7 v1.0 迁移)

### Requirement: D7-S2 Session Orchestrator

D1 `RouteInbound` MUST call `D7.ProcessMessage` instead of `D2.Process`.

**Review R1 补充：** OrchestratePath v1.0 不依赖 SynthesizeTaskGraph；并行 execute 委托 S3 Wave。见 `d7-requirements-clarifications.md` Routing Matrix。

#### Scenario: Entry point migration

- GIVEN `orchestration.d7_enabled=true`
- WHEN D1 Gateway RouteInbound runs
- THEN D7-S2-A01 ProcessMessage is called
- AND D2.Process is NOT called directly by D1

**Current:** `gateway.go:286` calls `g.contextEngine.Process`

---

### Requirement: D7 Migration Coexistence

Four-combination regression (`d7_enabled` × `plan.enabled`) MUST pass before defaulting `d7_enabled=true`.

See `d7-requirements-clarifications.md` §Migration Coexistence Contract. T: D7-MIG-T01.

---

### Requirement: D7 Task Model Trinity

Three task representations (PlanTask, WaveTaskNode, BackgroundRun) MUST remain separate in v1.0 with unified QueryWorkPlan. T: D7-S1-T07.

**Current:** PlanTask in `workmodel/`; Wave dispatch projection in `wavescheduler/`（WorkTree SoT，TD-WT-02 部分闭合）；Background in `workmodel/` + D2 nested

---

`query.Loop.Run` MUST be ≤200 lines with no D4/queue imports.

**Current:** ✅ IMPLEMENTED. loop.go 170 行（符合 ≤200 行目标），`LoopHooks` 结构体已删除，4 个编排字段已迁出（`PlanMode`/`TaskManager`/`Orchestration`/`Hub`），无 multiagent/queue imports。D7-THIN-T01/T02 闭环。

---

### Requirement: D7 Package Identity（HISTORICAL）

`internal/layers/orchestration/coordinator/` MUST exist as D7 coordinator sub-package.

**Current (v2.0):** `coordinator/` 降为 1-release type-alias shim（`aliases.go`）；S2/S5 实现分别在 `sessionorchestrator/`、`decisionplanning/`；共享类型在 `orchtypes/`（DM-20260619-005）。

---

## REMOVED / RETIRED

| 项 | 说明 |
|----|------|
| PEV Plan Engine | 2026-06-13 退役；由 PlanMode + TaskManager 替代 |
| ORCH 作为"纯读模型"定位 | 2026-06-13 升格为 D7 域（部分实现） |

---

## v2.0-Structure（DM-20260619-005）— IMPLEMENTED

| Phase | 内容 | 状态 |
|-------|------|------|
| A | 规格同步（design/layer-delta/d7-boundary/code-layout/a-registry） | ✅ |
| B1 | `wave/` → `wavescheduler/` | ✅ |
| B2 | S4 收敛 `executionflow/{hub,workplan,imsink}/` | ✅ |
| B3 | `coordinator` 拆 `sessionorchestrator` + `decisionplanning` + `orchtypes` | ✅ |
| B4 | `hubspoke` 拆 dispatch→S2、bridge→S4 | ✅ |
| C | WorkTree TD-WT-02/03 部分闭合 | ✅ PARTIAL |

**Shim（1 release）：** `coordinator/aliases.go`、`hubspoke/aliases.go` 重导出类型与构造函数；新代码应直接 import 目标包。

---

## HISTORICAL（PLANNED 已闭合）

以下条目在 v1.x 标为 PLANNED/PARTIAL，v2.0 Structure 后归入 HISTORICAL：

| 项 | 原状态 | 现态 |
|----|--------|------|
| D7-S2/S5 同包 `coordinator/` | PLANNED 拆分 | ✅ 已拆至 sessionorchestrator + decisionplanning |
| `wave/` / `flow/` Legacy 路径 | 迁移中 | ✅ 已迁至 wavescheduler / executionflow |
| hubspoke 整包 | 待拆分 | ✅ dispatch→S2，bridge→S4 |
| TaskNode 独立 SoT | PARTIAL | 🔶 TD-WT-02 投影化（TD-WT-01/04/05/06 defer） |

---

## Migration Checklist (D7 v1.0 — Review R1)

| Phase | 内容 | 状态 |
|-------|------|------|
| 1 / A | 域定义 + Review R1 澄清（demand/review-r1/tasks） | ✅ |
| 5 | D7-S3/S4 实现（ORCH 代码） | ✅ |
| B | D7 骨架 + contracts + re-export + bootstrap | ✅ |
| C | S5-P2 Classify + S2 ProcessMessage | ✅ |
| D | 入口切换（WireD7 + SetOrchestrationEntry） | ✅ |
| E | D2 Loop 精简（emit/executor分流，Loop.Run 196行） | ✅ |
| F | S3/S4 包路径迁移 | ✅ |
| G | 回归 + D7-MIG-T01 四组合矩阵 | ✅ |
| H (v1.1) | S5-P3 SynthesizeTaskGraph + CreateWorkPlan | ✅ DONE |
| I (v1.1) | Task 写模型迁入 coordinator | ✅ DONE |
| J (v1.1) | DiskStore 持久化迁入 D7 | ✅ DONE |
| K (v1.1) | SelectExecutor D2/D4 路由 | ✅ DONE |
| L (v1.1) | ClassifyIntent LLM fallback | ✅ DONE |
| M (v1.1) | SynthesizeTaskGraph LLM-based 拆解 | ✅ DONE |
| N (v1.2 + v2.0) | Task 状态机守卫 + 置信度阈值 + Turn Leader + LLM Invoker | ✅ DONE |

---

## IMPLEMENTED: MUPS 5 节点管道 + v5 EscapeEngine（2026-06-25 闭环）

### Requirement: D7-S8 Observe 节点

`ObserveQuantize` MUST 把用户消息 + 历史 + 上下文结构化为 4 类 Observation，产出 UncertaintyReport。

**Package:** `internal/layers/orchestration/observe/`
**DSAFT:** D7-S8-A15 ObserveQuantize

#### Scenario: ObsFact 不降级

- GIVEN 用户消息 "今天天气怎么样"
- WHEN ObserveQuantize runs
- THEN Observations 含 1 个 ObsFact（strength=★★）
- AND UncertaintyCoord.Kind=low_uncertainty

#### Scenario: ObsAnomaly 触发 Plan 升格

- GIVEN 历史会话中无用户偏好数据
- WHEN ObserveQuantize runs
- THEN Observations 含 1 个 ObsAnomaly（strength=★）
- AND Plan 应升格为 ScenarioPlan（Kind 提升）

### Requirement: D7-S9 Execute 节点

`ExecuteArtifact` MUST 按 Plan 调度执行，产出 4 类 Artifact（StateChangeCert / ResponseRecord / ProbeReport / ExperimentData）。

**Package:** `internal/layers/orchestration/execute/` + `shared/types/`（PR-C1 跨域类型上提）
**DSAFT:** D7-S9-A25 ExecuteArtifact, D7-S9-A26 RouteChannel

#### Scenario: C2/W8 1:1 映射

- GIVEN Plan.Step.Kind=state_change
- WHEN RouteChannel runs
- THEN ChannelKind=sync
- AND Artifact=StateChangeCert

#### Scenario: Artifact 跨域共享

- GIVEN ExecuteArtifact 产出 Artifact{SourcePlanID, Payload}
- WHEN Verify 节点 VerifyVerdict runs
- THEN 反向追溯 Plan 成功（SourcePlanID 在 Plan 表中可查）
- AND Artifact 跨域类型 `shared/types.Artifact` 兼容 D4/D2 SubQuery

### Requirement: D7-S10 Verify 节点

`VerifyVerdict` MUST 验证 Artifact 是否满足 Plan.FailureCriteria，产出 4 态 Verdict + 14 ExitReason。

**Package:** `internal/layers/orchestration/verify/`
**DSAFT:** D7-S10-A32..A35

#### Scenario: 14 ExitReason 映射

- GIVEN VerdictKind=ComplianceVerdict: FAIL
- WHEN VerdictToExitReason runs
- THEN ExitReason=unresolved（verify-driven，6 个之一）

- GIVEN VerdictKind=TimelinessVerdict: PASS
- WHEN VerdictToExitReason runs
- THEN ExitReason=resolved_in_window

#### Scenario: 8 deterministic ExitReason 不依赖 Verifier

- GIVEN session 因 /stop 中断
- WHEN ProcessMessage 终态
- THEN ExitReason=interrupted（D7-S2-A03 HandleInterrupt）
- AND 不经过 Verify 节点

### Requirement: D7-S11 Learn 节点

`RunLearner` MUST 把 Verdict + 追溯链沉淀为 LearningAsset + ReputationEvidence，下轮 Observe 注入 AdaptivePrior。

**Package:** `internal/layers/orchestration/learn/`
**DSAFT:** D7-S11-A36..A40

#### Scenario: Bayesian Beta 更新

- GIVEN prior Beta(5,3) + Verdict.Kind=ComplianceVerdict: PASS
- WHEN BayesianUpdate runs
- THEN posterior Beta(6,3)
- AND ReputationEvidence 持久化

### Requirement: D7-S12 Observe-Learner 跨域闭环集成

`buildObserveRequest` MUST 从 ReputationStore 加载历史 ReputationEvidence，3 层 fail-safe 注入 AdaptivePrior 到 ObserveRequest。

**Package:** `internal/layers/sessionorchestrator/observe_request.go` + `internal/layers/orchestration/observe/observe_node.go`
**DSAFT:** D7-S12-A41..A43

#### Scenario: 3 层 fail-safe（LP-1 闭环 E2E）

- GIVEN SessionID 历史无 ReputationEvidence
- WHEN buildObserveRequest runs
- THEN Layer 1 → Layer 2（DefaultDeveloperPrior Beta(5,3)）→ Layer 3（Beta(1,1) uniform 兜底）
- AND ObserveRequest 含 `prior=AdaptivePrior{Kind: developer, Alpha: 5, Beta: 3}`

### Requirement: D7-S13 Verify 自动闭环

`processAutoClose` MUST 在 ProcessRequest 终态但 Verifier 未触发时，自动 synthesizeVerdict + Auto-Close。

**Package:** `internal/layers/orchestration/verify/auto_close.go`
**DSAFT:** D7-S13-A47..A49

#### Scenario: Auto-Close R1 idle_timeout

- GIVEN last_activity > max_idle_seconds (1800s)
- WHEN ProcessRequest 进入终态
- THEN 触发 R1 规则
- AND ExitReason=auto_closed
- AND sessionSpan 含 6 prior attributes

### Requirement: D7-S14 EscapeEngine + ResumeSession

`RunEscapeEngine` MUST 触发 5 层 CircuitBreaker（L0..L5）；`ApplyResumeSession` MUST 走 3 决策路由（A/B/C）。

**Package:** `internal/layers/orchestration/escape/` + `internal/layers/sessionorchestrator/resume.go`
**DSAFT:** D7-S14-A50..A52

#### Scenario: CircuitBreaker L5 hard escape

- GIVEN 跨节点 10 次 stall
- WHEN RunEscapeEngine runs
- THEN circuit_level=L5
- AND ExitReason=aborted

#### Scenario: ResumeSession Decision B

- GIVEN circuit_level=L4 + 用户输入 `/resume accept-abort`
- WHEN ApplyResumeSession runs
- THEN ResumeDecision=user_accept
- AND ExitReason=force_exited
- AND sessionSpan 含 3 resume attributes（resume.decision=force_exited + circuit_level=L4 + user_choice）

---

## IMPLEMENTED: 6 S 博弈角色对齐精简（v6.0.0，2026-06-26 闭环）

> **v6.0.0（DM-20260626-001）6 S 精简本质是 A 活动重映射，代码路径不动**：原有 S1-S14 物理路径保留（`workmodel/` `sessionorchestrator/` `wavescheduler/` `executionflow/` `decisionplanning/` `observe/` `execute/` `verify/` `learn/` `escape/`），仅 S 编号博弈角色对齐精简。MUPS 5 节点管道（Observe/Plan/Execute/Verify/Learn）+ v5 EscapeEngine 完整保留。

### Requirement: 6 S 精简重映射

14 S → **6 S + 1 横切** 重映射为博弈角色 6 类：

| 6 S | 博弈角色 | A 数 | 物理路径 | 原 14 S 归属 |
|-----|---------|------|----------|--------------|
| **S1 WorkModel** | State Authority | 4 | `workmodel/` + `sessionorchestrator/workmodel.go` | S1 |
| **S2 SessionOrchestrator** | Mediator + Turn Leader + Error Recovery | 7 | `sessionorchestrator/` + `turn/` | S2 + S12 入口 + S13 入口 + S14 入口 |
| **S3 WaveScheduler** | Mechanism Designer | 4 | `wavescheduler/` | S3 |
| **S4 ExecutionFlow + Verify** | Costly Signaler + Certifier | 9 | `executionflow/` + `verify/` | S4 + S10 |
| **S5 DecisionPlanning + Observe** | Information Producer + Quantizer | 8 | `decisionplanning/` + `observe/` | S5 + S8 |
| **S6 MUPS Pipeline** | Pipeline Coordinator + Memory Curator | 15 | `execute/` + `learn/` | S7 + S9 + S11 + S12 E2E + S13 兜底 |
| **Cross-cutting Hardening** | Discipline Keeper | 2 | `hardening/` | S6 拆 2 A |

#### Scenario: A 活动总览

- GIVEN 14 S → 6 S + 1 横切精简
- WHEN A 活动按新 S 重归类
- THEN A 总数 56 → 49（S1:4 · S2:7 · S3:4 · S4:9 · S5:8 · S6:15 + Hardening:2）
- AND 7 Legacy A 全部并入 Canonical S（不再保留独立 Legacy 段）
- AND F 层按新 S 重归类（F 总数 75 → 68，Legacy 41 + Canonical 27）

#### Scenario: MUPS 5 节点挂载

- GIVEN MUPS 5 节点管道（Observe / Plan / Execute / Verify / Learn）
- WHEN 按 6 S 博弈角色挂载
- THEN Observe + Plan 归 **S5**（Information Producer + Quantizer）
- AND Execute + Learn 归 **S6**（Pipeline Coordinator + Memory Curator）
- AND Verify 归 **S4**（Certifier）
- AND AutoClose + Resume + EscapeEngine 入口 归 **S2**（Mediator + Error Recovery；Engine 物理独立）

#### Scenario: 5 个新 P0/P1 Span

- GIVEN v6.0.0 5 个新 Span ops
- WHEN 按 6 S 归类
- THEN `channel.route` / `memory.persist` → S6（P0）
- AND `system.anomaly_detect` → S4（P0）
- AND `taskgraph.synthesize` → S5（P1）
- AND `executor.select` → S3（P1）

**A/S 重映射 SoT：** `a-registry.md §v6.0.0 6 S 精简映射`（14 S → 6 S 完整映射表 + 6 S 完整 A 清单 49 A + 14 S 冗余合并依据 4 类）

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-13 | 初始 D7 delta（设计阶段） |
| 2.0.0 | 2026-06-14 | 对齐代码审计：IMPLEMENTED/PARTIAL/PLANNED 三分 |
| 2.1.0 | 2026-06-14 | Review R1 决议同步：路由矩阵、三模型、迁移契约 |
| 2.2.0 | 2026-06-15 | WireD7 bootstrap 实现完成；D2 Loop 瘦身待进行 |
| 2.3.0 | 2026-06-14 | Task 写模型迁入 coordinator (Phase I 完成)；CreateWorkPlan 基础版 (Phase H 进行中) |
| 2.4.0 | 2026-06-14 | SynthesizeTaskGraph 规则版实现 (D7-S5-A02)；Phase H 全部完成 |
| 2.5.0 | 2026-06-14 | v1.1 全部完成：DiskStore 持久化、SelectExecutor、LLM fallback、LLM-based 拆解 |
| 2.6.0 | 2026-06-15 | DM-020 Turn Leader wired + Hub-Spoke SoT + D2 Thin 进行中 |
| 3.0.0 | 2026-06-16 | **v1.2 + v2.0-b/c/f 全部闭环** |
| 4.0.0 | 2026-06-19 | v2.0 Structure（DM-20260619-005）：物理路径对齐 S 层；coordinator/hubspoke shim；WorkTree TD-WT-02/03 部分闭合 |
| **5.0.0** | **2026-06-25** | **MUPS v4.3 5 节点管道 + v5 EscapeEngine 全闭环（DM-20260623-001/002/003 + DM-20260624-001 + DM-20260625-001/003/004）**：14 S 层（D7-S1~S14）全部 IMPLEMENTED。新增 6 段 IMPLEMENTED requirements：D7-S8 Observe（D7-S8-A15 ObserveQuantize + 4 类 Observation + UncertaintyReport + UncertaintyCoord）+ D7-S9 Execute（D7-S9-A25/A26 4 Artifact + 4 Channel + 跨域类型上提 shared/types）+ D7-S10 Verify（D7-S10-A32..A35 4 态 Verdict + 14 ExitReason）+ D7-S11 Learn（D7-S11-A36..A40 Bayesian Beta 更新）+ D7-S12 Observe-Learner 跨域闭环（LP-1 round-trip + 3 层 fail-safe）+ D7-S13 Verify Auto-Close（4 规则 + sessionSpan 6 prior attributes）+ D7-S14 EscapeEngine（5 层 CircuitBreaker L0..L5 + ResumeSession 3 决策路由 A/B/C + sessionSpan 3 resume attributes）。t-registry 66 → 180 |
| **6.0.0** | **2026-06-26** | **6 S 博弈角色对齐精简（DM-20260626-001）**：14 S → **6 S + 1 横切** IMPLEMENTED（State Authority / Mediator+Turn Leader+Error Recovery / Mechanism Designer / Costly Signaler+Certifier / Information Producer+Quantizer / Pipeline Coordinator+Memory Curator / 横切 Discipline Keeper）；A 活动 **56 → 49**（S1:4 · S2:7 · S3:4 · S4:9 · S5:8 · S6:15 + Hardening:2）；F 层按新 S 重归类（F 总数 75 → 68，Legacy 41 + Canonical 27）；MUPS 5 节点挂载：Observe+Plan 归 S5，Execute+Learn 归 S6，Verify 归 S4，AutoClose+Resume+Escape入口 归 S2；7 Legacy A 全部并入 Canonical；t-registry 180 → 186（5 个新 P0/P1 span T 点）。详细 A/S 重映射见 `a-registry.md §v6.0.0 6 S 精简映射` + `f-registry.md` + `t-registry.md` + `span-registry.md §Operations`（v6.0.0 已落地 23 ops + 9 sessionSpan attributes）|

---

## Docs Sync（2026-06-16）

领域规格同步（`openspec/specs/d7-orchestration/`，无代码变更）：

| 新增/更新 | 文件 | 说明 |
|----------|------|------|
| REWRITE | `d7-domain.md` v1.0.0 | 薄领域 SoT（对齐 D1 `*-domain.md` 模式） |
| ADDED | `terminal-state-guide.md` | IntentKind 四链、A→F 编排树、跨域时序 |
| RENAMED | `d7-requirements-clarifications.md` | 原厚 `d7-domain.md`（Review R1/R2 + 历史 Gherkin） |
| UPDATED | `spec.md` v3.1.0 | Domain SoT 指针；域边界 LLM 产权修正 |
| UPDATED | `a-registry.md` v3.5.0 | Canonical 段补登 S1；24 A 统计 |
| UPDATED | `design.md` / `t-registry.md` / `d3-boundary.md` | 文档索引交叉引用 |
| UPDATED | `dsaft-architecture.md` | 收敛为 Stub v2.0.0 |
| ADDED | `observability-guide.md` | Span↔T、Trace 树、Hub Flow、P0 Runbook |
| UPDATED | `span-registry.md` | Guides 指针 |
