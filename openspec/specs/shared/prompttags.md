# shared/prompttags

**Demand:** DM-20260704-004, DM-20260704-005, DM-20260705-001, DM-20260705-002  
**Change ID:** `mups-prompttags` (archived 2026-07-04), `mups-prompttags-v2-io-registry` (archived 2026-07-04), `mups-prompt-tag-semantics` (archived 2026-07-05), `mups-parse-reject-feedback` (archived 2026-07-05)

---

## ADDED: L4 prompttags package

**Path:** `internal/shared/prompttags/`

Central registry and generic API for MUPS machine-readable prompt tags.

### Tag registry (P0)

| Tag | Profile | Payload type |
|-----|---------|--------------|
| `scope_contract` | envelope | JSON → ScopeContract / MUPSScopeContract |
| `deliverable_contract` | envelope | JSON → DeliverableContract |
| `deliverable_schema` | envelope | scalar text (legacy) |
| `prior_verify_reason` | envelope | scalar text |
| `open_questions` | linefield | newline-separated lines |

### Whole-body response shapes (P3, not envelope tags)

| Phase | Shape | Helper |
|-------|-------|--------|
| Observe | JSON array of observation proposals | `DocBlockObserveSchema()` |
| Plan | JSON object strategic plan | `DocBlockPlanSchema(contractExample)` |

### API contract

- `Wrap[T](name, v)` returns empty string for zero/empty payloads
- `ExtractOne[T](name, content)` uses `(?s)<tag>(.*?)</tag>` compatible regex
- `ExtractAll(content, phase)` scans envelope tags; non-empty `phase` filters via `TagAppliesToPhase`
- JSON tags MUST use `encoding/json` (no manual field concatenation)
- `ParseWholeBody[T]` strips optional ` ```json ` fences before unmarshal
- `ExecuteOutputTagDoc()` / `DocBlock(phase)` supply machine tag syntax only (no locale prose)

### DocBlock integration (P2)

- D2 `WorkItemExecuteOutputHints` composes locale header/footer + `ExecuteOutputTagDoc()`
- D2 Observe appendix injects `DocBlockObserveSchema()` as JSON element shape line
- D2 Plan appendix injects `DocBlockPlanSchema()` as JSON object shape line
- Locale-specific tactical rules remain in i18n constants

### ParseWholeBody consumers (P3)

- D7 `parseObservationProposalsJSON` — `ParseWholeBody[[]rawObsProposal]`
- D7 `parseStrategicPlanJSON` — `ParseWholeBody[rawStrategicPlan]`
- D7 `tryParseWholeBodyFindingsObject` — fast path for bare/fenced findings JSON; specialized marker logic retained for corrupt summaries

### LineField user frames (P1)

- `ObserveUserFrame` / `PlanUserFrame` + `BuildLineFrame` for Observe/Plan user prompts

### Consumers (P0)

- D2 `BuildExecuteOutputHints` writes envelope tags via `Wrap`
- D7 `DeliverableContractTag`, `ParseScopeContractBlock`, `DeliverableSchemaTag` delegate to prompttags

### Migration phases

| Phase | Scope | Status |
|-------|-------|--------|
| P0 | Registry + envelope + 4 call-site clusters | DONE |
| P1 | Observe/Plan prompt builders | DONE |
| P2 | i18n DocBlock | DONE |
| P3 | wholebody deliverable parse | DONE |

### MUPS system prompt assembly order

`AssembleMUPSSystemPrompt(staticBase, outputHints, workItemBody, phaseAppendix)` concatenates in order:

1. **outputHints** — Execute deliverable tags / machine I/O hints
2. **workItemBody** — dynamic WorkItem directive body
3. **phaseAppendix** — Observe/Plan phase JSON schema appendix
4. **staticBase** — PrepareBase / devrix_core static system prompt

Rationale: node-specific dynamic content precedes static base so the model sees task context before global rules.

---

## v2: Unified IO catalog (DM-20260704-005)

**Path:** `internal/shared/prompttags/registry.go` (extended)

### EncodingProfile

| Profile | Format | Role |
|---------|--------|------|
| `envelope` | `<tag>payload</tag>` | Execute input/output machine contracts |
| `linefield` | lines inside envelope | `open_questions` payload |
| `lineframe` | bare `key: value` lines | Observe/Plan user prompt frames |
| `wholebody` | bare JSON / fenced JSON | Observe/Plan LLM output |

### Registries

| Registry | Contents |
|----------|----------|
| `MUPSRegistry` | Envelope tags (unchanged API) |
| `LineFrameRegistry` | `observe_user` → `ObserveUserFrame`, `plan_user` → `PlanUserFrame` |
| `WholeBodyRegistry` | Observe proposals array, Plan strategic plan object |
| `MUPSIOCatalog` | Flat index of all I/O shapes (parseability invariant) |

### Line frames (user input)

| Frame | Fields (fixed order) |
|-------|---------------------|
| `ObserveUserFrame` | work_item_id, directive, **prior_parse_reject**, prior_mean, scope_goal, scope_open_question, signal, prior_observation_ids, incremental_only |
| `PlanUserFrame` | work_item_id, directive, **prior_parse_reject**, observation_ids, observation_summary, depth, max_depth, existing_children, remaining_children, max_children, decompose_used_today, remaining_daily, max_daily, max_iters, parent_scope_in, uncertainty_mean |

### Convergence invariants

1. **Parseability** — each LLM output shape has exactly one profile in `MUPSIOCatalog`
2. **Monotonic scope** — Observe → Plan → Execute scope tightening (see `scope_validate.go`)
3. **Observe max 3** — `ValidateObservationProposals` keeps first 3 valid proposals (matches prompt)
4. **Plan budget** — `applyBudgetCap` / `applySingleModeUncertaintyGate` (unchanged)
5. **Parse-reject round-trip** — Observe/Plan reject records on round N appear in matching phase `prior_parse_reject` user frame on round N+1 (`WorkItemPipelineRound.ObserveParseReject` / `PlanParseReject`)
6. **Semantics-enforce alignment** — prompt claims with `Enforced: true` in `TagSemanticsRegistry` must match Go gates (`ValidateObservationProposals`, `applySingleModeUncertaintyGate`, `applyBudgetCap`, `VerifyDeliverableContract`)

### v3: Tag semantics layer (DM-20260705-001)

**Path:** `internal/shared/prompttags/semantics.go` + `internal/layers/contextengine/i18n/prompttags_semantics_{zh,en}.go`

- `SemanticsForPhase` — locale-neutral registry (`FieldSemantic`, `PhaseSemantics`)
- `RenderSemanticAppendix(phase, locale)` — i18n bullet appendix inserted **before** `DocBlock*` in Observe/Plan/Execute prompts
- `RenderFrameFieldGuideForFields` — compact field guide for Observe/Plan user frames (present fields only)
- `BuildLineFrameFromStruct` — plain `key: value` lines (no plane prefix; DM-20260705-004)
- `BuildAnnotatedLineFrame` — optional plane prefix (internal/tests only)

See `openspec/archive/2026-07-05-mups-prompt-tag-semantics/specs/shared/prompttags-semantics.md` for normative kind/mode semantics.

### v4: Parse reject cross-round feedback (DM-20260705-002)

**Path:** `internal/shared/prompttags/parse_reject.go` + D7 `item_pipeline.go` + D2 lineframe inject

- `ParseRejectRecord` — compact JSON (`phase`, `code`, `field`, `message`, `requested`, `max_allowed`, `snippet`)
- `TagPriorParseReject` — control-plane lineframe field (after `directive`) on Observe/Plan user frames
- Round persistence: `ObserveParseReject` / `PlanParseReject` on `WorkItemPipelineRound`
- Codes: `parse_fail`, `budget_cap`, `uncertainty_gate`, `scope_gate`, `validate_empty`
- Execute retry unchanged (`PriorVerifyReason` / `machineSpawnFeedback`); no same-round LLM parse retry

### v2 call-site changes

- Observe user frame: when `LastRound.ObservationIDs` present → `prior_observation_ids` + `incremental_only: true`
- Plan user prompt: emits `uncertainty_mean` when `StrategicPlanInput.UncertaintyMean > 0`

---

## MUPS 三节点 LLM 协议参考（2026-07-05）

**Path:** `openspec/specs/shared/mups-node-llm-protocols.md`

Normative reference for Observe / Plan / Execute:

- Input protocol (lineframe fields, user message structure)
- Output protocol (wholebody JSON vs envelope tags)
- System dynamic prompt assembly (`AssembleMUPSSystemPrompt` order per phase)
- Cross-round `ParseRejectRecord` feedback
- §8 optimization review (known gaps OPT-01..OPT-09)

Cross-links: D2 `MaterializeForMUPS` — `openspec/specs/d2-context-engine/d7-boundary.md` § MaterializeForMUPS table.

### v5: Prompt dedup (DM-20260705-004)

**Change ID:** `mups-node-prompt-dedup`

- Observe LLM user frame: classifier-visible fields only (`observeLLMFieldMap`); no `work_item_id` / `prior_mean` / `incremental_only`
- Lineframe: no `[control]`/`[data]` line prefixes; field guide lists present fields only
- Observe/Plan appendix: no duplicate `observe.node_role` / `plan.node_role` in `RenderSemanticAppendix`
- Execute: `AssembleMUPSExecuteSystemPrompt` order = workItemBody → outputHints → staticBase; ZH `任务指令` label
