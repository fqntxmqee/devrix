# Proposal: D7 MUPS v4.3 Phase 3 PR-C2 — Execute 4 Channel + ChannelRouter

**Change ID:** `devrix-d7-mups-v4-phase3-channels`
**Demand ID:** DM-20260625-001-PRC2
**Status:** S7_Archived
**Priority:** P0
**Date:** 2026-06-23
**Author:** MUPS v4.3 Phase 3 Execute 节点 4 Channel 落地梳理

> **S7 归档时间线**：
> - **S1 需求**（2026-06-23）— DM-20260625-001-PRC2 立项；需求：`demand.md`
> - **S2 提案**（2026-06-23）— 本 proposal.md + `tasks.md`；1 PR + 5 T 点 + 3 天工作量
> - **S3 设计**（2026-06-23）— `design.md` §0-§6；Channel 抽象 + ChannelRegistry 1:1 绑定 + 4 Channel 各自语义
> - **S3-Gate A-**（2026-06-23）— design.md 5/5 维度 PASS（inherited from Phase 3 execute design）
> - **S4 实现**（2026-06-23）— PR #168 (PR-C2)；7 files +1728/-0；execute package 完整实现 + 22 tests
> - **S4-Gate A-**（2026-06-23）— go vet 0 issue + go test -race 0 race + 22/22 tests PASS
> - **S5 验收**（2026-06-23）— `acceptance-report.md`；5/5 P0 AC + 5/5 设计同步 + 22/22 tests PASS；✅ ACCEPTED
> - **S6 归档**（2026-06-23）— 本 archive；spec.md v4.2.0→v4.3.0 + t-registry v3.10.0→v3.11.0 + D7-S9-A26 IMPLEMENTED + demand-archive-index.md DM-20260625-001-PRC2 行 + verify-archive.sh 12/12 PASS

---

## 1. Background

`devrix-d7-mups-v4-phase3-execute` (DM-20260625-001) 已闭环 PR-C1（落地 PR #164 + #165，4 P0 T 点 D7-S9-A25-T01..T04 IMPLEMENTED）。本 PR 是 Phase 3 第二步（PR-C2），紧随 Artifact 数据契约之后，把 Execute 节点 Channel 抽象（doc 44）+ ChannelRouter 同步为 OpenSpec 三件套 + 1 个 PR。

### 1.1 Phase 3 PR-C1 已落地的契约基础

| Phase 3 PR-C1 资产 | PR-C2 用法 |
|--------------------|-----------|
| `ArtifactKind` 4 类枚举 (orchtypes) | PR-C2 的 ChannelRouter.Route 1:1 路由产出对应 Artifact |
| `SideEffectStatus` 5 态 | PR-C2 的 4 Channel 各自映射到对应 SideEffectStatus |
| `SideEffectDetail` 5 字段 | PR-C2 的 CommitChannel 强制填 IdempotencyKey + SentAt + ConfirmedAt |
| `wavescheduler.Artifact` +5 字段 (Kind/SourcePlanID/AnomaliesCount/SideEffectStatus/SideEffectDetail) | PR-C2 的 4 Channel 全部产出 Artifact |
| 跨域类型上提 `shared/types` | PR-C2 直接消费 `types.ArtifactKind` + `types.SideEffectStatus` |

### 1.2 Phase 2 PR-B1 直接前置依赖

PR-C2 ChannelRouter 需要：
- `PlanKind` 4 类枚举（来自 PR-B1）→ 1:1 路由到 4 Channel
- `Plan.SourceObservationIDs` → Artifact.SourcePlanID 上游可追溯
- `Plan.BlastRadius.PersistScope` → ExplorationChannel 派生 SideEffectStatus

### 1.3 后续 PR 直接依赖

- PR-C3 (StrategyDecider + RetryPolicy) — 在 ChannelRouter.Route 与 Channel.Execute 之间插桩
- PR-C4 (ToolSpec v3, 10 fields) — Channel 共享 ToolRunner interface 解耦（PR-C2 用本地 ToolRunner 隔离）
- PR-C5 (ExecutionEvidence 结构化) — Channel 输出 Artifact 加 Evidence 字段
- PR-C6 (VerifyTrigger wiring) — Channel 输出 Artifact 触发 Phase 4 Verify
- PR-C7 (Executor + DispatchWorker v2) — 包装 ChannelRouter + ChannelRegistry

## 2. PR-C2 范围

### 2.1 Channel 抽象层

1. **Channel interface**
   - `Name() string` — 稳定 channel 标识
   - `Supports(plan.PlanKind) bool` — 是否支持该 PlanKind
   - `Execute(ctx context.Context, p *plan.Plan, req ChannelRequest) (*wavescheduler.Artifact, error)`

2. **ChannelRegistry**
   - `Register(ch Channel) error` — PlanKind → Channel 1:1 绑定
   - 重复 Register 同一 PlanKind → ErrChannelUnsupported
   - `Get(plan.PlanKind) (Channel, error)` — 未注册 → ErrChannelNotFound
   - 注册后只读（不可变 map）

3. **ChannelRouter** — 无状态分发
   - `Route(ctx, p *plan.Plan, req) (*wavescheduler.Artifact, error)`
   - defensive checks: nil Plan → ErrChannelPlanNil；未知 Kind → ErrChannelNotFound
   - 1:1 映射 PlanKind → Channel → 产出对应 ArtifactKind

4. **ChannelRequest** — 上下文
   - `SessionID` (string) — session 标识
   - `PriorVerdictKinds` ([]string) — Phase 4 之前的占位（typed as string，Phase 4 类型上提时收紧）

5. **ToolRunner interface** (本地) — 解耦 PR-C4
   - `Invoke(ctx, ToolRequest) (ToolResult, error)`
   - `ToolRequest`: SessionID/ToolName/Args/IdempotencyKey/StepID
   - `ToolResult`: ToolName/Output/ExitCode/StartedAt/CompletedAt

### 2.2 4 个具体 Channel

1. **CommitChannel** (CommitmentPlan → ArtifactStateChangeCert)
   - 1-Step 严格（PP-3 blast-radius guard）
   - IdempotencyKey 强制（side-effecting tools 必填）
   - 短超时（默认 5s）
   - SideEffect:
     - exitCode=0 → SideEffectCommitted
     - ctx.DeadlineExceeded → SideEffectInflight (compensate on retry)
     - non-zero exitCode → SideEffectUnknown (if non-compensable) or RolledBack (if compensable)

2. **ProtocolChannel** (ProtocolPlan → ArtifactResponseRecord)
   - 顺序多步
   - 失败 reverse-order rollback (via `__rollback: true` Args hint)
   - SideEffect:
     - 全部 success → SideEffectCommitted
     - 任意 step 失败 → 已 commit 的 steps reverse-order rollback → SideEffectRolledBack

3. **ScenarioChannel** (ScenarioPlan → ArtifactProbeReport)
   - 并行探测 MaxParallel=5
   - 多数派投票 (success > len/2 → pass)
   - SideEffectStatus=None (read-only)

4. **ExplorationChannel** (ExplorationPlan → ArtifactExperimentData)
   - 多 agent 并行 MaxParallel=3
   - 容忍部分失败 (free-fork 语义)
   - 优先级排序: success → duration → EstimatedTokens
   - PersistScope → SideEffectStatus 派生:
     - PersistTransient → SideEffectNone
     - PersistSession / PersistPermanent → SideEffectCommitted
     - PersistUnset / unknown → SideEffectUnknown

### 2.3 错误层

5 SentinelError + 4 helpers：
- `ErrChannelNotFound` (EXEC_CHANNEL_9001) — PlanKind 未注册或未知
- `ErrChannelUnsupported` (EXEC_CHANNEL_9002) — 重复 Register
- `ErrChannelStepCountMismatch` (EXEC_CHANNEL_9003) — Step 数与 Channel 不匹配
- `ErrChannelPlanNil` (—) — Plan 参数为 nil
- `ErrChannelToolRunnerNil` (EXEC_CHANNEL_9004) — ToolRunner 参数为 nil

## 3. 验收标准

### 3.1 P0 AC

| ID | 验收标准 | 状态 |
|----|---------|------|
| AC1 | ChannelRegistry 1:1 绑定 + ChannelRouter 4 PlanKind 路由 + defensive checks | ✅ PASS |
| AC2 | CommitChannel 1-Step 同步 + IdempotencyKey 强制 + 超时 SideEffectInflight | ✅ PASS |
| AC3 | ProtocolChannel 顺序多步 + reverse-order rollback | ✅ PASS |
| AC4 | ScenarioChannel 并行探测 + 多数派投票 | ✅ PASS |
| AC5 | ExplorationChannel 多 agent + 优先级排序 + PersistScope 派生 | ✅ PASS |

### 3.2 测试与质量

- 22 个测试 100% PASS（0 race detector warnings）
- 覆盖率 88.1% ≥ 80% gate
- 5 P0 T 点全部 IMPLEMENTED（D7-S9-A26-T01..T05）
- go vet ./... 0 issue

## 4. PR 拆分

| PR | 范围 | 文件数 | 估算 LOC | 风险 | 分支 |
|----|------|--------|---------|------|------|
| PR-C2 | Execute 4 Channel + ChannelRouter + 5 P0 T 点 | 7 | +1728 / -0 | Low | feat/devrix-d7-mups-v4-phase3-pr-c2 |

PR URL: https://github.com/fqntxmqee/devrix/pull/168

## 5. 不在本次任务范围

- PR-C3 StrategyDecider + RetryPolicy
- PR-C4 ToolSpec v3 (10 fields)
- PR-C5 ExecutionEvidence 结构化
- PR-C6 VerifyTrigger wiring
- PR-C7 Executor + DispatchWorker v2
- Phase 4 Verify ReverseLookup consumer
- Phase 5 Learn ExperimentData consumer

## 6. 关联

### 6.1 前置依赖

- `devrix-d7-mups-v4-phase1-foundation` (Phase 1 OpenSpec: UncertaintyCoord precedent)
- `devrix-d7-mups-v4-phase2-observe-plan` (DM-20260623-001) — 间接
- `devrix-d7-mups-v4-phase2-plan` (DM-20260623-001-PRB1) — 直接前置（PlanKind 4 类）
- `devrix-d7-mups-v4-phase3-execute` (DM-20260625-001) PR-C1 — 直接前置（Artifact 数据契约）

### 6.2 后续依赖

- `devrix-d7-mups-v4-phase3-execute` (DM-20260625-001) PR-C3..C7
- Phase 4 Verify 入口（Artifact.SourcePlanID 反向追溯 Plan.SourceObservationIDs）
- Phase 5 Learn ExperimentData 消费（ExplorationChannel 输出）

### 6.3 参考

- doc 35 §三.3 (Execute 节点方法论)
- doc 37 §2.3 (Execute 节点数据模型)
- doc 44 (D7 Execute 节点详细技术方案)
- `openspec/specs/d7-orchestration/spec.md` (D7 域规范)
- `openspec/specs/d7-orchestration/t-registry.md` (D7 T 层注册表)
