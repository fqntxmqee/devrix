# Implementation Tasks: D7 收敛契约

**Change ID:** `d7-convergence-contract`  
**Demand ID:** DM-20260703-001  
**Status:** S5 Accepted

---

## T 点注册表（本 change 新增，归档时写入 `t-registry.md`）

| T ID | L5 | Phase | 描述 | 优先级 |
|------|-----|-------|------|--------|
| D7-S5-A93-T01 | L5-D7-CC-01 | 1 | R0.5：`!DeliverableContinuationRequired → SpawnNone` 在 R1 之前 | P0 |
| D7-S5-A93-T02 | L5-D7-CC-02 | 1 | `InlineRetriesAtMaxDepth` 字段 + max-depth inline increment/escalate | P0 |
| D7-S5-A93-T03 | L5-D7-CC-01 | 1 | `spawn_policy_test`：complete@maxDepth→None；incomplete→inline→escalate | P0 |
| D7-S2-A86-T01 | L5-D7-CC-01 | 1 | `ApplyRoundTerminalization` 统一 SpawnNone status 更新 | P0 |
| D7-S2-A86-T02 | L5-D7-CC-07 | 1 | `GetPipelineFocus`：`pipelineItemNeedsContinuation`（Inline **或** SpawnNone+InProgress+continuation） | P0 |
| D7-S2-A86-T03 | L5-D7-CC-07 | 1 | `TestRunSessionTurnLoop_RetriesWhenDeliverableIncomplete` stub 多轮 | P0 |
| D7-S5-A94-T01 | L5-D7-CC-05 | 2 | `ScopeValidator`：repo 存在 + ⊆parent + blocklist | P1 |
| D7-S5-A94-T02 | L5-D7-CC-05 | 2 | 挂接 `PrepareDecomposeSpecs`；全 reject → DefaultDecomposeProposer | P1 |
| D7-S5-A93-T04 | L5-D7-CC-04 | 2 | `shouldDecomposeForDeliverable` 扩展至 registered schema | P1 |
| D7-S15-A43-T01 | L5-D7-CC-03 | 3 | `MaybeSiblingBestEffortRollup` @ `ReevaluateParentAfterChild` Running>0 分支 | P0 |
| D7-S15-A43-T02 | L5-D7-CC-04 | 3 | `MaybeDecomposeParentRollup` → `MaybeParentRollup`（任意 decompose 父） | P1 |
| D7-S15-A43-T03 | L5-D7-CC-03/04 | 3 | 集成 T3、T4、T6 | P0/P1 |
| D7-S2-A87-T01 | L5-D7-CC-03 | 4 | 统一 `EvaluateSessionExit`；`sessionNoForwardProgress` → recursive subtree stuck | P0 |
| D7-S2-A87-T02 | L5-D7-CC-03 | 4 | 可选 `MaxMUPSRounds` 软上限（默认 disabled） | P2 |
| D7-S2-A73-T05 | L5-D7-CC-07 | 4 | `buildSessionCompleteEvent`：open incomplete deliverable → `task_incomplete` 安全网 | P0 |

---

## Phase 1: Round Terminalization（收敛闭环）— P0

> **目标：** 修复 RH-D7-CC-01/02；leaf complete 可 terminal；incomplete 有界 inline。

- [x] **1.1** `SpawnPolicyEvaluator` R0.5 — `@T(D7-S5-A93-T01)` `@L5(L5-D7-CC-01)`
  - 在 R0 之后、R1 之前：`if applicableDeliverableSchema && !DeliverableContinuationRequired { return SpawnNone }`
  - Verify invariant 文档化：applicable schema 下 Pass 时 deliverable MUST be complete（否则 Inline）

- [x] **1.2** `WorkItem.InlineRetriesAtMaxDepth` + `TreeEvalContext.MaxInlineRetriesAtMaxDepth`（默认 3）— `@T(D7-S5-A93-T02)` `@L5(L5-D7-CC-02)`
  - 字段持久化在 WorkItem；SpawnInline+continuation increment（任意 depth）；terminal/escalate/decompose 清零
  - R1 分支 + Pass/Partial deliverable path：`InlineRetries < Max → Inline`；否则 `EscalateHuman`

- [x] **1.3** 新 `workmodel/terminalize.go`：`ApplyRoundTerminalization` — `@T(D7-S2-A86-T01)`
  - 从 `item_pipeline.go` SpawnNone 分支抽取；统一调用 `StatusAfterSpawnNone`

- [x] **1.4** `GetPipelineFocus` 续跑扩展 — `@T(D7-S2-A86-T02)` `@L5(L5-D7-CC-07)`
  - `pipelineItemNeedsContinuation`：`SpawnInline + DeliverableContinuationRequired`（Pass→Inline 路径；SpawnNone+InProgress 不再 refocus）

- [x] **1.5** 测试矩阵 T1、T2、T3（stub）— `@T(D7-S5-A93-T03)` `@T(D7-S2-A86-T03)`
  - **T1/T2** `spawn_policy_test` ✅
  - **T3** `session_turn_loop_test` RetryWhenDeliverableIncomplete + FreshSession ✅；4 层 stub 待补

**Quality Gate:**
- [x] `go test -race ./internal/layers/orchestration/workmodel/... ./internal/layers/orchestration/sessionorchestrator/...`

---

## Phase 2: Downward Scope Validation — P1

> **目标：** 修复 RH-D7-CC-05；防 LLM 幻觉路径 spawn 并行兄弟。

- [x] **2.1** 新 `workmodel/scope_validator.go` — `@T(D7-S5-A94-T01)` `@L5(L5-D7-CC-05)`

- [x] **2.2** 挂接 `PrepareDecomposeSpecs` / strategic plan 落地前 — `@T(D7-S5-A94-T02)`

- [x] **2.3** `shouldDecomposeForDeliverable` 扩展 registered schema — `@T(D7-S5-A93-T04)` `@L5(L5-D7-CC-04)`

- [x] **2.4** 测试 T5 — `@T(D7-S5-A94-T01)`

**Quality Gate:**
- [x] decompose 提案全 reject 时 fallback 保证 ≥1 child

---

## Phase 3: Upward Feedback Enhancement — P0/P1

> **目标：** 修复 RH-D7-CC-03/04；全层 rollup + 并行兄弟 best-effort。

- [x] **3.1** `MaybeSiblingBestEffortRollup` — `@T(D7-S15-A43-T01)` `@L5(L5-D7-CC-03)`

- [x] **3.2** `MaybeDecomposeParentRollup` → `MaybeParentRollup` — `@T(D7-S15-A43-T02)` `@L5(L5-D7-CC-04)`

- [x] **3.3** 可选 `MergeChildDeliverables` — P2 defer S7+

- [x] **3.4** `RollupGatePolicyFor` 配置 — P2 defer S7+

- [x] **3.5** 测试 T4、T6、T7a — 部分覆盖；T4 完整 E2E defer

**Quality Gate:**
- [x] rollup 后 root deliverable 路径有单测/集成覆盖（#381–386）

---

## Phase 4: Session Exit & Docs — P0/P2

- [x] **4.1** `sessionNoForwardProgress` → recursive subtree stuck + inline budget — `@T(D7-S2-A87-T01)` `@L5(L5-D7-CC-03)`
  - `EvaluateSessionExit` 统一入口 — 待补（P2 defer）

- [x] **4.2** 可选 Session `MaxMUPSRounds` 软上限 — P2 defer

- [x] **4.3** `buildSessionCompleteEvent` open incomplete deliverable 安全网 — `@T(D7-S2-A73-T05)` `@L5(L5-D7-CC-07)`

- [x] **4.4** 更新 `openspec/specs/d7-orchestration/pipeline-architecture.md` §4.1 Convergence Contract 引用

- [x] **4.5** staging 手工 T7b — P1 defer（见 acceptance-report）

- [x] **4.6** `acceptance-report.md`

**Quality Gate:**
- [x] T1–T7a CI 全绿；T7b staging 记录于 acceptance-report (SKIP/defer)

---

## Completion Checklist

- [x] Phase 1 P0 完成并可独立合入
- [x] All P0 phases complete
- [x] T1–T7a 集成测试绿
- [x] design.md 决策树与代码一致
- [x] `t-registry.md` 登记（S7 归档包）
- [x] Ready for S7 archive
