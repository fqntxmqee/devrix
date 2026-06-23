# Tasks: D7 MUPS v4.3 Phase 3 PR-C2 — Execute 4 Channel + ChannelRouter

**Change ID:** `devrix-d7-mups-v4-phase3-channels`
**Demand ID:** DM-20260625-001-PRC2
**Status:** S2_Proposal → S4_Implemented → S7_Archived
**Date:** 2026-06-23
**Author:** MUPS v4.3 Phase 3 Execute 节点落地梳理

---

## PR-C2: Execute 4 Channel + ChannelRouter + 5 P0 T 点

> 目标：把 doc 44 (D7 Execute 节点) Channel 抽象 + ChannelRouter 设计稿同步为 Go 代码 + 测试 + spec 落地。
> 1 PR (PR #168) + 7 文件 + 22 测试 + 88.1% 覆盖率。

### PR-C2.1 Channel 抽象层（execute package 骨架）

- [x] **PR-C2.1.1** 新建 `internal/layers/orchestration/execute/errors.go`
  - 5 SentinelError + 4 helpers:
    - `ErrChannelNotFound` (EXEC_CHANNEL_9001) — PlanKind 未注册或未知
    - `ErrChannelUnsupported` (EXEC_CHANNEL_9002) — 重复 Register
    - `ErrChannelStepCountMismatch` (EXEC_CHANNEL_9003) — Step 数与 Channel 不匹配
    - `ErrChannelPlanNil` (—) — Plan 参数为 nil
    - `ErrChannelToolRunnerNil` (EXEC_CHANNEL_9004) — ToolRunner 参数为 nil
  - 4 helpers: `NewChannelNotFoundError` / `NewChannelUnsupportedError` / `NewChannelStepCountMismatchError` / `NewChannelToolRunnerNilError`

- [x] **PR-C2.1.2** 新建 `internal/layers/orchestration/execute/channel.go`
  - `Channel` interface: `Name()` / `Supports(plan.PlanKind)` / `Execute(ctx, *plan.Plan, ChannelRequest)`
  - `ChannelRequest` struct: `SessionID` / `PriorVerdictKinds []string` (typed as string; Phase 4 type alias will tighten)
  - `ChannelRegistry` struct: `Register(Channel) error` / `Get(plan.PlanKind) (Channel, error)` / 内部 map + 1:1 冲突检测
  - `ChannelRouter` struct: 无状态 `Route(ctx, *plan.Plan, ChannelRequest) (*wavescheduler.Artifact, error)` / defensive checks
  - 本地 `ToolRunner` interface: `Invoke(ctx, ToolRequest) (ToolResult, error)` — 解耦 PR-C4
  - `ToolRequest` / `ToolResult` struct

### PR-C2.2 4 个具体 Channel

- [x] **PR-C2.2.1** 新建 `internal/layers/orchestration/execute/channel_commit.go`
  - `CommitChannelConfig`: `Timeout time.Duration` (default 5s)
  - `CommitChannel` struct: runner + cfg
  - `NewCommitChannel(runner, cfg) (*CommitChannel, error)` — nil runner → ErrChannelToolRunnerNil
  - `Name()` → "commit"
  - `Supports(PlanKind)` → true 仅 CommitmentPlan
  - `Execute(ctx, p, req)`:
    - 强制 len(p.Steps) == 1 (else ErrChannelStepCountMismatch)
    - 强制 step.IdempotencyKey != ""
    - 同步 + 超时 → SideEffectInflight (compensate on retry)
    - exitCode=0 → SideEffectCommitted
    - 其他 error → SideEffectUnknown
  - 产出 `Artifact{Kind: ArtifactStateChangeCert, ...}`

- [x] **PR-C2.2.2** 新建 `internal/layers/orchestration/execute/channel_protocol.go`
  - `ProtocolChannel` struct: runner
  - `NewProtocolChannel(runner) (*ProtocolChannel, error)`
  - `Name()` → "protocol"
  - `Supports(PlanKind)` → true 仅 ProtocolPlan
  - `Execute(ctx, p, req)`:
    - 拒绝空 Steps
    - 顺序执行 Steps
    - 任一 step 失败 → 已 commit 的 steps reverse-order rollback (via `__rollback: true` Args hint)
    - 全部 success → SideEffectCommitted
    - rollback 后 → SideEffectRolledBack
  - 产出 `Artifact{Kind: ArtifactResponseRecord, ...}`

- [x] **PR-C2.2.3** 新建 `internal/layers/orchestration/execute/channel_scenario.go`
  - `ScenarioChannel` struct: runner + MaxParallel=5
  - `NewScenarioChannel(runner) (*ScenarioChannel, error)`
  - `Name()` → "scenario"
  - `Supports(PlanKind)` → true 仅 ScenarioPlan
  - `Execute(ctx, p, req)`:
    - 并行探测 MaxParallel=5
    - 多数派投票 (success > len/2 → pass)
    - 失败多数派 → ErrChannelStepCountMismatch
    - SideEffectStatus=None (read-only)
  - 产出 `Artifact{Kind: ArtifactProbeReport, ...}`

- [x] **PR-C2.2.4** 新建 `internal/layers/orchestration/execute/channel_exploration.go`
  - `ExplorationChannel` struct: runner + MaxParallel=3
  - `NewExplorationChannel(runner) (*ExplorationChannel, error)`
  - `Name()` → "exploration"
  - `Supports(PlanKind)` → true 仅 ExplorationPlan
  - `Execute(ctx, p, req)`:
    - 多 agent 并行 MaxParallel=3
    - 容忍部分失败 (free-fork 语义)
    - 优先级排序: success → duration → EstimatedTokens
    - PersistScope → SideEffectStatus 派生:
      - PersistTransient → SideEffectNone
      - PersistSession / PersistPermanent → SideEffectCommitted
      - PersistUnset / unknown → SideEffectUnknown
  - 产出 `Artifact{Kind: ArtifactExperimentData, ...}`

### PR-C2.3 测试

- [x] **PR-C2.3.1** 新建 `internal/layers/orchestration/execute/execute_test.go`
  - 22 tests / 88.1% coverage / 0 race detector warnings
  - 覆盖 D7-S9-A26-T01 (ChannelRegistry + ChannelRouter) + T02 (CommitChannel) + T03 (ProtocolChannel) + T04 (ScenarioChannel) + T05 (ExplorationChannel)
  - `fakeRunner`: `OnInvoke(tool, handler)` per-tool handler + `Invoke` 记录所有调用
  - `validPlan(kind, sessionID, steps)` fixture
  - `atomicCounter` 用于并行性断言

### PR-C2.4 文档同步

- [x] **PR-C2.4.1** `openspec/specs/d7-orchestration/spec.md` — 新增 D7-S9-A26 Requirement (D7 v4.3.0)
- [x] **PR-C2.4.2** `openspec/specs/d7-orchestration/t-registry.md` — 新增 D7-S9-A26-T01..T05 (v3.11.0)

## 验证

### S4-Gate

- [x] go vet ./... 0 issue
- [x] go test -race -count=1 ./internal/layers/orchestration/execute/... 22/22 PASS, 0 race
- [x] go build ./... 0 error
- [x] go test -race ./internal/lint/layer/... 0 violation (layer-lint)

### S5-Gate

- [x] go test -cover ./internal/layers/orchestration/execute/... 88.1% ≥ 80% gate
- [x] 5 个 P0 T 点全部 IMPLEMENTED（D7-S9-A26-T01..T05）
- [x] PR #168 squash merged 2026-06-23

### S6-Gate

- [x] spec.md v4.2.0 → v4.3.0 (D7-S9-A26 Requirement 落地)
- [x] t-registry v3.10.0 → v3.11.0 (D7-S9-A26-T01..T05 登记 + IMPLEMENTED 142→147, P0 109→114)
- [x] demand-archive-index.md DM-20260625-001-PRC2 行已加
- [x] verify-archive.sh devrix-d7-mups-v4-phase3-channels 12/12 PASS

## 不在本次任务范围

- PR-C3 StrategyDecider + RetryPolicy
- PR-C4 ToolSpec v3 (10 fields)
- PR-C5 ExecutionEvidence 结构化
- PR-C6 VerifyTrigger wiring
- PR-C7 Executor + DispatchWorker v2
- Phase 4 Verify ReverseLookup consumer
- Phase 5 Learn ExperimentData consumer
