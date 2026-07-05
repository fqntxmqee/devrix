# Tasks: d7-mups-strategy-injection — M3 实施任务列表

**Change ID:** d7-mups-strategy-injection
**Demand ID:** DM-20260705-008
**Phase:** S4 (Implementation)
**Status:** COMPLETED 2026-07-05

---

## T01 — Strategy interface + 4 PlanKind implementations

- [x] T01.1 — `internal/layers/orchestration/workmodel/strategy.go` (NEW, 64 lines)
  - 4-method interface: `RouteChannel`, `SpawnOverride`, `ShouldDecompose`, `IsReadOnly`
  - **Interface signature evolution** (M3 refinement during implementation):
    - Initial: `SpawnOverride(planKind, verdictKind) (SpawnPolicy, bool)`
    - Final: `SpawnOverride(round *WorkItemPipelineRound) (SpawnPolicy, bool)`
    - Rationale: CC-1.4 deliverable continuation must take precedence over
      the commitment terminal override. Strategy needs round context
      (DeliverableSchema/Status) to decide.
- [x] T01.2 — `strategy_commitment.go` (NEW, 65 lines)
  - 1-step synchronous commitment plan
  - M3 行为增量: `VerdictFail + CommitmentPlan → SpawnNone`, `VerdictPartial + CommitmentPlan → SpawnNone`
  - CC-1.4 precedence: incomplete deliverable → `ok=false` (fall through)
- [x] T01.3 — `strategy_protocol.go` (NEW, 42 lines)
  - Safe default; all 20 combinations return `ok=false` (no override)
- [x] T01.4 — `strategy_scenario.go` (NEW, 47 lines)
  - Read-only probe; no retry on fail
  - M3 行为增量: `VerdictFail + ScenarioPlan → SpawnNone` (was SpawnParallelExplore)
- [x] T01.5 — `strategy_exploration.go` (NEW, 51 lines)
  - Parallel experiments; continue on pass
  - M3 行为增量: `VerdictPass + ExplorationPlan → SpawnDecompose` (was SpawnNone)
- [x] T01.6 — `strategy_default.go` (NEW, 67 lines)
  - `defaultStrategies` registry (1:1 PlanKind → Strategy)
  - `LookupStrategy(planKind)` — returns bound or `protocolStrategy` (safe default)
  - `RegisterStrategy(planKind, s)` — extension point for tests
  - `init()` validation: exactly 4 PlanKind bindings

## T02 — WorkItemExecContext integration

- [x] T02.1 — `sessionorchestrator/workitem_exec_context.go` MODIFIED (+12 lines)
  - Added `import "internal/layers/orchestration/plan"` (for `plan.KindUnset`)
  - Added `Strategy workmodel.Strategy` field to `WorkItemExecContext` struct
  - Added nil-guard in `WithWorkItemExecContext`: `ec.Strategy == nil` →
    `ec.Strategy = workmodel.LookupStrategy(plan.KindUnset)` (protocolStrategy)

## T03 — spawn_decision_algebra integration

- [x] T03.1 — `workmodel/spawn_decision_algebra.go` MODIFIED (+8 lines)
  - `checkVerdictDirection` refactored: switch returns `policy` (was direct return)
  - End of function: 1-line M3 override hook:
    ```go
    if p, ok := LookupStrategy(round.PlanKind).SpawnOverride(round); ok {
        return p
    }
    return policy
    ```
  - `protocolStrategy` + unknown PlanKinds return `ok=false` → 0 behavior change

## T04 — Update existing tests (5 行为增量 alignment)

- [x] T04.1 — `workmodel/spawn_policy_test.go` MODIFIED (+8 lines)
  - `TestSpawnPolicyEvaluator_R4_ExplorationPass`: expected `SpawnNone` → `SpawnDecompose` (M3 行为增量)
  - `TestSpawnPolicyEvaluator_R6_ScenarioFail`: expected `SpawnParallelExplore` → `SpawnNone` (M3 行为增量)
- [x] T04.2 — `sessionorchestrator/item_pipeline_test.go` MODIFIED (+50 lines)
  - Added `commitmentOnlyPlanner` stub (forces CommitmentPlan for deterministic test)
  - Added `decomposeAwarePlanner` stub (ExplorationPlan for first call, CommitmentPlan for subsequent)
  - Updated `TestRunItemPipeline_SingleWorkItem_Completed` to use `commitmentOnlyPlanner`
    (Goal→intent_orchestrate→ExplorationPlan would trigger M3 行为增量)
- [x] T04.3 — `sessionorchestrator/session_turn_loop_test.go` MODIFIED (+8 lines)
  - `TestRunSessionTurnLoop_SingleGoal_Completes`: uses `commitmentOnlyPlanner`
  - `TestRunSessionTurnLoop_DecomposeRecursive_CompletesChildren`: uses `decomposeAwarePlanner`
    (parent=ExplorationPlan for decompose, children=CommitmentPlan for Pass→complete)

## T05 — New unit tests

- [x] T05.1 — `workmodel/strategy_test.go` (NEW, 270 lines, 14 tests)
  - 4 PlanKind × 5 VerdictKind = 20 cases (4 M3 行为增量 + 16 兜底)
  - `TestStrategy_SpawnOverride_NilRound` (4 sub-cases)
  - `TestStrategy_SpawnOverride_WrongPlanKind` (4 sub-cases)
  - `TestStrategy_RouteChannel` (4 PlanKinds + 1 兜底)
  - `TestStrategy_ShouldDecomposeAndIsReadOnly` (4 PlanKinds)
- [x] T05.2 — `workmodel/strategy_default_test.go` (NEW, 130 lines, 5 tests)
  - `TestLookupStrategy_FourBindings` (4 PlanKinds)
  - `TestLookupStrategy_KindUnset_Default` (safe default)
  - `TestLookupStrategy_UnknownKind_Default` (unknown kind fallback)
  - `TestRegisterStrategy_Override` (extension point)
  - `TestDefaultStrategies_InitValidation` (init() 4-binding count)
- [x] T05.3 — `workmodel/spawn_decision_algebra_test.go` MODIFIED (+185 lines, 5 tests)
  - `TestM3_StrategyOverride_CommitmentPlan_AllVerdicts` (5 verdicts)
  - `TestM3_StrategyOverride_ProtocolPlan_AllVerdicts` (5 verdicts, all fall through)
  - `TestM3_StrategyOverride_ScenarioPlan_AllVerdicts` (5 verdicts)
  - `TestM3_StrategyOverride_ExplorationPlan_AllVerdicts` (5 verdicts)
  - `TestM3_StrategyOverride_BehaviorChangeSummary` (4 行为增量 summary)

## T06 — Roadmap document

- [x] T06.1 — `openspec/specs/d7-orchestration/mups-5node-refactor-roadmap.md` (NEW)
  - 5 节点 (M1+M2+M4+M5+cleanup) + M3 完整 timeline
  - 设计意图 + 5 节点闭环总结

## T07 — Verification

- [x] T07.1 — `go build ./...` — 0 error
- [x] T07.2 — `go test ./internal/layers/orchestration/workmodel/ -count=1` — PASS (新增 19 测试 + 现有 50+ 测试)
- [x] T07.3 — `go test ./internal/layers/orchestration/sessionorchestrator/ -count=1` — PASS
- [x] T07.4 — `go test ./internal/layers/orchestration/plan/ ./internal/layers/orchestration/mups/... -count=1` — PASS
- [x] T07.5 — `go vet ./...` — 0 warning

## Implementation summary

| Metric | Value |
|--------|-------|
| NEW files | 8 (5 strategy + 2 test + 1 spec) |
| MODIFIED files | 5 (workitem_exec_context, spawn_decision_algebra, 2 test files, design.md) |
| New tests | 19 (14 strategy + 5 algebra integration) |
| Existing tests updated | 2 (R4_ExplorationPass, R6_ScenarioFail) |
| 0-behavior-change guarantee | 16/20 combinations (4 兜底) |
| M3 行为增量 | 4/20 combinations (commitment+fail/partial, scenario+fail, exploration+pass) |
| CC-1.4 precedence | preserved (commitment+partial+incomplete deliverable → fall through) |

## Commits

- (will be filled at S6 PR time)
