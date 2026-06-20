# Devrix Error Handling Design

**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-20
**Change:** devrix-error-handling-tier1-tier2 (DM-20260620-003)
**Scope:** All layers D1/D2/D3/D4/D5/D7 + shared/errors

---

## 1. Overview

Devrix error handling has 3 layers, each addressing a different concern:

1. **Construction** — `*SentinelError{Code, Message, Err}` wraps all typed errors
2. **Propagation** — `%w` verb preserves the chain for `errors.Is/As`; type
   aliases (`LLMError`, `SentinelError`) keep call sites compiling
3. **Sanitization** — `SanitizeForUser(err)` redacts sensitive patterns
   before they reach IM/UI surfaces

This document covers the design rationale, layer responsibilities, and the
migration path. Implementation authority: `internal/shared/errors/`.

---

## 2. Core Types

### 2.1 `*SentinelError` (canonical)

```go
// internal/shared/errors/communication.go
type SentinelError struct {
    Code    string  // stable machine-readable code (e.g. "LLM_AUTH_1004")
    Message string  // human-readable summary
    Err     error   // wrapped cause (may be nil)
}

func (e *SentinelError) Error() string {
    if e.Message != "" {
        return e.Message
    }
    if e.Err != nil {
        return e.Err.Error()
    }
    return e.Code
}

func (e *SentinelError) Unwrap() error {
    return e.Err
}
```

`SentinelError.Error()` falls back to inner Err then Code when Message is
empty — this preserves the more permissive behavior the legacy `*LLMError`
type offered and is the canonical implementation for both.

### 2.2 `LLMError` (deprecated alias)

```go
// internal/shared/errors/llm.go
type LLMError = SentinelError  // type alias for back-compat
```

`LLMError` is a Go type alias for `SentinelError` so existing callers using
`*LLMError` keep compiling without modification. New code MUST use
`*SentinelError`. The alias is scheduled for removal in devrix v2.0.0
(>= 2026-09-01).

---

## 3. Code Conventions

All error codes are 5-letter prefix + 4-digit number, e.g. `LLM_AUTH_1004`.
Prefix mapping by domain:

| Prefix | Domain | Code range |
|--------|--------|------------|
| `COMM_` | D1 Communication | 1000–1999 |
| `MSG_`  | D1 Message | 2000–2999 |
| `PERM_` | D1 Permission | 3000–3999 |
| `GW_`   | D1 Gateway | 4000–4999 |
| `LLM_`  | D3 LLM Gateway | 1000–1008 (see llm.go) |
| `AGT_`  | D4/D7 Multi-Agent | 5000–5999 |
| `CTX_`  | D2 Context Engine | 7000–7999 |

See `internal/shared/errors/*.go` for the authoritative code lists.

---

## 4. Layer Responsibilities

### 4.1 Producer (errors created here)

- D1: `NewSessionNotFoundError`, `NewPermissionDeniedError`, ...
- D3: `NewLLMAuthFailedError`, `NewLLMTimeoutError`, ...
- D7: `NewAgentInvalidTransitionError`, `NewInvariantViolationError`, ...

All producers return `*SentinelError` and embed a stable sentinel
(`ErrXxx`) so callers can `errors.Is(err, ErrLLMAuthFailed)` regardless of
wrapping depth.

### 4.2 Propagator (errors wrapped here)

Use `%w` when wrapping:

```go
// GOOD
return fmt.Errorf("taskmanager: create work item: %w", err)

// BAD — breaks errors.Is chain
return fmt.Errorf("taskmanager: create work item: %v", err)
```

Codebase audit (2026-06-20): all callers in `internal/` use `%w` for error
wraps. `%v` is reserved for non-error values (e.g. `[]string`, structs).

### 4.3 Sanitizer (errors rendered to user)

`sharederrors.SanitizeForUser(err error) string` is the **single** entry
point for any IM/UI surface that displays error content to humans. It:

1. Returns `""` for nil error
2. Applies 7 redaction regexes in order:
   - `Bearer <token>` → `[REDACTED]`
   - `sk-...` API keys
   - `gh[pousr]_...` GitHub tokens
   - `xox[baprs]-...` Slack tokens
   - `AKIA...` AWS access keys
   - `/Users/...` and `/home/...` absolute paths
   - long base64 blobs (>=64 chars, no spaces)
3. Truncates to 240 chars + `"..."` suffix
4. Trims whitespace

**Never** call `err.Error()` directly when emitting to user-facing channels.
**Always** use `SanitizeForUser(err)` (which is safe to call on nil too).

---

## 5. Sentinel Migration (deprecated aliases)

Three sentinels moved from local definitions to `shared/errors`:

| Old location | New home | Code | Notes |
|--------------|----------|------|-------|
| `turn_adapter.ErrInvariantViolation` | `shared/errors.ErrInvariantViolation` | `AGT_INVARIANT_5013` | old var is alias; `Prepare` now wraps shared sentinel |

Callers using `turn_adapter.ErrInvariantViolation` keep working via the
deprecated alias. New code MUST use `shared/errors.ErrInvariantViolation`
or import as `sharederrors.ErrInvariantViolation`.

`LLMError` type alias: see §2.2.

---

## 6. Patterns

### 6.1 Defensive wrap (don't `return nil` silently)

```go
// GOOD — propagate error
func (m *TaskManager) Create(...) (*Task, error) {
    item, err := m.tree.Create(...)
    if err != nil {
        return nil, fmt.Errorf("taskmanager: create (session=%s): %w", sessionID, err)
    }
    return item.ToTask(), nil
}

// BAD — silent fallback
func (m *TaskManager) Create(...) *Task {
    item, err := m.tree.Create(...)
    if err != nil {
        return nil  // ← swallows error, caller can't tell why
    }
    return item.ToTask()
}
```

The previous `TaskManager.Create` signature `*Task` (no error) is the
canonical example of the silent-fallback anti-pattern. Fixed in
DM-20260620-003 PR-C.

### 6.2 Nil-sentinel (don't wrap nil)

```go
// GOOD — record a real inner cause
if lastErr == nil {
    lastErr = sharederrors.NewProviderUnavailableError(
        errors.New("retry loop completed without recording an error"))
}

// BAD — wraps nil, breaks errors.Unwrap
if lastErr == nil {
    lastErr = sharederrors.NewProviderUnavailableError(nil)
}
```

Fixed in DM-20260620-003 PR-A retry.go:91.

### 6.3 Classifier fallback (log but don't crash)

```go
// GOOD — log so ops can see degradation
llmResult, err := c.llm.ClassifyIntent(ctx, message)
if err != nil {
    slog.Warn("decisionplanning: LLM classify failed, using rule fallback",
        "error", err,
        "rule_kind", ruleResult.Kind,
    )
    return ruleResult, nil
}

// BAD — silent
if err != nil {
    return ruleResult, nil
}
```

Fixed in DM-20260620-003 PR-C M4.

### 6.4 Error code via metadata

User-facing channels (`EngineEvent.Metadata`) carry the error code so the
front-end can branch on it without re-parsing the message:

```go
o.emitError(out, req.SessionID,
    sharederrors.SanitizeForUser(err),  // safe content
    sharederrors.ErrorCode(err),         // e.g. "LLM_AUTH_1004"
)
```

The `emitError` signature is variadic for back-compat: existing
3-arg callers keep working; new callers can pass the code.

---

## 7. Testing

Each error code MUST have at least one unit test that:

1. Constructs the error via the factory function
2. Verifies `errors.Is(err, ErrXxx)` matches
3. Verifies `errors.As(err, &se)` extracts the code

`SanitizeForUser` has 14 unit tests in `internal/shared/errors/redact_test.go`
covering Bearer/sk-/ghp_/AWS/path/truncation/idempotent/normal/empty/long-blob/short-ids.

---

## 8. Migration Cheatsheet

| From | To | Migration tool |
|------|-----|---------------|
| `fmt.Errorf("...: %v", err)` | `fmt.Errorf("...: %w", err)` | sed / golangci-lint |
| `return nil` (silent fallback) | `return nil, fmt.Errorf("...: %w", err)` | manual review |
| `*LLMError` | `*SentinelError` | sed across call sites |
| `turn_adapter.ErrInvariantViolation` | `sharederrors.ErrInvariantViolation` | rename across call sites |
| `err.Error()` in user-facing channels | `sharederrors.SanitizeForUser(err)` | grep + sed |

---

## 9. References

- Implementation: `internal/shared/errors/`
- OpenSpec change: `openspec/changes/devrix-error-handling-tier1-tier2/`
- Archived (post-S6): `openspec/archive/2026-06-20-devrix-error-handling-tier1-tier2/`
- Related: `docs/context-budget.md` (Phase A/B/C context budget fixes)
- Related: `openspec/specs/d7-orchestration/t-registry.md` (T points registered)