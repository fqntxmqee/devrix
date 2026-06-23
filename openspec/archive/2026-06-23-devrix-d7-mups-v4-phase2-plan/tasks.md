# Tasks: D7 MUPS v4.3 Phase 2 PR-B1 — Plan Data Contract + Planner

**Change ID:** `devrix-d7-mups-v4-phase2-plan`
**Demand ID:** DM-20260623-001-PRB1
**Status:** S2_Proposal → S4_Implemented → S7_Archived
**Date:** 2026-06-23
**Author:** MUPS v4.3 Phase 2 Plan 节点落地梳理

---

## PR-B1: Plan 4 类 + Planner interface + MatchKind 4 Rules + Plan.Validate

> 目标：把 doc 43 (D7 Plan 节点) 设计稿同步为 Go 代码 + 测试 + spec 落地。
> 1 PR (PR #167) + 6 文件 + 30 测试 + 93.5% 覆盖率。

### PR-B1.1 数据契约层（plan package 骨架）

- [x] **PR-B1.1.1** 新建 `internal/layers/orchestration/plan/plan.go`
  - `PlanKind` enum (uint8): `KindUnset=0` / `CommitmentPlan=1` / `ProtocolPlan=2` / `ScenarioPlan=3` / `ExplorationPlan=4`
  - `String()` snake_case wire format (`commitment_plan` / `protocol_plan` / `scenario_plan` / `exploration_plan`)
  - `MarshalJSON` 输出 wire format 字符串 + `KindUnset` omitempty
  - `UnmarshalJSON` 未知值 fail-fast
  - `IsKnown()` helper（KindUnset 返回 false）
  - `ParsePlanKind(s string) (PlanKind, error)` CLI 反向解析

- [x] **PR-B1.1.2** 新建 `internal/layers/orchestration/plan/blast_radius.go`
  - `BlastRadius` struct: `FileCount` / `APICallCount` / `TokenCost` / `PersistScope`
  - `PersistScope` enum (uint8): `PersistUnset=0` / `PersistTransient=1` / `PersistSession=2` / `PersistPermanent=3`
  - `PersistScope.Valid()` 边界检查
  - `BlastRadius.Zero()` 工厂（零值）
  - `FailureCriterion` struct: `Field` / `Op` (whitelist) / `Value` (any)
  - `Step` struct: `ID` / `Directive` / `ToolName` / `ToolArgs` / `IdempotencyKey` / `EstimatedTokens`

- [x] **PR-B1.1.3** 新建 `internal/layers/orchestration/plan/errors.go`
  - 9 SentinelError（标准 errors.New + sharederrors 包装）：
    - `ErrPlanKindUnset` (PLAN_KIND_8001)
    - `ErrPlanSourceObservationIDsRequired` (PLAN_LINEAGE_8002)
    - `ErrPlanBlastRadiusExceeded` (PLAN_BLAST_8003)
    - `ErrPlanStrengthOutOfRange` / `ErrPlanStepsEmpty` / `ErrPlanFailureCriteriaEmpty` / `ErrPlanFailureCriterionInvalidOp` / `ErrPlanFailureCriterionInvalidField` / `ErrPlanPersistScopeInvalid`
  - 3 helpers: `NewPlanKindUnsetError` / `NewPlanSourceObservationIDsRequiredError` / `NewPlanBlastRadiusExceededError`

### PR-B1.2 Plan struct + 不可变 With* + Validate

- [x] **PR-B1.2.1** 新建 `internal/layers/orchestration/plan/plan_struct.go`
  - `Plan` struct (immutable)
  - `NewPlan(id, sessionID, kind, obsIDs, steps, strength) Plan` constructor
    - **防御性拷贝** `sourceObservationIDs` 切片
  - `WithKind(kind PlanKind) Plan` — 返回新副本
  - `WithStrength(s float64) Plan` — 强制 PP-1 ∈ [0,1]
  - `WithFailureCriteria([]FailureCriterion) Plan` — 防御性拷贝
  - `WithBlastRadius(BlastRadius) Plan` — 返回新副本
  - `WithAnomaliesCount(int) Plan` — 返回新副本
  - `Validate() error` — PP-1 强度范围 + PP-2 FailureCriteria 非空 + Op 白名单 + Field 可观察 + PP-3 BlastRadius 阈值
  - `ValidateWithOpts(ValidateOpts) error` — 自定义阈值
  - `ReverseLookupObservations(lookup ObservationLookup) []Observation` — Phase 4 入口

### PR-B1.3 Planner + DefaultPlanner + MatchKind

- [x] **PR-B1.3.1** 新建 `internal/layers/orchestration/plan/planner.go`
  - `Planner` interface: `Plan(ctx context.Context, input PlanInput) (*Plan, error)`
  - `PlanInput` struct: `SessionID` / `UncertaintyReport` / `StepTemplates []Step`
  - `DefaultPlanner` struct 实现
  - `MatchKind(quantizedKind string, stepCount, anomaliesCount int) PlanKind` — 4 规则 + uncertainty-first tie-break
  - `strengthFloor(anomaliesCount, observationCount int) float64` — 公式 `0.7 - 0.1·anomalies + min(observations·0.02, 0.2)`

### PR-B1.4 测试

- [x] **PR-B1.4.1** 新建 `internal/layers/orchestration/plan/plan_test.go`
  - 30 tests / 93.5% coverage / 0 race detector warnings
  - 覆盖 D7-S8-A22-T01 (PlanKind 4 类 + wire format) + T02 (SourceObservationIDs + ReverseLookup) + T03 (MatchKind 4 Rules + DefaultPlanner 集成)
  - 补充 22 个边界测试（With* 不可变性 / BlastRadius.Zero / PersistScope.Valid / Validate 失败路径 / MarshalJSON round-trip / helper errors / ValidateOpts 自定义阈值）

### PR-B1.5 文档同步

- [x] **PR-B1.5.1** `openspec/specs/d7-orchestration/spec.md` — 新增 D7-S8-A22 Requirement (D7 v4.3.0)
- [x] **PR-B1.5.2** `openspec/specs/d7-orchestration/t-registry.md` — 新增 D7-S8-A22-T01..T03 (v3.11.0)

## 验证

### S4-Gate

- [x] go vet ./... 0 issue
- [x] go test -race -count=1 ./internal/layers/orchestration/plan/... 30/30 PASS, 0 race
- [x] go build ./... 0 error

### S5-Gate

- [x] go test -cover ./internal/layers/orchestration/plan/... 93.5% ≥ 80% gate
- [x] 3 个 P0 T 点全部 IMPLEMENTED（D7-S8-A22-T01/T02/T03）
- [x] PR #167 squash merged 2026-06-23

### S6-Gate

- [x] spec.md v4.2.0 → v4.3.0 (D7-S8-A22 Requirement 落地)
- [x] t-registry v3.10.0 → v3.11.0 (D7-S8-A22-T01..T03 登记 + IMPLEMENTED 139→142)
- [x] demand-archive-index.md DM-20260623-001-PRB1 行已加
- [x] verify-archive.sh devrix-d7-mups-v4-phase2-plan 12/12 PASS

## 不在本次任务范围

- PR-A2 IntentQuantizer
- PR-A3 AnomalyDetector
- PR-A4 ObserveNode wiring
- PR-B2 Plan.Validate 细化（Field 可观察性扩展）
- PR-B3 LLMPlanner
- Phase 3 Execute 4 Channel (PR-C2): 并行 PR，依赖本 PR 落地
- Phase 4 Verify ReverseLookup consumer
- Phase 5 Learn ReputationEvidence consumer
