# Delta: shared/prompttags

**Change ID:** `mups-prompttags`  
**Demand:** DM-20260704-004

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
