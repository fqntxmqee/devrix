# Delta: D7 Orchestration — ResolutionContract + DecideBinding (Obs→Execution 统一契约)

**Change ID:** `devrix-d7-uncertainty-resolution-traceability`
**Demand ID:** DM-20260704-006
**Affects:** Orchestration 5 节点 (Observe→Plan→Execute→Verify→Decide) + WorkItemPipelineRound + i18n 引导词
**Design SoT:** [`../design.md`](../design.md)
**Depends on:**
- `d7-uncertainty-spawn-decouple` (DM-20260704-001) — U 驱动拓扑 CC-U1～U6
- `mups-semantics-schema-alignment` (DM-20260705-003) — locale-neutral 语义层
- `mups-node-prompt-dedup` (DM-20260705-004) — 三节点 prompt 净化
- `devrix-d7-taskcontract-unification` (DM-20260629-007/008) — TaskContract 平行不冲突

---

## ADDED

### Requirement: RC-1 Plan 输出 ResolutionStrategy[]

The Plan WorkItem MUST output a `ResolutionStrategy[]` array in its artifact, where each element binds to one `Observation.ObsID` with planned_tool, success_criterion, and **optional** `sub_worktree` (replacing the legacy `child_specs[]` field).

#### Scenario: Plan returns 3 strategies bound to 3 obs_uncertainty
- GIVEN an Observe WorkItem returning 3 `obs_uncertainty` observations (obs_u1/obs_u2/obs_u3 with strength 0.92/0.88/0.82)
- WHEN the Plan WorkItem's LLM completes
- THEN `ResolutionStrategy[]` length MUST equal 3
- AND each `ResolutionStrategy.ObsID` MUST be unique and align with one `Observation.ObsID`
- AND each `ResolutionStrategy.PlannedTool` MUST reference a tool registered in the tool registry
- AND `ResolutionStrategy.SubWorktree` MAY be nil (same-layer Execute handles) or non-nil (Decide spawns child WI)

#### Scenario: Plan LLM does not output ResolutionStrategy (fallback)
- GIVEN a Plan LLM outputting legacy `execution_mode: "decompose"` + `child_specs[]` without ResolutionStrategy
- WHEN the Decide WorkItem runs
- THEN the legacy R0-R8 logic MUST apply (`execution_mode` as hint)
- AND a warning log `resolution_strategy_missing_fallback_to_execution_mode` MUST be emitted

---

### Requirement: RC-2 Execute 输出 ResolutionClaim[]

The Execute WorkItem MUST output a `ResolutionClaim[]` array in its artifact, where each element binds to one `ResolutionStrategy.ObsID` with answer, confidence, and supporting_evidence.

#### Scenario: Execute produces 1 claim for obs_u3
- GIVEN a Plan with 3 ResolutionStrategy (obs_u1/obs_u2/obs_u3)
- AND the Execute tool `read_file rollup_directive.go` succeeds
- WHEN the Execute WorkItem's LLM completes
- THEN `ResolutionClaim[]` MUST contain at least 1 element for obs_u3
- AND `claim.ObsID` MUST equal "obs_u3"
- AND `claim.Confidence` MUST be in [0, 1]
- AND `claim.SupportingEvidence` MUST reference the tool output (file:line)

#### Scenario: Execute LLM does not output ResolutionClaim
- GIVEN a Plan with 3 ResolutionStrategy
- AND the Execute tool fails entirely (files_inspected = 0)
- WHEN the Execute WorkItem's LLM completes
- THEN `ResolutionClaim[]` MAY be empty
- AND Verify MUST compute `CoverageRatio = 0`
- AND Verify MUST emit `UnresolvedObs` with reason `no_resolution_claim`

---

### Requirement: RC-3 Verify 计算 ResolutionCoverage

The Verify WorkItem MUST compute `ResolutionReport` by comparing `ResolutionStrategy[]` from Plan against `ResolutionClaim[]` from Execute.

#### Scenario: Partial coverage with 2 unresolved obs
- GIVEN Plan has 3 ResolutionStrategy
- AND Execute produces 1 ResolutionClaim for obs_u3 (Confidence=0.85)
- WHEN Verify runs `verifyResolutionCoverage()`
- THEN `TotalStrategies = 3`, `TotalClaims = 1`, `CoverageRatio = 0.333`
- AND `UnresolvedObs` length MUST be 2 (obs_u1, obs_u2)
- AND each `UnresolvedObs.Reason` MUST be `no_resolution_claim`

#### Scenario: Full coverage with all claims confident
- GIVEN Plan has 3 ResolutionStrategy
- AND Execute produces 3 ResolutionClaim (all Confidence ≥ 0.7)
- WHEN Verify runs
- THEN `CoverageRatio = 1.0`
- AND `UnresolvedObs = []`
- AND `VerdictKind = VerdictPass`

#### Scenario: No claims at all
- GIVEN Plan has 3 ResolutionStrategy
- AND Execute produces 0 ResolutionClaim
- WHEN Verify runs
- THEN `CoverageRatio = 0`
- AND `VerdictKind = VerdictFail` or `VerdictPartial`
- AND `VerifyRationale` MUST contain "no resolution claims"

---

### Requirement: RC-4a Decide 强制 SpawnDecompose via sub_worktree

The Decide WorkItem MUST select `SpawnPolicy = SpawnDecompose` when `UnresolvedObs` contains at least one entry with `HasSubWorktree = true`, REGARDLESS of `DeliverableStatus`.

#### Scenario: 3 unresolved obs, all with sub_worktree
- GIVEN `UnresolvedObs = [obs_u1(HasSubWorktree=true, Strength=0.92), obs_u2(HasSubWorktree=true, Strength=0.88), obs_u3(HasSubWorktree=true, Strength=0.82)]`
- AND `DeliverableStatus = incomplete` (format issue)
- WHEN Decide runs
- THEN `SpawnPolicy = SpawnDecompose`
- AND `round.ChildSpecs` length MUST equal 3 (all unresolved with sub_worktree)
- AND the format incomplete MUST NOT cause fallback to SpawnInline

#### Scenario: 3 unresolved obs, only 1 with sub_worktree
- GIVEN `UnresolvedObs = [obs_u1(HasSubWorktree=false), obs_u2(HasSubWorktree=true), obs_u3(HasSubWorktree=false)]`
- WHEN Decide runs
- THEN `SpawnPolicy = SpawnDecompose`
- AND `round.ChildSpecs` length MUST equal 1 (only obs_u2)

#### Scenario: Budget exceeded fallback
- GIVEN `UnresolvedObs` with sub_worktree but depth >= MaxSubWorktreeDepth (default 3)
- WHEN Decide runs
- THEN `SpawnPolicy = SpawnInline` (fallback)
- AND a warning log `sub_worktree_budget_exceeded` MUST be emitted with depth/children/daily metrics

---

### Requirement: RC-4b Decide 强制 SpawnUserGate via 高强度无 sub_worktree

The Decide WorkItem MUST select `SpawnPolicy = SpawnUserGate` when ALL `UnresolvedObs` have `HasSubWorktree = false` AND `MaxUnresolvedStrength >= UnresolvedStrengthThreshold` (default 0.85).

#### Scenario: All unresolved, high strength, no sub_worktree
- GIVEN Plan has 3 ResolutionStrategy (all `SubWorktree == nil`)
- AND `UnresolvedObs = [obs_u1(Strength=0.92, HasSubWorktree=false), obs_u2(Strength=0.88, HasSubWorktree=false), obs_u3(Strength=0.82, HasSubWorktree=false)]`
- AND `MaxUnresolvedStrength = 0.92 >= 0.85`
- WHEN Decide runs
- THEN `SpawnPolicy = SpawnUserGate`
- AND the next WorkItem's `Prompt.Input.ToolFilter` MUST equal `[ask_user_question]`
- AND `Prompt.Input.UnresolvedObs` MUST equal the UnresolvedObs list

---

### Requirement: RC-4c Decide 沿用 SpawnInline (其它情况)

The Decide WorkItem MUST select `SpawnPolicy = SpawnInline` (existing R0-R8 logic) when ALL `UnresolvedObs` have `HasSubWorktree = false` AND `MaxUnresolvedStrength < UnresolvedStrengthThreshold`.

#### Scenario: All unresolved, low strength, no sub_worktree
- GIVEN `UnresolvedObs = [obs_u1(Strength=0.55, HasSubWorktree=false), obs_u2(Strength=0.42, HasSubWorktree=false)]`
- AND `MaxUnresolvedStrength = 0.55 < 0.85`
- WHEN Decide runs
- THEN `SpawnPolicy = SpawnInline` (existing logic applies)

---

### Requirement: RC-5 DecomposeFromSubWorktree 入口

The Decompose WorkItem MUST provide a `DecomposeFromSubWorktree` entry point that creates child WorkItems from `SubWorktreeSpec[]`.

#### Scenario: 3 sub_worktree → 3 child WI
- GIVEN Decide selects SpawnDecompose with `SubWorktreeSpec[]` of 3 items
- WHEN `DecomposeFromSubWorktree` runs
- THEN 3 child WorkItems MUST be created
- AND each `child.Directive` MUST equal `parent_directive + "\n\n" + sub_worktree.DirectiveSuffix`
- AND `child.ScopeIn` MUST equal `sub_worktree.ScopeIn`
- AND `child.Title` MUST equal `sub_worktree.Title`
- AND `child.ExpectedReturn` MUST equal `sub_worktree.ExpectedReturn`
- AND `child.PlannedTool` MUST equal `sub_worktree.PlannedTool`

---

### Requirement: RC-6 Type Stability 跨域类型上提

The 5 ResolutionContract types MUST be defined in `internal/layers/orchestration/orchtypes/resolution.go` as the single source of truth, importable by D7/D2 without import cycles.

#### Scenario: Type importable across D7 packages
- GIVEN `orchtypes/resolution.go` defines `ResolutionStrategy/SubWorktreeSpec/ResolutionClaim/ResolutionReport/UnresolvedObs`
- WHEN `mups/plan/plan.go` imports `orchtypes.ResolutionStrategy`
- AND `mups/execute/channel.go` imports `orchtypes.ResolutionClaim`
- AND `mups/verify/verify.go` imports `orchtypes.ResolutionReport`
- AND `workmodel/spawn_policy.go` imports `orchtypes.ResolutionReport`
- THEN all imports MUST succeed without circular dependency errors

---

## MODIFIED

### Modify: child_specs[] Field Compatibility

The legacy `child_specs[]` field on `StrategicPlanProposal` MUST remain for backward compatibility but MUST be marked as `// Deprecated: use ResolutionStrategy[].SubWorktree instead`. New code MUST NOT read this field directly.

#### Scenario: Existing strategic_plan_proposer parses child_specs
- GIVEN a Plan LLM outputs `execution_mode: "decompose"` + `child_specs[]` (legacy format)
- WHEN `StrategicPlanProposal.Validate()` runs
- THEN `child_specs[]` MUST still be parsed (legacy compat)
- AND a deprecation warning log MUST be emitted
- AND `ResolutionStrategy[]` MUST be empty (caller decides fallback path)

---

### Modify: detectUserGate Safety Net Retained

The existing `detectUserGate` text/regex-based user-gate detection in `verify_decision_table.go` MUST remain as a safety-net fallback when Plan LLM does not output ResolutionStrategy or when obs_uncertainty detection precedes the ResolutionContract rollout.

#### Scenario: Plan missing ResolutionStrategy, detectUserGate catches
- GIVEN Plan LLM outputs no `ResolutionStrategy[]`
- AND Execute artifact contains phrase "awaiting your guidance"
- WHEN Verify runs
- THEN `detectUserGate` MUST return `true` (safety net)
- AND the existing logic MUST apply (Partial verdict + SpawnUserGate path)

---

### Modify: StrategicPlanProposer sub_worktree Parsing

The `StrategicPlanProposal` parser MUST accept the new `sub_worktree` field within each strategy entry, while still accepting the legacy `child_specs[]` field.

#### Scenario: LLM outputs new format with sub_worktree
- GIVEN a Plan LLM JSON output containing:
  ```json
  {
    "resolution_strategies": [
      {"obs_id": "obs_u1", "planned_tool": "read_file", "sub_worktree": {"title": "读 plan 目录", "directive_suffix": "请读 plan/*.go", "scope_in": ["plan/"]}}
    ]
  }
  ```
- WHEN `StrategicPlanProposal.Validate()` runs
- THEN `len(ResolutionStrategy) = 1`
- AND `ResolutionStrategy[0].SubWorktree != nil`
- AND `ResolutionStrategy[0].ObsID = "obs_u1"`

---

## OBSOLETE (Pending Deprecation)

The following fields/behaviors are marked for future removal but retained for compatibility in v1.0:

| Field/Behavior | Status | Replacement | Removal Target |
|----------------|--------|-------------|----------------|
| `child_specs[]` (legacy) | Deprecated | `ResolutionStrategy[].SubWorktree` | v2.0 (next minor after v1.0) |
| `execution_mode: "decompose"` hint path | Deprecated (fallback) | `ResolutionStrategy[]` primary path | v2.0 (if LLM adoption > 95%) |
| `detectUserGate` text/regex | Retained (safety net) | `MaxUnresolvedStrength ≥ 0.85 + HasSubWorktree=false` | Never (safety net guarantee) |

---

## Cross-Cutting Concerns

### Span/Metric Contract

The following sessionSpan attributes MUST be emitted by their respective nodes:

| Node | Attribute | Type | Description |
|------|-----------|------|-------------|
| Plan | `d7.mups.resolution_strategy_count` | int | len(ResolutionStrategy) |
| Plan | `d7.mups.resolution_sub_worktree_count` | int | count of strategies with SubWorktree != nil |
| Execute | `d7.mups.resolution_claim_count` | int | len(ResolutionClaim) |
| Execute | `d7.mups.resolution_claim_max_confidence` | float64 | max(claim.Confidence) |
| Verify | `d7.mups.resolution_coverage` | float64 | TotalClaims/TotalStrategies |
| Verify | `d7.mups.resolution_unresolved_obs_count` | int | len(UnresolvedObs) |
| Verify | `d7.mups.resolution_unresolved_obs_max_strength` | float64 | max(unresolved.Strength) |
| Decide | `d7.spawn.from_resolution` | string | decision_action ∈ {decompose, user_gate, inline} |
| Decide | `d7.spawn.resolution_observation_count` | int | count of sub_worktree created |

### i18n Field Guide

The `format_hints_mups.go` MUST add a new `ResolutionStrategy` and `ResolutionClaim` field guide for both `LocaleZH` and `LocaleEN`:

**ZH:**
```
resolution_strategies: 每个元素必须包含 obs_id（与 Observation.ObsID 对齐）+ planned_tool（tool registry 中的 tool name）+ success_criterion（自由文本）+ 可选 sub_worktree（如需独立子 WI 消解该 obs）
  sub_worktree: { title: 子 WI 标题; directive_suffix: 拼到 parent directive 后; scope_in: 子 WI scope; expected_return: 子 WI 期望产出; planned_tool: 子 WI 主要 tool }
resolution_claims: 每个元素必须包含 obs_id（与 Strategy.ObsID 对齐）+ answer（自由文本或 JSON）+ confidence ∈ [0, 1] + supporting_evidence（引用 tool output 或文件:行号）
```

**EN:**
```
resolution_strategies: each element MUST contain obs_id (align with Observation.ObsID) + planned_tool (tool name in tool registry) + success_criterion (free text) + optional sub_worktree (if separate child WI needed to resolve this obs)
  sub_worktree: { title: child WI title; directive_suffix: appended to parent directive; scope_in: child WI scope; expected_return: child WI expected output; planned_tool: child WI primary tool }
resolution_claims: each element MUST contain obs_id (align with Strategy.ObsID) + answer (free text or JSON) + confidence ∈ [0, 1] + supporting_evidence (cite tool output or file:line)
```

### Backward Compatibility Matrix

| Plan LLM Output Format | Decide Path | L5 Test |
|------------------------|-------------|---------|
| **New:** `ResolutionStrategy[]` + `sub_worktree` | **Primary (RC-4a/b/c)** | RT-01..RT-13 |
| **Old:** `execution_mode: "decompose"` + `child_specs[]` | Legacy R0-R8 fallback | RT-14 |
| **Empty:** No Plan output | Legacy R0-R8 + `detectUserGate` fallback | RT-15 |
| **Mixed:** new + old fields both present | Primary path (new wins) | RT-01 + RT-14 |

### Feature Flag

`resolution_contract_v1` FeatureFlag controls the rollout:
- `OFF` (default in v1.0 release): Only legacy `execution_mode + child_specs[]` path; `ResolutionStrategy[]` field parsed but not used by Decide
- `ON` (staged rollout post-v1.0): RC-4a/b/c logic active; legacy path retained as fallback

---

## Acceptance Criteria Mapping

| L5 ID | Description | Maps to Requirement |
|-------|-------------|---------------------|
| L5-D7-RT-01 | Plan outputs ResolutionStrategy[] length = obs count | RC-1 |
| L5-D7-RT-02 | Execute outputs ResolutionClaim[] for matched obs | RC-2 |
| L5-D7-RT-03 | Verify CoverageRatio + UnresolvedObs | RC-3 |
| L5-D7-RT-05 | All claims confident → VerdictPass | RC-3 |
| L5-D7-RT-06 | No claims → VerdictFail/Partial + rationale | RC-3 |
| L5-D7-RT-07 | Plan missing strategy → detectUserGate safety net | RC-5 safety net |
| L5-D7-RT-08 | sub_worktree forces SpawnDecompose (治本断链 B) | RC-4a |
| L5-D7-RT-09 | Partial sub_worktree → partial decompose | RC-4a |
| L5-D7-RT-10 | No sub_worktree + high strength → SpawnUserGate | RC-4b |
| L5-D7-RT-11 | No sub_worktree + low strength → SpawnInline (沿用) | RC-4c |
| L5-D7-RT-12 | sub_worktree creates child WI with parent+suffix directive | RC-5 |
| L5-D7-RT-13 | Child rollup merges ResolutionClaim to parent asset | (backlog v1.1) |
| L5-D7-RT-14 | Old format execution_mode + child_specs fallback | RC-5 safety net |
| L5-D7-RT-15 | No Plan output → legacy + detectUserGate fallback | RC-5 safety net |
| L5-D7-RT-16 | sub_worktree budget exceeded → SpawnInline + warning | RC-4a budget gate |

**Total:** 16 L5 test points (11 P0 + 5 P1) — see `tasks.md` for T-layer decomposition.