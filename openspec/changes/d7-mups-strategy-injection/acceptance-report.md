# Acceptance Report: d7-mups-strategy-injection — S5 验收

**Change ID:** d7-mups-strategy-injection
**Demand ID:** DM-20260705-008
**Phase:** S5 (Acceptance)
**Status:** ACCEPTED
**Date:** 2026-07-05
**Verdict:** ✅ ACCEPTED — all success criteria met

---

## 验收范围 (Scope)

M3 节点实施验收：MUPS Strategy 抽象注入 WorkItemExecContext (PlanKind 路由恢复)，
完成 5 节点 (M1+M2+M4+M5+cleanup) 重构总图行为增量最后一步。

## 验收标准 (Acceptance Criteria)

### AC1 — Strategy interface + 4 PlanKind implementations ✅
- [x] `Strategy` interface 定义在 workmodel 包, 4 method 签名一致
- [x] 4 个 PlanKind Strategy 实现完整 (commitment/protocol/scenario/exploration)
- [x] DefaultStrategy registry + LookupStrategy helper 实现
- [x] init() 1:1 验证 (4 bindings) PASS

### AC2 — WorkItemExecContext integration ✅
- [x] `Strategy workmodel.Strategy` 字段新增 (interface, 可空)
- [x] WithWorkItemExecContext nil-guard 兜底 → protocolStrategy
- [x] L1 (mups/execute) 不依赖 workmodel 包 (无循环依赖)

### AC3 — spawn_decision_algebra integration ✅
- [x] `checkVerdictDirection` 末尾 1 行 SpawnOverride override
- [x] 16/20 combinations 0 行为变化 (兜底 fall through)
- [x] 4/20 combinations M3 行为增量 (commitment+fail/partial, scenario+fail, exploration+pass)

### AC4 — Test coverage ✅
- [x] 19 NEW tests created (14 strategy + 5 algebra integration)
- [x] 24 total strategy cases (4 PlanKind × 5 verdict + 4 兜底)
- [x] 4 M3 行为增量 locked in `TestM3_StrategyOverride_BehaviorChangeSummary`
- [x] CC-1.4 precedence: `TestStrategy_SpawnOverride_CommitmentPlan_Partial/incomplete_deliverable → fall through`

### AC5 — Existing tests preserved ✅
- [x] 50+ existing workmodel tests PASS
- [x] 30+ existing sessionorchestrator tests PASS
- [x] 2 existing tests updated (R4_ExplorationPass, R6_ScenarioFail) — both are M3 行为增量 alignment
- [x] CC-1.4 deliverable continuation tests (4) all PASS

### AC6 — Quality gates ✅
- [x] `go build ./...` — 0 error
- [x] `go test ./internal/layers/orchestration/workmodel/ -count=1` — PASS
- [x] `go test ./internal/layers/orchestration/sessionorchestrator/ -count=1` — PASS
- [x] `go test ./internal/layers/orchestration/plan/ -count=1` — PASS
- [x] `go test ./internal/layers/orchestration/mups/... -count=1` — PASS
- [x] `go vet ./...` — 0 warning

### AC7 — Documentation ✅
- [x] `mups-5node-refactor-roadmap.md` 5 节点重构总图创建
- [x] `demand.md` (S1) — 154 lines
- [x] `proposal.md` (S2) — 143 lines
- [x] `design.md` (S3) — 397 lines, 含 interface signature 演进说明
- [x] `tasks.md` (S4) — 实施任务列表
- [x] `acceptance-report.md` (S5) — 本文档
- [x] `specs/d7-orchestration/d7-mups-strategy-injection.md` (spec) — 见 T07

## 关键设计决策 (Key Design Decisions)

### KD1 — Strategy.SpawnOverride 签名演进
- **Initial design**: `SpawnOverride(planKind, verdictKind) (SpawnPolicy, bool)`
- **Final design**: `SpawnOverride(round *WorkItemPipelineRound) (SpawnPolicy, bool)`
- **Rationale**: CC-1.4 deliverable continuation 必须先于 commitment terminal override.
  Strategy 需要 round.DeliverableSchema/Status 来决策. Interface 多一个参数是合理的 trade-off.
- **影响**: 4 strategy 实现 + checkVerdictDirection integration + 14 测试.

### KD2 — 4 M3 行为增量锁定
| PlanKind | Verdict | Old (5-case) | New (M3) | 语义 |
|----------|---------|--------------|----------|------|
| Commitment | Fail | SpawnNone | SpawnNone | 1-step terminal (no-op, locked) |
| Commitment | Partial | varies | SpawnNone | terminal partial acceptance |
| Scenario | Fail | SpawnParallelExplore | SpawnNone | read-only, no retry |
| Exploration | Pass | SpawnNone | SpawnDecompose | parallel explore continues |

### KD3 — CC-1.4 deliverable continuation precedence
- commitment + Partial + incomplete deliverable → fall through (CC-1.4 wins)
- 保证现有 4 个 deliverable continuation 测试 (PartialIncompleteDeliverable_Inlines,
  CCU1_inlineNotEscalateWithEvidence, CCU1_escalateWithoutEvidence,
  DeliverableInlineWouldExhaustEscalatesAtDepth0) 0 修改 PASS

### KD4 — Integration tests 改造 (3 tests)
- `TestRunItemPipeline_SingleWorkItem_Completed`: Goal→ExplorationPlan+Pass 触发 M3
  行为增量, 注入 `commitmentOnlyPlanner` 隔离
- `TestRunSessionTurnLoop_SingleGoal_Completes`: 同上
- `TestRunSessionTurnLoop_DecomposeRecursive_CompletesChildren`: 父需要分解 (Exploration)
  + 子需要 complete (Commitment), 注入 `decomposeAwarePlanner` (call-count aware)

## Gap Analysis (Code ↔ Design)

| 设计元素 | 实现 | 一致性 |
|----------|------|--------|
| Strategy interface (4 method) | ✅ 实现, 签名 (round) 演进 | 一致 (1 refinement) |
| 4 PlanKind Strategy 实现 | ✅ commitment/protocol/scenario/exploration | 一致 |
| DefaultStrategy registry | ✅ 1:1 binding + LookupStrategy + RegisterStrategy | 一致 |
| WorkItemExecContext.Strategy 字段 | ✅ 字段 + nil 兜底 | 一致 |
| checkVerdictDirection 1-line override | ✅ 末尾 1 行 + 4 行 doc | 一致 |
| 16/20 兜底 0 行为变化 | ✅ 16 cases fall through | 一致 |
| 4/20 M3 行为增量 | ✅ 4 cases locked by 24 测试 | 一致 |
| CC-1.4 precedence | ✅ 1 测试 + strategy 逻辑 | 一致 |

## Risks / Open Questions

无 — 所有设计意图已实现并通过验收。

## 结论

**S5 验收通过 (ACCEPTED)**. 5 节点重构总图 (M1+M2+M3+M4+M5+cleanup) 全部完成, M3 行为增量
最后一步落地. Strategy 抽象恢复 Phase 3 PR-C2 设计的 `ChannelRouter 4 PlanKind 路由`
可观察性, 让 spawn policy / plan proposer / verify 等下游节点能根据 PlanKind 显式选择 strategy.

**S6 交付**: PR + Auto-merge → master
**S6 归档**: openspec/changes/ → openspec/archive/2026-07-05-d7-mups-strategy-injection/
