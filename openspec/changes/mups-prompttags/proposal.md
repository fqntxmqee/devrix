# Proposal: MUPS prompttags framework

**Change ID:** `mups-prompttags`  
**Demand ID:** DM-20260704-004  
**Created:** 2026-07-04  
**Status:** S4_Implementation  
**Demand:** [`demand.md`](demand.md)

---

## 1. Problem Statement

MUPS envelope tags are serialized and parsed ad hoc across D2 (`phase_prompts.go`) and D7 (`workmodel`). Manual JSON in `scopeContractBlock` produces invalid `out_of_scope` encoding. No shared registry exists for tag names, encoding profiles, or phase-aware extraction.

## 2. Proposed Solution

Introduce `internal/shared/prompttags/` with:

| Component | Responsibility |
|-----------|----------------|
| `TagSpec` + `MUPSRegistry` | Tag names, `EncodingProfile` (envelope / linefield / wholebody) |
| `Wrap[T]` / `ExtractOne[T]` | Generic envelope serialize/deserialize |
| `ExtractAll` | Scan content for all registered envelope tags |
| `ParseWholeBody[T]` | Fence-stripped JSON/array parse (foundation for P3) |

P0 migrates four call-site clusters while keeping workmodel public APIs as thin wrappers.

## 3. Capabilities

| Capability | Layer | Package |
|------------|-------|---------|
| Tag registry | L4 shared | `internal/shared/prompttags` |
| Execute output hints wrap | L4 D2 | `materialize/phase_prompts.go` |
| Deliverable contract tag | L4 D7 | `workmodel/deliverable_contract.go` |
| Scope contract parse | L4 D7 | `workmodel/scope_contract_parse.go` |

## 4. Migration plan

| Phase | Scope |
|-------|-------|
| **P0** | Registry + envelope API + 4 call-site clusters |
| **P1** | Observe/Plan user prompt builders |
| **P2** | i18n appendix DocBlock |
| **P3** | wholebody adoption in deliverable parse paths |

## 5. Non-goals (P0)

- Observe/Plan prompt migration
- i18n DocBlock migration
- t-registry registration (defer to S5)
