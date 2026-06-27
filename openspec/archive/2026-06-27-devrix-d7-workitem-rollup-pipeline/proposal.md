# Proposal: D7 WorkItem Rollup 闭环

**Change ID:** `devrix-d7-workitem-rollup-pipeline`  
**Demand ID:** DM-20260627-001  
**Priority:** P0  
**Status:** S7_Archived — Phase 1 完成 2026-06-27  
**SoT:** trace `58e6c55dd4d42284e4c2bed3ebeda28b` + WorkTree/MUPS 架构对齐讨论  

---

## 1. Problem Statement

WorkItem Pipeline（Phase A–E）已合流为 ingress 主路径，但 **G1「不确定问题可递归探索直至收敛」** 与 **G5「Turn Loop 驱动 session 工作集关闭」** 在终局交付上 **未闭合**：

1. **无父层 Rollup**：`ReevaluateParentAfterChild` 在子全完成时将父标为 `completed`，**跳过**父 MUPS Round 2 synthesize（与设计 §3.1「子 terminal → decide **或** completed」不符）。  
2. **向上传播半实现**：`CollectStructuredChildBubbles` 仅 LP-5 元数据；`BubbleSummary` Evaluator 存在但 **未 materialize 到 Observe**。  
3. **拆解不可审计**：LLM 通过 `todo_write` / `free_fork` 绕过 `SpawnPolicyEvaluator`；无 **DecomposeProposer** 产出 `ChildSpec` + rationale。  
4. **并行名存实亡**：`RunParallelExplore` 为 no-op；TurnLoop 未调 Wave。  
5. **Session 无 deliverable**：`complete.Content=""`；用户仅见流式 planning 片段。

**谁受影响：** 所有 open-ended 指令（code review、架构调研、多模块 compare）的飞书用户与 Jaeger 运维。

## 2. Proposed Solution（分 Phase）

### Phase 1 — Rollup 闭环（P0，本 change 首批交付）

| 组件 | 做法 |
|------|------|
| **Parent Rollup Gate（Path A）** | `SpawnDecompose/await` 子树 terminal 后，`ShouldRollupAfterChildren(RollupGatePolicy)` 判定；父 **不** auto-complete；置 `NeedsRollup` + `pending` |
| **RollupGatePolicy** | `all_pass` / `best_effort`（Phase 1 默认）/ `min_coverage`；在 `ReevaluateParentAfterChild` 求值 |
| **Root Session Rollup Fallback（Path B）** | session 将关闭 && root goal && 存在 checklist 子项 && 未 rollup → 强制 `NeedsRollup`；**不对 checklist 跑 MUPS** |
| **GetFocus 跳过 ephemeral** | `GetReadyItems`/`GetFocus` 排除 `Ephemeral && WorkKindChecklist`；避免 trace 类 11 轮串行 MUPS |
| **Virtual Checklist Bubbles** | Rollup Observe 将 checklist `Directive` 作 `checklist_child_bubble` 注入（无需 promote 后跑子 MUPS） |
| **Summary Bubble Materialize** | 父 Rollup Observe **双 bubble**：Structured（强制）+ Summary（CB3 截断）；T05 S4 P0 闭合 |
| **Rollup Execute** | 父 Round 2+ PlanKind=`CommitmentPlan`；Directive 模板化 synthesize |
| **Session Deliver** | `RunSessionTurnLoop` 结束写 `complete.Content=root post-rollup ArtifactSummary` |

### Phase 2 — 受控 Decompose + Parallel（P1，同 change 文档登记、可拆 PR）

| 组件 | 做法 |
|------|------|
| **LLM DecomposeProposer** | R5/R6 时 `Propose` **先于** `ApplySpawnPolicy`；`ChildSpec[]` + `ExpectedReturn` + `LastRound.ChildSpecs` 审计 |
| **FailureCriteria 向下契约** | 父 Plan 模板 → 子 Directive/Plan → 子 Verify（Phase 2） |
| **RunParallelExplore** | 接 `WaveScheduler` + ephemeral probes（设计 D3） |
| **Plan 多 Step** | ExplorationPlan N Step → `ExplorationChannel` |

### Phase 3 — Verify 深化（P2，可选）

- Review 类 `FailureCriteria` 模板（章节/覆盖度）  
- 可选 LLM Verifier（已有 `VerificationAgent` 升格路径）

## 3. Capabilities（L4 映射）

| L4 ID | 名称 | Phase |
|-------|------|-------|
| D7-S1-A50 | `needs_rollup` 父节点状态与 Rollup Gate | 1 |
| D7-S1-A55 | `RollupGatePolicy` + `ShouldRollupAfterChildren` 门控 | 1 |
| D7-S2-A60 | 父 Round 2+ MUPS synthesize 编排 | 1 |
| D7-S2-A61 | Session `complete` deliverable 回填 | 1 |
| D7-S1-A51 | Summary + Structured 双 bubble Observe materialize（P0） | 1 |
| D7-S1-A52 | LLM DecomposeProposer + ChildSpec（含 ExpectedReturn）+ LastRound 审计 | 2 |
| D7-S2-A62 | RunParallelExplore → Wave | 2 |
| D7-S1-A53 | Ephemeral checklist 不参与 MUPS focus / session close 门禁 | 1 |
| D7-S1-A54 | Root Session Rollup Fallback + Virtual Checklist Bubbles | 1 |

## 4. Scope

### In Scope

- `resolve.go` 父 rollup 语义  
- `item_observe.go` summary bubble  
- `session_turn_loop.go` complete 回填 + rollup focus + Root Fallback 门控  
- `work_tree.go` GetFocus 排除 ephemeral checklist  
- `workitem.go` / schema：`NeedsRollup` 或等价 `RoundPhase`  
- `context_bubble_apply.go` Virtual Checklist Bubble 语句  
- Delta spec + L5 测试点 + 集成测试  
- 设计文档 `design.md` 现状/目标对比（本包）  
- **`review-r1.md` TurnLoop×WorkTree×MUPS 终审（S3）**

### Out of Scope

- D2 ContextScope 磁盘分区（ContextGraph F5）  
- 跨 Session 飞书 UI（TD-WT-04）  
- LLM Verifier 全量替换 deterministic verify（Phase 3）  
- `free_fork` D4 与 WorkTree 深度合并（单独 change）

## 5. Impact Analysis

| 组件 | 变更 | 详情 |
|------|------|------|
| workmodel | Yes | `NeedsRollup`、rollup 状态机、Observe bubble |
| sessionorchestrator | Yes | TurnLoop、Observe、complete |
| wavescheduler | Phase 2 | ItemPipeline 注入 Scheduler |
| D1 飞书 | Yes | 终局 summary 卡片有内容 |
| DB/schema | Yes | DiskWorkItemStore v2→v3 可选字段 |
| API | No | — |

## 6. Success Criteria

- [ ] Gherkin §9 场景「父节点在子 Pass 后 synthesize → completed」可自动化  
- [ ] Trace 级：同一父 `wi_*` 出现 ≥2 次 `D7_MUPS_Pipeline`  
- [ ] Trace 重放 fixture（`review d2` + todo_write checklist、spawn=none）：root 2× MUPS、无 checklist 串行 MUPS、complete 含 P0/P1 结构  
- [ ] `review d2` 类 E2E：complete summary 非空且非 planning monologue  
- [ ] 无回归：现有 spawn R0–R8 单测全绿  

## 7. Risks & Mitigations

| Risk | Prob | Impact | Mitigation |
|------|------|--------|------------|
| Rollup 双次 LLM 增 token | Med | Med | Summary bubble 截断；仅 direct children |
| 与 complete 空 Content hotfix 冲突 | Low | High | 区分 internal metadata vs deliverable |
| checklist 子项过多导致 rollup 延迟 | Med | Med | max rollup depth=1；Escape budget |
| Phase 2 Wave 接线面大 | Med | Med | 独立 PR + feature flag |

## 8. Open Questions（Review 时确认）

| OQ | 问题 | 决议（2026-06-27） |
|----|------|---------------------|
| OQ-1 | `NeedsRollup` 显式字段 vs 仅 SpawnPolicy | ✅ **显式 bool** + LastRound 审计 |
| OQ-2 | Rollup PlanKind | ✅ **复用 CommitmentPlan** + rollup FailureCriteria 模板 |
| OQ-3 | Phase 1 是否禁止 free_fork？ | ✅ **不禁止**；文档标「非 SoT」；Phase 2 收紧 |
| OQ-4 | checklist promote 时机 | ✅ Virtual Checklist Bubble；GetFocus 跳过 ephemeral |
| R1-V1 | Rollup verify 口径 | ✅ **IT stub LLM + 生产轻量 heuristic**（len≥500、P0/P1、planning 黑名单） |
| R1-V2 | Learn 以哪轮 Verdict 为准 | ✅ **Rollup 终局 Verdict**（Pass 覆盖 R1 Fail） |

## 9. Review Checklist（确认后进入 S4）

- [x] 同意 Phase 1/2 边界（含 Path B Root Fallback）  
- [x] 同意每层四问 + **§4.2 层级闭环模型**  
- [x] 同意 L5 测试点（spec_delta.md）  
- [x] OQ-4 拍板  
- [x] 同意 **review-r1.md** 终审结论与 S4 三条原则  
- [x] OQ-1～3 + R1-V1/V2 拍板（2026-06-27）  

---

## Archive Information

**Archived:** 2026-06-27  
**Outcome:** Phase 1 implemented — PARTIAL acceptance (unit PASS; trace E2E stub)  
**Verdict:** PARTIAL — see `acceptance-report.md`

### Code touched (S4 closure)

- `workmodel/rollup_gate.go` — `RollupGatePolicy`, `RollupGatePolicyFor`, `ShouldRollupAfterChildren`
- `workmodel/context_bubble_apply.go` — `CollectSummaryChildBubbles`
- `sessionorchestrator/item_observe.go` — `observationsFromChildSummaryBubbles`
- `tests/integration/d7/d7_rollup_trace_replay_test.go` — Path B stub IT

### Specs Updated

- `openspec/specs/d7-orchestration/spec.md` v4.13.0
- `openspec/specs/d7-orchestration/t-registry.md` v4.8.0
- `openspec/t-registry.md`

