---
demand-id: DM-20260625-001
title: D7 MUPS v4.3 Phase 3 — Execute 节点落地（7 PR 联动）
priority: P0
status: S1_Proposal
dsaft_domain: orchestration
created: 2026-06-23
---

# D7 MUPS v4.3 Phase 3 — Execute 节点落地

## 1. 背景

Phase 1（DM-20260623-001，UncertaintyCoord scaffold）与 Phase 2（DM-20260624-001，Observe + Plan 节点）已 S7_Archived 落地。Phase 3 是 5 Phase 落地的第三步，把 [[../../../brain/01知识探索/项目/20260620-certain-architecture/project-application/44-d7-execute-node-design|doc 44 Execute 节点]] 设计稿同步为 OpenSpec 五件套（含 d7-orchestration + tool-surface 两个 spec delta），把当前 WaveScheduler 单通道执行升级为 4 类 PlanKind → 4 类 Channel + StrategyDecider + RetryPolicy + ToolSpec v3 + ExecutionEvidence 的完整 Execute 节点。

### 1.1 Phase 2 已落地的契约基础（Phase 3 直接消费）

| Phase 2 资产 | Phase 3 用法 |
|------------|------------|
| `plan.Plan` 含 4 类 PlanKind + Steps + FailureCriteria + BlastRadius + SourceObservationIDs + AnomaliesCount | Executor.Execute(Plan, sessionID) 输入 |
| `plan.PlanStep` 含 ToolName + Parameters + IdempotencyKey | Execute 调用 ToolSurface 时的关键参数 |
| `plan.BlastRadius` | Channel 选择 + 并发数决策 |
| `AnomaliesCount` | Verdict 反向追溯 Phase 5 Learn 路径（Phase 3 只透传）|

### 1.2 与已有 change 的关系

| 已有 change | 关系 |
|------------|------|
| `devrix-d7-mups-v4-phase1-foundation` (DM-20260623-001) | 直接前置：UncertaintyCoord scaffold |
| `devrix-d7-mups-v4-phase2-observe-plan` (DM-20260624-001) | **直接前置**：Plan 类型 + 3 项强制约束 |
| `devrix-tool-surface-contract` (DM-20260617-007) | **直接前置**：ToolSurface 4 方法接口 + ToolFilter 链 |
| `devrix-tool-surface-phase2-full` (DM-20260617-008) | 直接前置：12→0 global loop 闭环 |
| `devrix-tool-surface-3changes` (DM-20260618-001/002/003) | 直接前置：ToolSpec v2 + CheckPermission + DeferLoading |
| `devrix-d7-v2-structure` (DM-20260619-005) | 间接前置：v2.0 物理路径 |
| `devrix-d7-metrics-and-concurrency-hardening` (DM-20260622-001) | 间接前置：D5 Span 可复用 |
| `devrix-d7-mups-v4-phase4-verify-promotion` (DM-20260626-001 候选) | **被前置**：Phase 4 Verifier 消费 Artifact |
| `devrix-d7-mups-v4-phase5-learn` (DM-20260627-001 候选) | 间接被前置：Learn 消费 Verdict → SourceArtifactID → Plan |

## 2. 问题陈述

### Problem 1 (HIGH): Execute 节点无统一抽象，4 类执行通道散落

**位置**：
- `internal/layers/orchestration/wavescheduler/scheduler.go` 有 WaveScheduler 但只支持单通道（并行 max=5）
- `internal/layers/orchestration/sessionorchestrator/dispatch.go` 是 DispatchWorker 现有实现，零散耦合
- **没有 PlanKind → 4 类通道的路由**：CommitPlan/ProtocolPlan/ScenarioPlan/ExplorationPlan 都走同一条 WaveScheduler 路径

**根因**：v2.0 结构重构时只切分物理路径，未抽象 Execute 节点 + 4 类通道。

**影响**：
- Execute 无法做 PlanKind 决策（commit/protocol/scenario/exploration 应走不同策略）
- Tool 调用契约无统一入口，分散在 wavescheduler + sessionorchestrator + delegatetools
- 失败重试与降级策略只能 hardcode 在 WaveScheduler，无法 per-Plan 配置

### Problem 2 (HIGH): Artifact 不可分类，4 类执行产物混用

**位置**：`internal/layers/orchestration/wavescheduler/artifact.go` 的 `Artifact` 类型。

**根因**：当前 `Artifact` 只有 `Kind` (4 string) + `Payload any` 弱类型，无法区分 StateChangeCert / ResponseRecord / ProbeReport / ExperimentData。

**影响**：
- Verify 节点（Phase 4）无法用 Artifact 类型路由到对应 Verifier（StateChange → Compliance；ProbeReport → Statistical；ExperimentData → RootCause）
- Learn 节点（Phase 5）无法聚合跨会话的 StateChangeCert（重操作去重）vs ExperimentData（实验复盘）

### Problem 3 (MEDIUM): ToolSpec v2 缺补偿/降级/资源元数据

**位置**：`internal/layers/orchestration/toolrunner/surface/spec.go` 当前 4 字段（Name/Description/Parameters/RiskLevel）。

**根因**：v2 阶段只满足 D7-S5 路由决策需求，未覆盖 Execute 节点的失败恢复 + 资源调度需求。

**影响**：
- Channel 失败时无法判断 IsCompensable → 无法自动 rollback
- 重试无 IdempotencyKey 支持 → 副作用类操作重试有数据风险
- 无 Fallback 字段 → 无降级路径

### Problem 4 (MEDIUM): Tool 调用证据无统一机器可解析结构

**位置**：当前 Tool 调用结果散落在 Worker 日志、WorkPlan 内存、FlowEvent 流中。

**根因**：v2 阶段只关注 WorkPlan 读模型 + FlowEvent 流，未定义"单次 Tool 调用完整证据包"。

**影响**：
- Verify 节点（Phase 4）无法回放 Tool 调用 → 无 Verifier 训练数据
- Learn 节点（Phase 5）无法分析"什么 Tool 组合失败率高" → 无 SOP 沉淀

## 3. 验收标准

### 3.1 核心 AC（PR-C1 Artifact 升级）

| ID | 标准 | 优先级 |
|----|------|--------|
| **AC1** | `ArtifactKind` 枚举 4 类：`StateChangeCert` / `ResponseRecord` / `ProbeReport` / `ExperimentData`（强类型 + 字符串 alias 双向转换）| P0 |
| **AC2** | `SideEffectStatus` 枚举 4 类：`SideEffectNone` / `SideEffectInflight` / `SideEffectConfirmed` / `SideEffectRolledBack` | P0 |
| **AC3** | `wavescheduler/artifact.go::Artifact` 升级：新增 `ArtifactKind` + `SideEffectStatus` + `SourcePlanID`（反向追溯 Phase 2 Plan），向后兼容（v2 Artifact 调用方零修改） | P0 |
| **AC4** | `orchtypes/artifact_kind_test.go` + `side_effect_status_test.go` 单测覆盖 4×4 组合 + 边界（空字符串、未知值、JSON 双向） | P0 |

### 3.2 4 类执行通道（PR-C2）

| ID | 标准 | 优先级 |
|----|------|--------|
| **AC5** | 新建 `internal/layers/orchestration/execute/channel.go` 定义 `Channel` interface（Name / Supports(planKind) / Execute(ctx, plan, req) (*Artifact, error)）| P0 |
| **AC6** | 4 个 Channel 实现：`CommitChannel`（1 Step 直执行）/ `ProtocolChannel`（有序串行）/ `ScenarioChannel`（受控并行 ≤5）/ `ExplorationChannel`（最大并行 3 + freefork 可选）| P0 |
| **AC7** | `Executor.Execute(plan)` 内部按 `plan.PlanKind` 路由到对应 Channel，未匹配返回 `ErrUnsupportedPlanKind` | P0 |
| **AC8** | 4 Channel 单测 + `tests/integration/d7/execute_channels_test.go` 集成测试覆盖 4 路径（commit/protocol/scenario/exploration）| P0 |

### 3.3 StrategyDecider + RetryPolicy（PR-C3）

| ID | 标准 | 优先级 |
|----|------|--------|
| **AC9** | `Strategy` 枚举 4 类：`StrategyContinue` / `StrategyAskAtRoundEnd` / `StrategyAskNow` / `StrategyAskAndRollback` | P0 |
| **AC10** | `DefaultStrategyDecider` MVP 实现 L0 硬规则（SideEffectInflight → AskNow；FailureCount ≥ 3 → AskAtRoundEnd；else Continue），L1 LLM 决策为 stub | P0 |
| **AC11** | L1 LLM SystemPrompt + CriticalReminder 强约束："你是 StrategyDecider... **不要调用任何 tool**" | P0 |
| **AC12** | `RetryPolicy` 升级 5 字段：`MaxRetries` / `BackoffStrategy`（exponential/linear/fixed）/ `InitialDelayMs` / `MaxDelayMs` / `UseIdempotencyKey` | P0 |
| **AC13** | `turn/orchestrator.go::ExitReason` 扩展：`unresolved` / `abstain` / `verifier_abstain` 3 态（与 doc 18 L1 ExitReason 扩展对齐）| P0 |

### 3.4 ToolSpec v3 扩展（PR-C4）

| ID | 标准 | 优先级 |
|----|------|--------|
| **AC14** | `ToolSpec` 从 4 字段扩展到 14 字段：新增 `IsAsync` / `IsIdempotent` / `IsRetryable` / `IsCompensable`（4 Capability）+ `CompensationTool` / `CompensationArgs` / `CompensationTimeoutMs`（3 补偿契约）+ `MaxRetries` / `BackoffStrategy` / `TimeoutMs`（3 重试资源）+ `FallbackTool` / `FallbackArgs`（2 降级路径）| P0 |
| **AC15** | `ToolSpec.IsSideEffect() bool` 方法（基于 `IsCompensable` 推导：`!IsCompensable && RiskLevel >= medium` 视为有副作用）| P0 |
| **AC16** | `devrix.yaml` + `orchtypes/config.go` 新增 `d7.execute.*` 配置（retry / circuit_breaker / channels）| P0 |
| **AC17** | YAML → ToolSpec 加载路径 `toolrunner/surface/spec_loader.go` 同步扩展，YAML 字段缺失走默认值（不破坏现有 YAML）| P0 |

### 3.5 ExecutionEvidence + VerifyTrigger（PR-C5+PR-C6）

| ID | 标准 | 优先级 |
|----|------|--------|
| **AC18** | `ExecutionEvidence` 结构体：`ToolInvocations []ToolInvocation` + `Logs []LogEntry` + `Metrics MetricSnapshot`（JSON 双向）| P0 |
| **AC19** | `ToolInvocation` 9 字段：`ToolName` / `Args` / `IdempotencyKey` / `ExitCode` / `Stdout|Stderr` / `StartedAt|CompletedAt` / `RetryCount` | P0 |
| **AC20** | `Artifact.Evidence *ExecutionEvidence` 字段（PointTo ExecutionEvidence，可 nil）| P0 |
| **AC21** | Wave 完成时 `sessionorchestrator/orchestrator.go` 调 `VerifyTrigger`（Phase 4 占位 interface），Artifact + Plan 反向追溯链组装后 emit `FlowEvent` | P0 |

### 3.6 Executor interface + DispatchWorker v2（PR-C7）

| ID | 标准 | 优先级 |
|----|------|--------|
| **AC22** | `Executor` interface：`Execute(ctx, plan, sessionID) (*Artifact, error)` | P0 |
| **AC23** | `DefaultExecutor` 实现：PlanKind → Channel 路由 + RetryPolicy 应用 + StrategyDecider 调用（失败时）+ ExecutionEvidence 组装 | P0 |
| **AC24** | `sessionorchestrator/dispatch.go::DispatchWorker` v2 升级：调用 `DefaultExecutor.Execute` 替换原 wavescheduler 直调 | P0 |
| **AC25** | 旧 `wavescheduler.Scheduler.Dispatch()` 路径保留为 shim（标记 `Deprecated:`），D5 metric `dispatch_legacy` 监控 | P0 |

### 3.7 测试 + 治理

| ID | 标准 | 优先级 |
|----|------|--------|
| **AC26** | 7 个 PR 全部 `go vet` 0 issue + `go test -race ./internal/layers/orchestration/...` 全绿 | P0 |
| **AC27** | 新增 P0 T 点 ≥ 21 个（D7 域 ≥ 17 + tool-surface 域 ≥ 4），全部 IMPLEMENTED | P0 |
| **AC28** | 覆盖率 ≥ 72.2%（持平 baseline），无 regression | P0 |
| **AC29** | D5 spans P95 不退化（Execute 关键路径新增 ≤ 5 个 span）| P0 |
| **AC30** | live spec.md v4.1.0 → v4.2.0（新增 7 PR 对应 Requirement 块），t-registry v3.10.0 → v3.13.0 | P0 |
| **AC31** | docs/context-budget.md / docs/tool-surface-contract.md 同步（如有新增 cross-domain 行为）| P1 |
| **AC32** | 文档：proposal.md / design.md / tasks.md / spec_delta.md / t-registry_delta.md 五件套 + .openspec.yaml + acceptance-report.md 全闭环 | P0 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| **依赖** | Phase 1 + Phase 2 资产（`plan.Plan` / `plan.PlanStep` / `plan.BlastRadius` / `AnomaliesCount`）|
| **依赖** | `devrix-tool-surface-contract` (DM-20260617-007) ToolSurface 4 方法接口 |
| **依赖** | `devrix-tool-surface-3changes` (DM-20260618-001/002/003) ToolSpec v2 |
| **依赖** | `internal/shared/errors/` SentinelError 模式 |
| **依赖** | `orchtypes/intent.go::IntentKind` 枚举（fast/command/orchestrate/skip）|
| **约束** | 不修改 D5/D4/D2 域规范文件（除 spec.md cross-reference）|
| **约束** | 不动 `decisionplanning.Decomposer`（Phase 4 后退役，本期保留共存期）|
| **约束** | 7 PR 顺序落地：PR-C1 → PR-C2 → PR-C3 → PR-C4 → PR-C5 → PR-C6 → PR-C7，每 PR 单独 verify-archive check |
| **约束** | WaveScheduler 旧路径保留为 shim（AC25），不在本期删除 |

## 5. 风险评估

| 风险 | 等级 | 缓解 |
|------|------|------|
| 7 PR 合并冲突 | HIGH | 每个 PR 独立 base 在 master，按顺序 squash merge；`DecisionPlanning.Decomposer` 保留共存期避免 break |
| ToolSpec v3 YAML 兼容性 | MEDIUM | AC17 强制走默认值；devrix.yaml + config_test 覆盖所有新字段 |
| Channel.Execute 错误传播 | MEDIUM | 4 Channel 统一返回 `(*Artifact, error)`，`error` 走 SentinelError |
| L1 LLM Strategy 决策延迟 | MEDIUM | MVP 仅 L0 硬规则，L1 留 stub（AC10 末尾注），Phase 5 Learn 闭环时再启用 L1 |
| DispatchWorker v2 兼容旧调用 | LOW | 旧路径保留 shim + D5 metric `dispatch_legacy` 监控（AC25）|

## 6. 工作量估算

| PR | 范围 | 文件数 | LoC | 工作量 |
|----|------|--------|------|--------|
| PR-C1 | Artifact 升级 | 4 新 + 1 改 + 2 测试 | ~600 | 1.5 天 |
| PR-C2 | 4 Channel | 9 新 + 1 集成 | ~1100 | 2.5 天 |
| PR-C3 | StrategyDecider + RetryPolicy | 4 新 + 2 改 | ~700 | 1.5 天 |
| PR-C4 | ToolSpec v3 | 2 改 + 1 配置 + 2 测试 | ~800 | 2 天 |
| PR-C5 | ExecutionEvidence | 2 新 + 1 改 | ~400 | 1 天 |
| PR-C6 | VerifyTrigger wiring | 1 改 | ~200 | 0.5 天 |
| PR-C7 | Executor + DispatchWorker v2 | 3 新 + 1 改 | ~600 | 1.5 天 |
| 文档同步 | spec / t-registry / docs | 5 改 | ~500 | 0.5 天 |
| **总计** | — | — | ~4900 | **11 天** |

## 7. 落地顺序

1. **PR-C1** Artifact 升级 → 最小风险入口
2. **PR-C2** 4 Channel → 路由前置
3. **PR-C3** StrategyDecider + RetryPolicy → 失败恢复策略
4. **PR-C4** ToolSpec v3 → Tool 元数据升级（PR-C2/C3 内部 Channel 决策要读 IsCompensable）
5. **PR-C5** ExecutionEvidence → Verify 节点（Phase 4）数据源
6. **PR-C6** VerifyTrigger wiring → 跨节点 wiring（为 Phase 4 留 hook）
7. **PR-C7** Executor + DispatchWorker v2 → 终点，串联所有 PR

每 PR 单独走 S1-S6 子流程（PR-C1 ~ PR-C7 各 1 个 squash PR），全程累计 7 PR + 1 文档同步 PR。
