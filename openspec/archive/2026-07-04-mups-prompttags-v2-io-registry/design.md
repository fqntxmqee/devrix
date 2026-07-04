# Design: MUPS prompttags v2 IO registry

**Change ID:** `mups-prompttags-v2-io-registry`  
**Demand ID:** DM-20260704-005  
**Status:** S3_Design  
**Demand:** [`demand.md`](demand.md)  
**Proposal:** [`proposal.md`](proposal.md)

---

## 1. IO Registry architecture

```
internal/shared/prompttags/
├── registry.go      # MUPSRegistry (envelope) + MUPSIOCatalog + LineFrameRegistry
├── linefield.go     # FrameSpec, ObserveUserFrame, PlanUserFrame, BuildLineFrame
├── envelope.go      # Wrap / ExtractOne / ExtractAll
├── wholebody.go     # ParseWholeBody
└── docblock.go      # DocBlock* schema helpers
```

### 1.1 EncodingProfile (extended)

| Profile | Format | Examples |
|---------|--------|----------|
| `envelope` | `<tag>payload</tag>` | scope_contract, deliverable_contract, … |
| `linefield` | `<tag>line\nline</tag>` | open_questions (inside envelope) |
| `lineframe` | bare `key: value\n` lines | ObserveUserFrame, PlanUserFrame |
| `wholebody` | bare `{...}` / `[...]` or fenced | Observe proposals array, Plan JSON object |

`MUPSRegistry` unchanged for envelope tags. New `LineFrameRegistry` and `WholeBodyRegistry` document non-envelope shapes without breaking `Lookup` / `Wrap`.

### 1.2 MUPSIOCatalog

Flat list of `IOShapeEntry{Name, Profile, Phases}` built from the three registries for introspection and tests. One profile per parse target (parseability invariant).

## 2. Observe max-3 cap

**Choice:** keep **first 3 valid** proposals in LLM return order.

Rationale:
- Matches streaming/parse order (no re-sort by strength)
- Invalid proposals are skipped before counting (existing G3 gate)
- Aligns with i18n `format_hints_mups.go` "Maximum 3 proposals"

```go
const maxObservationProposals = 3

func ValidateObservationProposals(...) {
    for _, p := range proposals {
        o, err := validateOneProposal(...)
        if err != nil { continue }
        out = append(out, o)
        if len(out) >= maxObservationProposals { break }
    }
}
```

## 3. Plan uncertainty_mean

`PlanUserFrame` already lists `TagUncertaintyMean` last. `buildStrategicPlanUserPrompt` adds:

```go
if in.UncertaintyMean > 0 {
    fields[prompttags.TagUncertaintyMean] = in.UncertaintyMean
}
```

Same `> 0` guard as `TagPriorMean` in Observe frame.

## 4. Observe incremental frame

When `WorkItem.LastRound.ObservationIDs` is non-empty:

| Field | Value |
|-------|-------|
| `prior_observation_ids` | comma-joined IDs from last round |
| `incremental_only` | `true` |

Appended to `ObserveUserFrame` after `signal`. LLM appendix rules already say not to invent signals; frame fields make prior round explicit.

## 5. Convergence invariants (reference)

| Invariant | Owner |
|-----------|-------|
| Monotonic scope Observe→Plan→Execute | `workmodel/scope_validate.go`, Plan `ScopeIn` subset |
| Plan budget gates | `applyBudgetCap`, `DivergenceBudget` |
| Reject feedback loops | P2 defer |

## 6. Testing

- `registry_test.go`: catalog covers all profiles; `LookupLineFrame` round-trip
- `observation_proposer_test.go`: 5 proposals → 3 obs; 4th valid dropped
- `strategic_plan_proposer_test.go`: uncertainty_mean present when wired
- `linefield_test.go`: incremental fields in Observe frame golden
