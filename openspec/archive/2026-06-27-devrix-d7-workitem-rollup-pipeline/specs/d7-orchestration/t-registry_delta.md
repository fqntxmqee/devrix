# T-Registry Delta — D7-Orchestration — WorkItem Rollup 闭环

**Change ID:** `devrix-d7-workitem-rollup-pipeline`  
**Target T-Registry:** `openspec/specs/d7-orchestration/t-registry.md`  
**Target Version:** v4.7.0 → v4.8.0  
**Demand ID:** DM-20260627-001  
**Created:** 2026-06-27  
**Status:** S3_Design — 待 Review

---

## Header Change log

Add to Change log section:

| Change ID | Demand ID | Date | T points | Status |
|-----------|-----------|------|----------|--------|
| `devrix-d7-workitem-rollup-pipeline` | DM-20260627-001 | 2026-06-27 | +21 P0 (Phase 1) + 6 P1 (Phase 2) | s3_design → s4_dev |

## Statistics Update (Phase 1 merge target)

| Metric | Before (v4.7.0) | After Phase 1 (v4.8.0) | Delta |
|--------|-----------------|------------------------|-------|
| Total T points | 186 | 207 | +21 |
| IMPLEMENTED | 186 | 186 | 0 (until S4) |
| PLANNED / IN_PROGRESS | — | +20 PLANNED, +1 IN_PROGRESS | +21 |
| P0 T points | 153 | 174 | +21 |
| Scenarios | 14 (D7-S1..S14) | 15 (added D7-S15) | +1 |

## Scenarios Table Update

| Scenario | Description | IMPLEMENTED | PLANNED | Total |
|----------|-------------|-------------|---------|-------|
| D7-S15 | WorkItem Rollup 闭环 — 父 synthesize、bubble materialize、complete deliverable | 0 | 21 | 21 |

---

## ADDED Test Points (D7-S15) — Phase 1 P0

### D7-S15-A50: Parent Rollup Gate

| T ID | Given-When-Then | 归属 A/F | Test 位置（计划） | Status | Priority |
|------|-----------------|----------|-------------------|--------|----------|
| **D7-S15-A50-T01** | Given WorkItem JSON v2 When Load Then NeedsRollup defaults false | D7-S15-A50 | `workmodel/workitem_store_test.go` | PLANNED | P0 |
| **D7-S15-A50-T02** | Given decompose parent + 2 terminal implement children When ReevaluateParent Then parent NeedsRollup=true pending | D7-S15-A50 | `workmodel/resolve_rollup_test.go` | PLANNED | P0 |
| **D7-S15-A50-T03** | Given rollup pending parent + open explore child When GetPipelineFocus Then parent selected first | D7-S15-A50 | `workmodel/work_tree_test.go` | PLANNED | P0 |

### D7-S15-A55: RollupGatePolicy — ShouldRollupAfterChildren

| T ID | Given-When-Then | 归属 A/F | Test 位置（计划） | Status | Priority |
|------|-----------------|----------|-------------------|--------|----------|
| **D7-S15-A55-T01** | Given RollupGatePolicy=all_pass + 1 child fail When ShouldRollupAfterChildren Then false | D7-S15-A55 | `workmodel/resolve_rollup_test.go` | PLANNED | P0 |
| **D7-S15-A55-T02** | Given RollupGatePolicy=min_coverage 0.8 + 4/5 terminal When ShouldRollupAfterChildren Then true | D7-S15-A55 | `workmodel/resolve_rollup_test.go` | PLANNED | P0 |
| **D7-S15-A55-T03** | Given best_effort + all children terminal incl fail When ReevaluateParent Then NeedsRollup=true | D7-S15-A55 | `workmodel/resolve_rollup_test.go` | PLANNED | P0 |

### D7-S15-A51: Summary Bubble Materialize（P0 — T05 S4 闭合）

| T ID | Given-When-Then | 归属 A/F | Test 位置（计划） | Status | Priority |
|------|-----------------|----------|-------------------|--------|----------|
| **D7-S15-A51-T01** | Given child ArtifactSummary 3000 runes When SummaryBubbleStatement Then preview ≤2048 runes | D7-S15-A51 | `workmodel/context_bubble_apply_test.go` | PLANNED | P0 |
| **D7-S15-A51-T02** | Given rollup parent Observe When buildObserveContext Then contains summary_child_bubble **and** structured_child_bubble per child | D7-S15-A51 | `sessionorchestrator/item_observe_test.go` | **IN_PROGRESS** | **P0** |
| **D7-S15-A51-T03** | Given dual-bubble Observe When rollup Execute Then directive uses summary previews not structured-only | D7-S15-A51 | `sessionorchestrator/item_pipeline_rollup_test.go` | PLANNED | P0 |

### D7-S15-A60: Parent Rollup Round 2+ MUPS

| T ID | Given-When-Then | 归属 A/F | Test 位置（计划） | Status | Priority |
|------|-----------------|----------|-------------------|--------|----------|
| **D7-S15-A60-T01** | Given NeedsRollup parent When RunItemPipeline Then PlanKind=CommitmentPlan + rollup FailureCriteria | D7-S15-A60 | `sessionorchestrator/item_pipeline_rollup_test.go` | PLANNED | P0 |
| **D7-S15-A60-T02** | Given 2 children pass/fail When rollup Execute Then directive lists wi_id+verdict+summary | D7-S15-A60 | `sessionorchestrator/item_pipeline_rollup_test.go` | PLANNED | P0 |
| **D7-S15-A60-T03** | Given rollup Verify Pass When round ends Then NeedsRollup=false Status=completed | D7-S15-A60 | `sessionorchestrator/item_pipeline_rollup_test.go` | PLANNED | P0 |

### D7-S15-A61: Session complete Deliverable

| T ID | Given-When-Then | 归属 A/F | Test 位置（计划） | Status | Priority |
|------|-----------------|----------|-------------------|--------|----------|
| **D7-S15-A61-T01** | Given root post-rollup summary len≥500 When TurnLoop exits Then complete.Content=summary | D7-S15-A61 | `sessionorchestrator/session_turn_loop_test.go` | PLANNED | P0 |
| **D7-S15-A61-T02** | Given empty root summary + child summaries When TurnLoop exits Then best-effort complete or error | D7-S15-A61 | `sessionorchestrator/session_turn_loop_test.go` | PLANNED | P0 |

### D7-S15-A53: Ephemeral Checklist Gate

| T ID | Given-When-Then | 归属 A/F | Test 位置（计划） | Status | Priority |
|------|-----------------|----------|-------------------|--------|----------|
| **D7-S15-A53-T01** | Given 11 ephemeral checklist pending When GetPipelineFocus Then not checklist | D7-S15-A53 | `workmodel/work_tree_test.go` | PLANNED | P0 |
| **D7-S15-A53-T02** | Given only ephemeral open after rollup When HasOpenWork Then false | D7-S15-A53 | `workmodel/spawn_apply_test.go` | PLANNED | P0 |
| **D7-S15-A53-T03** | Given root R1 done + checklist When focus Then nil or root rollup | D7-S15-A53 | `workmodel/work_tree_test.go` | PLANNED | P0 |

### D7-S15-A54: Root Session Rollup Fallback

| T ID | Given-When-Then | 归属 A/F | Test 位置（计划） | Status | Priority |
|------|-----------------|----------|-------------------|--------|----------|
| **D7-S15-A54-T01** | Given root spawn=none + checklist When maybeRootRollupFallback Then NeedsRollup=true | D7-S15-A54 | `sessionorchestrator/session_turn_loop_test.go` | PLANNED | P0 |
| **D7-S15-A54-T02** | Given checklist directive When ChecklistBubbleStatement Then CB3 truncate | D7-S15-A54 | `workmodel/context_bubble_apply_test.go` | PLANNED | P0 |
| **D7-S15-A54-T03** | Given Path B rollup Observe When buildObserveContext Then checklist_child_bubble | D7-S15-A54 | `sessionorchestrator/item_observe_test.go` | PLANNED | P0 |
| **D7-S15-A54-T04** | Given trace replay fixture When TurnLoop Then root 2× MUPS + complete P0/P1 | D7-S15-A54 | `tests/integration/d7/d7_rollup_trace_replay_test.go` | PLANNED | P0 |

### D7-S15 Integration (P0 E2E)

| T ID | Given-When-Then | 归属 A/F | Test 位置（计划） | Status | Priority |
|------|-----------------|----------|-------------------|--------|----------|
| **D7-S15-IT01** | Given decompose fixture session When full TurnLoop Then same wi_id 2× MUPS + complete non-empty | D7-S15 | `tests/integration/d7/d7_rollup_test.go` | PLANNED | P0 |
| **D7-S15-IT02** | Given spawn=none + checklist fixture When TurnLoop Then zero checklist MUPS spans | D7-S15 | `tests/integration/d7/d7_rollup_trace_replay_test.go` | PLANNED | P0 |

---

## ADDED Test Points (D7-S15) — Phase 2 P1（登记）

| T ID | Description | Status | Priority |
|------|-------------|--------|----------|
| D7-S15-A52-T01 | DecomposeProposer.Propose called before ApplySpawnPolicy on SpawnDecompose | PLANNED | P1 |
| D7-S15-A52-T02 | LastRound.ChildSpecs persisted for audit | PLANNED | P1 |
| D7-S15-A52-T03 | ChildSpec.ExpectedReturn stored as text constraint (no JSON schema) | PLANNED | P1 |
| D7-S15-A52-T04 | Parent FailureCriteria template instantiated on child Plan | PLANNED | P1 |
| D7-S15-A52-T05 | todo_write alone does not satisfy decompose audit | PLANNED | P1 |
| D7-S15-A62-T01 | RunParallelExplore dispatches Wave | PLANNED | P1 |
| D7-S15-A62-T02 | Trace contains Wave schedule span | PLANNED | P1 |

---

## Scenario D7-S15 Detail (test points summary)

```
D7-S15  WorkItem Rollup 闭环
├── A50  Parent Rollup Gate
│   ├── T01  NeedsRollup schema backward compat     [PLANNED P0]
│   ├── T02  ReevaluateParent rollup gate           [PLANNED P0]
│   ├── T03  GetPipelineFocus priority boost        [PLANNED P0]
├── A55  RollupGatePolicy
│   ├── T01  all_pass blocks on child fail          [PLANNED P0]
│   ├── T02  min_coverage threshold                 [PLANNED P0]
│   └── T03  best_effort default on all terminal    [PLANNED P0]
├── A51  Summary Bubble Materialize
│   ├── T01  CB3 truncate                           [PLANNED P0]
│   ├── T02  Observe dual bubble (T05 closure)        [IN_PROGRESS P0]
│   └── T03  Rollup directive uses summary previews [PLANNED P0]
├── A60  Parent Rollup Round 2+ MUPS
│   ├── T01  CommitmentPlan + FailureCriteria      [PLANNED P0]
│   ├── T02  Rollup directive template              [PLANNED P0]
│   └── T03  Verify Pass clears NeedsRollup         [PLANNED P0]
├── A61  Session complete Deliverable
│   ├── T01  complete.Content = root summary        [PLANNED P0]
│   └── T02  empty fallback / error                 [PLANNED P0]
├── A53  Ephemeral Checklist Gate
│   ├── T01  GetFocus skips ephemeral checklist       [PLANNED P0]
│   ├── T02  HasOpenWork after rollup                  [PLANNED P0]
│   └── T03  root R1 + checklist focus behavior        [PLANNED P0]
├── A54  Root Session Rollup Fallback
│   ├── T01  maybeRootRollupFallback                   [PLANNED P0]
│   ├── T02  ChecklistBubbleStatement CB3              [PLANNED P0]
│   ├── T03  Path B Observe checklist bubbles          [PLANNED P0]
│   └── T04  trace replay E2E                          [PLANNED P0]
├── IT01  decompose + rollup E2E                      [PLANNED P0]
└── IT02  trace replay no checklist MUPS              [PLANNED P0]
```

**Phase 1 Total:** 21 P0 T points (18 unit + 2 IT + 1 IN_PROGRESS on A51-T02), S4 开发中。
