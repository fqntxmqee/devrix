# Implementation Tasks: MUPS semantics schema alignment

**Change ID:** `mups-semantics-schema-alignment`  
**Demand ID:** DM-20260705-003  
**Status:** S4_Development

---

## Phase P0 — Schema alignment

| Task | L5 | Status | File |
|------|-----|--------|------|
| T1 `SemanticCondition` enum | — | [x] | `semantic_condition.go` |
| T2 `SemanticRule` + `MachineLine()` + `InputRulesForFrame()` | shared-A97 | [x] | `semantic_rule.go` |
| T3 `SemanticBlock(phase)` | shared-A97 | [x] | `semantic_block.go` |
| T4 Rewrite `semantics.go` — output rules only | shared-A97 | [x] | `semantics.go` |
| T5 Shrink i18n to glossary maps | D2-S15-A97 | [x] | `prompttags_semantics_{zh,en}.go` |
| T6 Rewrite `RenderSemanticAppendix` | D2-S15-A97 | [x] | `prompttags_semantics_render.go` |
| T7 Fix `prompttags_semantics_init.go` | — | [x] | uses `InputRulesForFrame` |
| T8 Update unit + golden tests | L5-MUPS-TAG-01..04 | [x] | `*_test.go` |

## Verification

```bash
go test ./internal/shared/prompttags/... ./internal/layers/contextengine/i18n/... -count=1
```

## S5 — Acceptance

- [ ] User acceptance per L5 test points
- [ ] Golden hash drift reviewed (expected — format change)

## S6 — Domain sync (post-ACCEPTED)

- [ ] Merge delta into `openspec/specs/shared/prompttags.md`
