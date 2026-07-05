# Proposal: MUPS semantics schema alignment

**Change ID:** `mups-semantics-schema-alignment`  
**Demand ID:** DM-20260705-003  
**Created:** 2026-07-05  
**Updated:** 2026-07-05 (→ S7_Archived)
**Status:** S7_Archived
**Demand:** [`demand.md`](demand.md)

---

## 1. Problem Statement

DM-20260705-001 delivered locale-neutral `SemanticsForPhase` but i18n still owns per-target prose keys (`observe.kind.*.when_use`). Input rules were duplicated in `PhaseSemantics.InputRules` instead of deriving from `LineFrameRegistry`.

## 2. Proposed Solution

Split semantics into three layers:

```text
SemanticCondition (machine codes, prompttags)
        ↓
SemanticRule + SemanticBlock(phase)  ← locale-neutral JSON-lines
        ↓
i18n glossary (condition → label) + node role  ← thin overlay
        ↓
RenderSemanticAppendix(phase, locale)
```

### Render format example

```json
{"target":"obs_uncertainty","plane":"data","when":"scope_unclear","when_not":"strong_fact_exists","enforced":false}
```

## 3. Capabilities

| ID | Capability | Owner |
|----|------------|-------|
| **shared-A97** | SemanticCondition + SemanticRule + SemanticBlock | `internal/shared/prompttags/` |
| **D2-S15-A97** | Glossary render + appendix assembly | `internal/layers/contextengine/i18n/` |

## 4. Migration

| Step | Action |
|------|--------|
| 1 | Add `semantic_condition.go`, `semantic_rule.go`, `semantic_block.go`, `prompt_plane.go` |
| 2 | Rewrite `semantics.go` — output rules only; input via `InputRulesForFrame` |
| 3 | Shrink `prompttags_semantics_{zh,en}.go` → glossary maps |
| 4 | Rewrite `prompttags_semantics_render.go` |
| 5 | Update tests + golden hashes |

## 5. Success metrics

- `go test ./internal/shared/prompttags/... ./internal/layers/contextengine/i18n/...` PASS
- i18n maps keyed by `SemanticCondition` only (no per-target prose keys)
- `RegisterFrameFieldGuide` init uses `InputRulesForFrame`
