# Tasks: D7 WorkItem Rollup 闭环

**Change ID:** `devrix-d7-workitem-rollup-pipeline`  
**Demand ID:** DM-20260627-001  
**Status:** S7_Archived — Phase 1 完成 2026-06-27  
**Total Tasks:** Phase 1 = 8 组（P0）；Phase 2 = 4 组（P1，登记不编码）

---

## Phase 1 — Rollup 闭环（P0）

### P1-T1 Parent Rollup Gate + NeedsRollup 状态

**Files:** `workmodel/resolve.go`, `workmodel/workitem.go`, `workmodel/workitem_store.go`, `workmodel/work_tree.go`  
**L4:** D7-S1-A50  
**L5:** D7-S15-A50-T01..T03  
**Effort:** 1 天

| ID | Description | L5 | Status |
|----|-------------|-----|--------|
| T01 | `WorkItem` 新增 `NeedsRollup bool`（JSON omitempty）；DiskStore v3 向后兼容 | D7-S15-A50-T01 | [x] |
| T02 | `reevaluateParentAfterChild`：调用 `ShouldRollupAfterChildren(parent, RollupGatePolicy)`；满足门控 → 父 `pending` + `NeedsRollup=true`，**不** auto-complete | D7-S15-A50-T02 | [x] |
| T03 | `GetPipelineFocus`：rollup pending 父节点优先级提升（高于新 explore 子项） | D7-S15-A50-T03 | [x] |
| T03b | `RollupGatePolicy` 枚举 + `ShouldRollupAfterChildren`：Phase 1 **best_effort only**（`RollupGatePolicyFor`） | D7-S15-A55-T01..T03 | [x] |

**AC:** `go test ./internal/layers/orchestration/workmodel/... -run Rollup -count=1` PASS ✓

| ID | Description | L5 | Status |
|----|-------------|-----|--------|
| T01b | `ReopenForRollup`：`failed/completed+Locked` → `pending` 合法迁移 + 解锁；同 WI 可跑 R2 | D7-S15-A50-T04 | [x] |

---

### P1-T1b TurnLoop Fallback 插入顺序

**Files:** `sessionorchestrator/session_turn_loop.go`  
**L4:** D7-S1-A54  
**L5:** D7-S15-A54-T05  
**Effort:** 0.5 天

| ID | Description | L5 | Status |
|----|-------------|-----|--------|
| T01c | `focus==nil` 时先 `maybeRootRollupFallback` / rollup gate，再决定 break 或 continue | D7-S15-A54-T05 | [x] |

**AC:** root R1 fail 后 loop 不立即 exit；NeedsRollup 时进入 R2 ✓

---

### P1-T2 Summary Bubble Materialize（T05 P0 闭合）

**Files:** `sessionorchestrator/item_observe.go`, `workmodel/context_bubble_apply.go`  
**L4:** D7-S1-A51  
**L5:** D7-S15-A51-T01..T03  
**Effort:** 0.5 天

| ID | Description | L5 | Status |
|----|-------------|-----|--------|
| T04 | `SummaryBubbleStatement(childID, artifactSummary)` 格式化 + CB3 截断（2k runes） | D7-S15-A51-T01 | [x] |
| T05 | 父 Rollup Observe：`observationsFromChildSummaryBubbles` 注入 summary；**与** structured bubble **双路** materialize | D7-S15-A51-T02 | [x] |
| T05b | Rollup Execute directive 消费 Summary bubble preview（非 structured-only） | D7-S15-A51-T03 | [x] |

**AC:** 单测 Observe 输出含 `summary_child_bubble:` **且** `structured_child_bubble:` 成对出现 ✓

---

### P1-T3 父 Round 2+ Rollup MUPS 编排

**Files:** `sessionorchestrator/item_pipeline.go`, `sessionorchestrator/workitem_executor.go`  
**L4:** D7-S2-A60  
**L5:** D7-S15-A60-T01..T03  
**Effort:** 1.5 天

| ID | Description | L5 | Status |
|----|-------------|-----|--------|
| T06 | 检测 rollup round：`NeedsRollup==true` → PlanKind=`CommitmentPlan` + rollup FailureCriteria 模板 | D7-S15-A60-T01 | [x] |
| T07 | Rollup directive 模板（含子 verdict + summary 列表；禁止 planning monologue） | D7-S15-A60-T02 | [x] |
| T08 | Rollup Verify Pass → `NeedsRollup=false`, `Status=completed`；Fail 保留 best-effort summary | D7-S15-A60-T03 | [x] |
| T08b | rollup 专用 verify（len + 章节启发式） | D7-S15-A60-T04 | [x] |

**AC:** `item_pipeline_rollup_test.go` PASS ✓

---

### P1-T4 Session complete Deliverable 回填

**Files:** `sessionorchestrator/session_turn_loop.go`  
**L4:** D7-S2-A61  
**L5:** D7-S15-A61-T01..T02  
**Effort:** 0.5 天

| ID | Description | L5 | Status |
|----|-------------|-----|--------|
| T09 | `extractSessionDeliverable`：root post-rollup `LastRound.ArtifactSummary` → `complete.Content` | D7-S15-A61-T01 | [x] |
| T10 | 空 summary fallback：best-effort 拼接 direct children summaries；仍空则明确 error 事件 | D7-S15-A61-T02 | [x] |

**AC:** review fixture summary len ≥ 500 ✓

---

### P1-T5 Ephemeral Checklist 门禁 + GetFocus 过滤

**Files:** `workmodel/work_tree.go`, `sessionorchestrator/session_turn_loop.go`  
**L4:** D7-S1-A53  
**L5:** D7-S15-A53-T01..T03  
**Effort:** 0.5 天

| ID | Description | L5 | Status |
|----|-------------|-----|--------|
| T11 | `GetReadyItems`/`GetFocus`：跳过 `Ephemeral && WorkKindChecklist` | D7-S15-A53-T01 | [x] |
| T12 | `HasOpenWork`：ephemeral checklist 不阻塞 session close | D7-S15-A53-T02 | [x] |
| T13 | 单测：11 checklist + root R1 后 focus=nil 或 root rollup pending | D7-S15-A53-T03 | [x] |

**AC:** fixture 无 11× checklist MUPS span ✓

---

### P1-T6 Root Session Rollup Fallback + Virtual Checklist Bubbles

**Files:** `sessionorchestrator/session_turn_loop.go`, `workmodel/context_bubble_apply.go`, `sessionorchestrator/item_observe.go`  
**L4:** D7-S1-A54  
**L5:** D7-S15-A54-T01..T04  
**Effort:** 1 天

| ID | Description | L5 | Status |
|----|-------------|-----|--------|
| T14 | `maybeRootRollupFallback`：root goal + checklist + spawn=none/failed → `NeedsRollup=true` | D7-S15-A54-T01 | [x] |
| T15 | `CollectChecklistChildBubbles` + `ChecklistBubbleStatement`（CB3 截断 directive） | D7-S15-A54-T02 | [x] |
| T16 | Rollup Observe/Execute 消费 checklist bubbles + root R1 context | D7-S15-A54-T03 | [x] |
| T17 | 集成 fixture trace 重放：root 2× MUPS + complete 含 P0/P1 | D7-S15-A54-T04 | [x] stub IT |

**AC:** stub `d7_rollup_trace_replay_test.go` PASS；全 E2E Phase 2 follow-up

---

## Phase 1 验收门禁（S5）

| Check | Command | Pass |
|-------|---------|------|
| 单元 | `go test ./internal/layers/orchestration/workmodel/... ./internal/layers/orchestration/sessionorchestrator/... -run 'Rollup\|SummaryBubble\|SessionDeliverable\|ChecklistFocus\|ObserveWorkItem' -count=1` | ✅ |
| 集成 | stub IT Path B gate | ✅ PARTIAL |
| 回归 | `go test ./internal/layers/orchestration/workmodel/... -run Spawn -count=1` | ✅ |
| Lint | `go vet ./internal/layers/orchestration/...` | ✅ |

**Quality Gate:** PASSED (unit); IT PARTIAL documented in acceptance-report.md

---

## Phase 2 — Decompose + Parallel（P1，登记，独立 PR）

| ID | Description | L4 | L5 | Status |
|----|-------------|-----|-----|--------|
| T20 | `DecomposeProposer.Propose` **先于** `ApplySpawnPolicy` | D7-S1-A52 | A52-T01..T02 | PLANNED |
| T21 | `ChildSpec.ExpectedReturn` 文本约束 | D7-S1-A52 | A52-T03 | PLANNED |
| T22 | 父 `FailureCriteria` 模板 → 子 Plan/Directive | D7-S1-A52 | A52-T04 | PLANNED |
| T18 | `RunParallelExplore` 接 `WaveScheduler` | D7-S2-A62 | A62-T01..T02 | PLANNED |
| T19 | ExplorationPlan N Step → `ExplorationChannel` | D7-S2-A62 | A62-T01..T02 | PLANNED |

**Note:** Phase 2 编码待独立分支/PR。
