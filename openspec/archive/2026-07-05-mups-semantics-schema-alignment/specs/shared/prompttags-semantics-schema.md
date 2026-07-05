# Delta: shared/prompttags semantics schema alignment

**Change ID:** `mups-semantics-schema-alignment`  
**Demand:** DM-20260705-003  
**Base:** `openspec/specs/shared/prompttags.md` § v3

---

## MODIFIED: Tag semantics layer (v3 → v3.1)

**Path:** `internal/shared/prompttags/`

### SemanticRule replaces FieldSemantic prose keys

| Field | Type | Notes |
|-------|------|-------|
| `Target` | string | tag name, kind, or field |
| `Plane` | `PromptPlane` | `data` \| `control` |
| `WhenUse` | `SemanticCondition` | machine code |
| `WhenNot` | `SemanticCondition` | optional |
| `Enforced` | bool | aligns with Go gate |
| `Gate` | string | gate function name when enforced |

`MachineLine()` emits compact JSON (ParseRejectRecord profile):

```json
{"target":"obs_uncertainty","plane":"data","when":"scope_unclear","when_not":"strong_fact_exists"}
```

### Input rules derived from LineFrameRegistry

- `InputRulesForFrame(frame)` walks `LineFrameRegistry[frame].Fields` in canonical order
- Per-tag `WhenUse` condition stored in `observeInputSemantics` / `planInputSemantics` maps
- `PhaseSemantics` no longer carries hand-maintained `InputRules` slice

### i18n overlay (D2)

- `semanticGlossary{ZH,EN}`: `SemanticCondition` → locale label
- `semanticNodeRole{ZH,EN}`: node role keys only
- `RenderSemanticAppendix`: node role + machine JSON-lines + condition glossary

### Assembly unchanged

`RenderSemanticAppendix(phase, locale)` still inserted **before** `DocBlock*` in phase appendix.
