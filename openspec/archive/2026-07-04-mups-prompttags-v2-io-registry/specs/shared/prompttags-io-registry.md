# Delta: shared/prompttags IO registry v2

**Change ID:** `mups-prompttags-v2-io-registry`  
**Demand:** DM-20260704-005  
**Base:** `openspec/specs/shared/prompttags.md`

---

## ADDED: Unified IO catalog

**Path:** `internal/shared/prompttags/registry.go`

### EncodingProfile extension

| Profile | Role |
|---------|------|
| `lineframe` | Bare `key: value` user prompt frames (Observe/Plan) |

### Registries

| Registry | Contents |
|----------|----------|
| `MUPSRegistry` | Envelope tags (unchanged API) |
| `LineFrameRegistry` | `observe_user` → `ObserveUserFrame`, `plan_user` → `PlanUserFrame` |
| `WholeBodyRegistry` | Observe proposals array, Plan strategic plan object |
| `MUPSIOCatalog` | Flat index of all shapes for parseability invariant |

### Line frames

| Frame | Fields (order fixed) |
|-------|---------------------|
| `ObserveUserFrame` | work_item_id, directive, prior_mean, scope_goal, scope_open_question, signal, prior_observation_ids, incremental_only |
| `PlanUserFrame` | work_item_id, directive, observation_ids, observation_summary, depth…, uncertainty_mean |

### Whole-body shapes

| Phase | Shape | Helper |
|-------|-------|--------|
| Observe | JSON array (max 3 elements enforced in Go) | `DocBlockObserveSchema()` |
| Plan | JSON object | `DocBlockPlanSchema()` |
| Execute | envelope tags | `ExecuteOutputTagDoc()` |

## ADDED: Convergence invariants

1. **Parseability** — each output shape has exactly one `EncodingProfile` in catalog
2. **Monotonic scope** — Observe→Plan→Execute scope tightening (reference: `scope_validate.go`)
3. **Observe max 3** — `ValidateObservationProposals` keeps first 3 valid proposals
4. **Plan budget** — reference `applyBudgetCap` (no change)
5. **Reject feedback loops** — P2 defer

## MODIFIED: Observe user frame (P1)

When `LastRound.ObservationIDs` present, user prompt includes `prior_observation_ids` and `incremental_only: true`.

## MODIFIED: Plan user prompt (P1)

`buildStrategicPlanUserPrompt` emits `uncertainty_mean` when `StrategicPlanInput.UncertaintyMean > 0`.
