# Design: MUPS prompttags framework

**Change ID:** `mups-prompttags`  
**Demand ID:** DM-20260704-004  
**Status:** S3_Design  
**Demand:** [`demand.md`](demand.md)  
**Proposal:** [`proposal.md`](proposal.md)

---

## 1. Architecture

```
internal/shared/prompttags/
├── registry.go    # TagName, TagSpec, EncodingProfile, MUPSRegistry
├── envelope.go    # Wrap[T], ExtractOne[T], ExtractAll
└── wholebody.go   # ParseWholeBody[T] (fence strip + json.Unmarshal)
```

### 1.1 EncodingProfile

| Profile | Format | Tags (P0) |
|---------|--------|-----------|
| `envelope` | `<tag>payload</tag>` | scope_contract, deliverable_contract, deliverable_schema, prior_verify_reason |
| `linefield` | `<tag>line\nline</tag>` | open_questions |
| `wholebody` | bare `{...}` / `[...]` or fenced | reserved P3 |

### 1.2 TagSpec registry

```go
var MUPSRegistry = map[TagName]TagSpec{
    TagScopeContract:       {Profile: EncodingEnvelope},
    TagDeliverableContract: {Profile: EncodingEnvelope},
    TagDeliverableSchema:   {Profile: EncodingEnvelope}, // scalar text
    TagPriorVerifyReason:   {Profile: EncodingEnvelope}, // scalar text
    TagOpenQuestions:       {Profile: EncodingLineField},
}
```

JSON-typed tags use `json.Marshal` / `json.Unmarshal` inside `Wrap` / `ExtractOne`. Scalar string tags pass through trimmed text.

### 1.3 API

```go
func Wrap[T any](name TagName, v T) string
func ExtractOne[T any](name TagName, content string) (T, bool)
func ExtractAll(content string, phase string) map[TagName]string
func ParseWholeBody[T any](content string) (T, bool)
```

`ExtractAll` accepts `phase` for future phase-filtered extraction (P1); P0 scans all envelope tags.

## 2. Call-site migration (P0)

| File | Before | After |
|------|--------|-------|
| `phase_prompts.go` | manual `scopeContractBlock` + fmt.Sprintf tags | `prompttags.Wrap` |
| `deliverable_contract.go` | string concat + Index parse | `Wrap` / `ExtractOne` |
| `expected_return.go` | manual deliverable_schema tag | `prompttags.Wrap` |
| `scope_contract_parse.go` | regexp + json.Unmarshal | `ExtractOne[ScopeContract]` |

Public workmodel functions remain; bodies delegate to prompttags.

## 3. Bug fix

`scopeContractBlock` encoded `out_of_scope` as `"out_of_scope":"a,b"` instead of JSON array. P0 uses `json.Marshal(contracts.MUPSScopeContract)` via `Wrap`.

## 4. Testing

- Round-trip golden tests per envelope tag in `prompttags/envelope_test.go`
- `ParseWholeBody` fence/bare JSON tests in `wholebody_test.go`
- Existing workmodel/materialize tests must remain green

## 5. P1–P3 roadmap

| Phase | Work |
|-------|------|
| P1 | `item_observe.go`, `strategic_plan_proposer.go` user prompt tag builders |
| P2 | `i18n/format_hints_mups.go` DocBlock tags |
| P3 | Replace ad-hoc JSON extraction with `ParseWholeBody` in deliverable parse |
