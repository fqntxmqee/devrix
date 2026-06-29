# D3 LLM Gateway — Dead Code & Unused Export Audit (DM-20260629-003 PR-1)

**Status:** COMPLETED 2026-06-29
**Auditor:** DSAFT Review (2026-06-29 session)
**Scope:** 57 Go files / 3995 LOC across 9 sub-packages

---

## 1. Methodology

1. List all exported `func` and `type` declarations
2. Cross-reference each with internal + external usage
3. Mark dead/unused/under-used exports
4. Decide: keep (in use) / unexport (internal only) / delete (truly dead)

---

## 2. Audit Results

### 2.1 Dead Code LOC

| Category | Count | Action |
|----------|-------|--------|
| `legacy/` directory | 0 | N/A (D3 never had legacy/) |
| Unused exported `func` | 0 | — |
| Unused exported `type` | 0 | — |
| Dead span ops | 0 | 5 active ops all in production emit (R1 Q3 runtime stability) |
| **Total dead code LOC** | **0** | — |

### 2.2 Cross-Package Export Audit

| Export | Package | Internal Users | External Users | Status |
|--------|---------|----------------|----------------|--------|
| `APIError` + `NewAPIError` | `llmgateway` (root) | `stream/`, `stream/adapter/` | `bridges/llm/`, `cmd/` | ✅ KEEP (V4 closed-set, DM-20260628-001) |
| `Request` / `Chunk` / `ToolSchema` | `llmgateway` (root) | `stream/`, `protect/`, `route/`, `budget/`, `guard/`, `configure/` | `bridges/llm/`, `cmd/`, tests | ✅ KEEP (cross-cutting types) |
| `IGateway` / `ILLMGateway` / `ITierResolver` / `IAdapter` | `llmgateway` (root) | — | `bridges/llm/`, `internal/layers/communication/` | ✅ KEEP (D2/D7 contracts) |
| `ICircuitBreaker` + observer | `llmgateway` (root) | `protect/`, `stream/` | `bridges/llm/` | ✅ KEEP (D7 consumer) |
| `Gateway` + `New` + `NewFromConfig` | `stream/` | — | `bridges/llm/wire.go`, `cmd/devrix/main.go` | ✅ KEEP (primary entry) |
| `Gateway.TokenCounter()` | `stream/` | — | `bridges/llm/wire.go` | ✅ KEEP (D2 token counter source) |
| `Router` + `Resolve` + `ResolveTier` | `route/` | — | `stream/gateway.go` | ✅ KEEP |
| `Counter` + `NewCounter` | `budget/` | `stream/`, `routes/` | `bridges/llm/` | ✅ KEEP (BPE token counting) |
| `Filter` + `Check` | `guard/` | `stream/` | `bridges/llm/` | ✅ KEEP (S5 content safety) |
| `CircuitBreaker` + `Executor` | `protect/` | `stream/`, `bridges/llm/` | `bridges/llm/` | ✅ KEEP (S3) |
| `LLMGatewayConfig` + `DefaultLLMGatewayConfig` | `configure/` | `route/`, `stream/`, `cmd/` | `cmd/`, `bridges/llm/` | ✅ KEEP (S6) |
| `ModelCatalog` + `Lookup` | `configure/` | `route/`, `stream/` | (catalog used in routing) | ✅ KEEP |

### 2.3 Under-Used Types (could be unexported)

| Type | File | Reason Kept Exported |
|------|------|----------------------|
| `Match` (in guard) | `guard/filter.go` | Returned via `Result.Matches` field, callers may inspect |
| `Pattern` (in guard) | `guard/filter.go` | `Filter.Patterns()` exposes them |
| `Action` (in guard) | `guard/filter.go` | Used in `Match.Action` field for type safety |
| `Clock` (in protect) | `protect/circuit_breaker.go` | `WithClock` is the test injection point |
| `PublishBreakerStateDefault` | `protect/breaker_observer.go` | Default no-op publisher, used in tests |

**Decision**: KEEP all under-used types exported — they are part of the public API surface and unexporting would be a BREAKING change. Audit artifacts only, no deletions.

### 2.4 Span Ops Audit (R1 Q3 Runtime Stability)

| Span Op | Used In | Status |
|---------|---------|--------|
| `D3_LLM_Stream` | `stream/gateway.go` (root span) | ✅ ACTIVE |
| `D3_LLM_Provider_Route` | `stream/gateway.go` (routing span) | ✅ ACTIVE |
| `D3_LLM_CircuitBreaker` | `stream/gateway.go` (breaker span) | ✅ ACTIVE |
| `D3_LLM_Retry` | `stream/gateway.go` (retry span) | ✅ ACTIVE |
| `D3_LLM_Adapter_Stream` | `stream/gateway.go` (adapter span) | ✅ ACTIVE |

**Decision**: 0 dead span ops. R1 Q3 决议保持 runtime 字面量稳定性。

---

## 3. Conclusion

**0 dead code LOC / 0 unused exports / 0 dead span ops** in D3 domain.

D3 v1.0.0 already maintains a clean state, no P5 Deprecated gate to bypass (unlike D2 v8.2.0 facade/ → legacy/ cleanup). PR-1 audit-only — no file deletions required.

The 6 子 Change refactoring can proceed without legacy-cleanup blockers. Focus shifts to:
- PR-2/3: god fn split (Stream() 235 LOC + configure + errorclass)
- PR-4: F path 4 drift fixes
- PR-5: ValueFlow Alias
- PR-6: T↔Span Evidence coverage
- PR-7: 0 boundary-debt decision

---

**Revision History**

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-29 | Initial audit (DM-20260629-003 PR-1) — 0 dead code / 0 unused / 0 dead ops |
