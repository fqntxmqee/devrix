# Spec Delta — D7-Orchestration — Layer SubContext

**Change ID:** `devrix-d7-layer-subcontext`  
**Target Spec:** `openspec/specs/d7-orchestration/spec.md`  
**Target Version:** v4.13.0 → v4.14.0  
**Target Design:** `workitem-context-graph-design.md` v0.3.0 → **v0.4.0（CG2′ 修订）**  
**Demand ID:** DM-20260627-003  
**Created:** 2026-06-27  
**Status:** S3_Design — 待 Review

---

## MODIFIED Requirements

### CG2 — 默认隔离语义（Design Doc §2 CG2 → CG2′）

**原：** 子/sibling 上下文 **不自动** 合并；任何透传必须产生 `ContextLinkRecord`。

**修订为：**

- **Transcript 隔离：** 未经批准的 ContextLink，**不得**将其他 WorkItem 的 WorkItemPrivate ReAct 链注入 Materialize payload。  
- **Cohort 域共享：** 同一 Parent 下 sibling **共享 ScopeContract 与 cohort 元数据**（非 transcript）。  
- **Signal 透传：** Bubble、UpstreamSignal、PeerStatus、ContextLink 均为结构化 Signal，适用 CL0–CL8 / CB0–CB6。

#### Scenario: sibling share cohort domain not transcript (CG2′)

- **Given** parent `P` SpawnDecompose 创建 `wi_a` 与 `wi_b`，共享 cohort `cohort:<sid>:P`
- **And** `P` 的 ScopeContract 已写入 cohort meta
- **When** `Materialize(wi_a)` 与 `Materialize(wi_b)` 分别执行
- **Then** 两者 system prompt 均含相同 ScopeIn/Out（来自 ScopeContract）
- **And** `wi_a` 的 Messages **不含** `wi_b` 私有 jsonl 全文
- **And** `wi_b` 的 Messages **不含** `wi_a` 私有 jsonl 全文

---

## ADDED Requirements

本 delta 新增 **Scenario D7-S16（Layer SubContext）** 下 7 个 A 能力（Phase 1）与 2 个 A 能力（Phase 2 登记）。

### D7-S16-A60: ScopeContract — Goal 范围收敛

Before `SpawnDecompose` on an open-ended Goal WorkItem, the Plan phase shall produce a parseable **ScopeContract** containing at minimum: `goal_statement`, `in_scope`, `out_of_scope`, `assumptions`, `open_questions`, `success_criteria`.

#### Scenario: open questions block decompose

- **Given** Goal WorkItem `G` with user instruction requiring domain exploration (not single-file)
- **When** Plan outputs `ScopeContract.open_questions` with len ≥ 1
- **Then** `EvaluateSpawnPolicy` shall **not** return `SpawnDecompose`
- **And** SpawnPolicy shall be `SpawnInline` or user clarification path

#### Scenario: specific instruction rule-inferred scope skips extra LLM round

- **Given** user instruction matches single-file or single-function pattern (configurable regex)
- **When** Plan runs for Goal `G`
- **Then** ScopeContract may be rule-inferred without additional LLM scope round
- **And** `in_scope` shall contain the matched path

#### Scenario: ScopeContract flows to L1 ChildDownlink

- **Given** Goal `G` with valid ScopeContract and `SpawnDecompose`
- **When** children `C1`, `C2` are created
- **Then** each child's `ChildDownlink.ScopeIn/Out` shall reflect `G.ScopeContract`

---

### D7-S16-A61: ChildDownlink — 父→子下行契约

When ApplySpawnPolicy creates a non-ephemeral child WorkItem, the orchestrator shall persist a **ChildDownlink** containing: `Directive`, `ScopeIn`, `ScopeOut`, `ExpectedReturn`, `FailureCriteria`, and default `ContextPolicy=fresh`.

#### Scenario: child first Execute materializes ChildDownlink

- **Given** child `C` with persisted ChildDownlink from parent `P`
- **When** `WorkItemExecutor` runs Execute for `C` with `FeatureLayerSubContextEnabled=true`
- **Then** Materialize payload shall include ChildDownlink directive and scope paths in system or user messages
- **And** Execute append target shall be `wi:<sid>:C` only

#### Scenario: sandbox child forces private partition

- **Given** child `C` with non-empty `sandbox_slug`
- **When** Materialize runs
- **Then** policy shall force WorkItemPrivate partition
- **And** peer sibling private chains shall not be injected

---

### D7-S16-A62: LayerCohort Partition

Sibling WorkItems under the same parent shall register a shared **LayerCohort** partition keyed `cohort:<session_id>:<parent_wi_id>` for ScopeContract and signal metadata. ReAct transcript shall **not** be stored in cohort partition by default.

#### Scenario: cohort created on decompose

- **Given** parent `P` triggers SpawnDecompose with two children
- **When** children are persisted
- **Then** `EnsureCohortScope(session, P.ID)` shall exist
- **And** Goal ScopeContract shall be readable from cohort meta by all siblings

---

### D7-S16-A63: UpstreamSignal — BlockedBy 物化（Phase 2）

When WorkItem `B` has `BlockedBy` containing terminal WorkItem `A`, Materialize for `B` with mode `Upstream` shall inject `StructuredBubbleStatement(A)` and truncated `ArtifactSummary` only.

#### Scenario: upstream without private chain

- **Given** `wi_a` completed with private ReAct chain length N > 10
- **And** `wi_b.BlockedBy = [wi_a]`
- **When** Materialize(wi_b) with policy Upstream
- **Then** payload shall contain structured bubble for wi_a
- **And** payload shall **not** contain wi_a private jsonl messages verbatim

---

### D7-S16-A64: PeerStatusSignal — 并行 cohort（Phase 2，可选）

When SpawnParallelExplore creates sibling explore WorkItems, upon terminal status the orchestrator **may** append a **PeerStatusSignal** (`wi_id`, `verdict`, summary ≤ 240 chars) to cohort signals. Live ReAct tail sharing is **prohibited** by default.

#### Scenario: peer status only after terminal

- **Given** parallel siblings `A`, `B` under parent `P`
- **When** `A` reaches terminal
- **Then** optional PeerStatusSignal for A may be written to cohort signals
- **And** `B`'s in-flight Materialize shall not include A's live tool results

---

### D7-S16-A70: WorkItemExecutor Materialize 接线

With `FeatureLayerSubContextEnabled=true`, WorkItemExecutor shall obtain LLM messages exclusively via D2 `ContextMaterializer.Materialize` using resolved partition and policy. Session-level `ContextPreparer.Prepare(sessionID)` shall **not** be used for depth ≥ 1 WorkItems.

#### Scenario: sub workitem uses materialize not session prepare

- **Given** child WorkItem at depth ≥ 1
- **And** `FeatureLayerSubContextEnabled=true`
- **When** Execute phase runs
- **Then** `ContextMaterializer.Materialize` shall be invoked
- **And** Jaeger span `D2_Context_Materialize` shall record `wi_id`, `message_count`

#### Scenario: feature flag off preserves legacy path

- **Given** `FeatureLayerSubContextEnabled=false`
- **When** any WorkItem Execute runs
- **Then** behavior shall match pre-change `prepareContext` session Prepare path

#### Scenario: rollup uses RollupSynth policy

- **Given** parent `P` with `NeedsRollup=true`
- **When** rollup Execute runs with flag on
- **Then** Materialize policy mode shall be `RollupSynth`
- **And** child private chains shall not be loaded wholesale

---

### D7-S16-A71: ResolvePartitionForWorkItem

The orchestrator shall map WorkItem tree position to `ContextPartition`: Goal/root → Session + wi:goal; depth ≥ 1 → Cohort(parent) + wi:self; delegate agents → agent partition (Phase 3).

#### Scenario: partition resolution by depth

- **Given** WorkItem `G` at depth 0 (Goal)
- **Then** partition kind shall include SessionContext
- **Given** WorkItem `C` at depth 1 with parent `G`
- **Then** partition shall include Cohort(G) and WorkItemPrivate(C)

---

### D7-S16-A72: Signal→Observation 边界 — Observe 规则映射，Execute 禁止每轮 Obs  taxonomy

WorkItem MUPS shall maintain a strict boundary between **Execute outputs (Signal + transcript)** and **Observe outputs (Observation taxonomy)**. The four `ObservationKind` values (`ObsFact`, `ObsSignal`, `ObsDeviation`, `ObsUncertainty`) shall be produced primarily by **rule-based Observe mapping** into `UncertaintyReport`, not by requiring the LLM to self-label every Execute ReAct iteration.

#### Scenario: execute react transcript excludes observation taxonomy blocks

- **Given** child WorkItem `C` completes Execute with multiple ReAct iterations
- **And** `FeatureLayerSubContextEnabled=true`
- **When** WorkItemPrivate transcript for `wi:<sid>:C` is inspected
- **Then** messages shall **not** contain mandatory JSON/YAML blocks requiring `ObsFact|ObsSignal|ObsDeviation|ObsUncertainty` self-labeling per iteration
- **And** structured delivery blocks (`<conclusion>`, `<open_questions>`, `ScopeContract`) are allowed as soft guidance only

#### Scenario: scope contract open questions map to ObsUncertainty at Observe

- **Given** Goal WorkItem `G` with `ScopeContract.open_questions` len ≥ 1
- **When** `observeWorkItem(G)` runs
- **Then** `UncertaintyReport` shall contain at least one `ObsUncertainty` (CatBusiness)
- **And** `EvaluateSpawnPolicy` shall **not** return `SpawnDecompose`
- **And** mapping shall be rule-driven (R-OBS-1), not direct LLM Obs label passthrough

#### Scenario: scope contract complete maps to ObsFact at Observe

- **Given** Goal `G` with non-empty `in_scope`, empty `open_questions`
- **When** `observeWorkItem(G)` runs after ScopeContract persistence
- **Then** report shall include `ObsFact` with scope statement and evidence referencing `G.ID`
- **And** `ObsFact.Strength` shall be assigned by rule/prior, not copied from LLM self-declared Obs labels

#### Scenario: child bubble remains ObsFact path unchanged

- **Given** terminal child `C` with structured bubble under parent `P`
- **When** parent Observe runs (rollup or normal)
- **Then** `observationsFromChildStructuredBubbles` shall emit `ObsFact` as today (#262 compatible)
- **And** parent shall **not** ingest child WorkItemPrivate ReAct chain for Obs mapping

---

### D7-S16-A73: Execute Structured Delivery Template（非 Obs  taxonomy）

Materialize templates for WorkItem Execute shall optionally include **soft-structured delivery hints** (`<conclusion>`, `<open_questions>`) while explicitly instructing the model **not** to emit MUPS Observation taxonomy labels. Terminal `LastRound` and Goal `ScopeContract` remain the canonical Signal carriers.

#### Scenario: materialize template includes delivery hints not obs labels

- **Given** WorkItem Execute with LayerSubContext enabled
- **When** D2 Materialize builds system or user prompt
- **Then** prompt may include delivery hint section for `<conclusion>` / `<open_questions>`
- **And** prompt shall state that Obs* classification is performed by the system at Observe phase

#### Scenario: terminal lastround carries signals not uncertainty report

- **Given** WorkItem pipeline round completes Execute
- **When** `LastRound` is persisted
- **Then** `LastRound` may contain verdict, artifact summary, and scope fields
- **And** `LastRound` shall **not** be treated as authoritative `UncertaintyReport` without Observe mapping

---

## Cross-References

| Requirement | D2 Delta | Rollup (#262) |
|-------------|----------|---------------|
| D7-S16-A70 | D2-S16-A20 | RollupSynth aligns with DirectiveForItem |
| D7-S16-A63 | D2-S16-A20 Upstream mode | Observe bubbles unchanged |
| D7-S16-A72 | ScopeContract → ObsUncertainty (R-OBS-1) | SpawnPolicy gate |
| D7-S16-A73 | D2-S16-A20 Materialize templates | Execute delivery hints |
| CG2′ | D2-S16-A21 cohort store | — |

---

## Non-Goals (Explicit)

- Execute **per-iteration** mandatory ObsFact/ObsSignal/ObsDeviation/ObsUncertainty LLM self-labeling
- Observe LLM ObservationProposer full implementation (Phase 2 / PR-A4 registration)
- RunParallelExplore Wave scheduling (rollup Phase 2)
- LLM DecomposeProposer full path
- SubTurn brief/fork/full unification (Phase 3)
- Cross-session ContextGraph UI (TD-WT-04)
