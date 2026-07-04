# Delta: shared/prompttags semantics

**Change ID:** `mups-prompt-tag-semantics`  
**Demand:** DM-20260705-001  
**Base:** `openspec/specs/shared/prompttags.md`

---

## ADDED: Tag semantics layer (v3)

**Path:** `internal/shared/prompttags/semantics.go`

Extends IO catalog (DM-005) with **LLM-facing field/tag semantics**: when to use, data vs control plane, enforce alignment.

### PhaseSemantics

| Phase | Node role (LLM-visible) | Output semantics | Input semantics |
|-------|-------------------------|------------------|-----------------|
| Observe | Classify Obs* from structured signals; no tools | `obs_*` kind selection + field glossary | lineframe `[data]`/`[control]` fields |
| Plan | Propose execution strategy + deliverable contract | `execution_mode` decision tree + contract example | budget fields = control; obs summary = data |
| Execute | ReAct + deliverable; Obs taxonomy forbidden | envelope Required/Optional/HumanProse matrix | wiBody + instance tags = input contract |

### Observe kind semantics (normative)

| kind | When use | When not |
|------|----------|----------|
| `obs_uncertainty` | Scope unclear, open questions | Strong evidence for fact |
| `obs_fact` | Signal-backed statement | Speculation |
| `obs_signal` | Structured signal summary | Prefer uncertainty if unclear |
| `obs_deviation` | Expected vs observed delta | No baseline |

### Plan execution_mode semantics

- `single`: scope clear, low uncertainty, one-pass sufficient
- `decompose`: needs child WorkItems; respect budget fields in user frame
- `parallel_probe`: exploratory parallel paths

Go enforce: `uncertainty_mean ≥ 0.45` rejects `single` (CC-U4).

### Execute output semantics

| Artifact | Plane | Required when |
|----------|-------|---------------|
| `<deliverable_contract>` | control | contract applicable |
| findings JSON | data | structure=findings_json |
| `<open_questions>` | data | optional |
| `<conclusion>` | human prose | optional; not verified |

### Assembly

`RenderSemanticAppendix(phase, locale)` inserted **before** `DocBlock*` in phase appendix; schema syntax unchanged.

### Six-node terminology

Documentation and prompts use **六节点** (Observe → Plan → Execute → Verify → Learn → Decide). LLM prompts cover first three only.

## MODIFIED: Convergence invariants

Add invariant **6. Semantics-enforce alignment** — prompt claims with `Enforced: true` must match Go gates listed in design.md §6.
