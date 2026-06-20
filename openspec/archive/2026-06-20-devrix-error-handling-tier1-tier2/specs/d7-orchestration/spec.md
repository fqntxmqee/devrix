# D7 Orchestration Error Recovery Specification

## ADDED

### Requirement: D7 turn loop MUST preserve error code through emit chain

The D7 `DefaultOrchestrator.runLoop` and `SubTurnRunner.RunSubTurn` MUST propagate the original error's `code` to the emitted `EngineEvent.Metadata["error_code"]` so that D1 IM adapters can render error-type-aware messages.

<!-- T: D7-S2-A06-T24, D7-S2-A06-T25, D7-S2-A06-T26 -->

#### Scenario: prepare error from D2 carries LLMUnavailable code through emit

- GIVEN a D2 `Prepare` call returns `*sharederrors.Error{Code: "CTX_LLM_4004"}`
- WHEN `runLoop` catches the error at line 256 (nested) or line 292 (main)
- AND calls `emitError(out, sessionID, SanitizeForUser(err), ErrorCode(err))`
- THEN the emitted `EngineEvent` MUST have `Metadata["error_code"] = "CTX_LLM_4004"`

#### Scenario: sub-agent error preserves sentinel via Wrap pattern

- GIVEN a sub-agent's stream emits an "error" event with content
- WHEN `SubTurnRunner.collectSubTurnResult` wraps the content
- THEN the wrapping MUST use `sharederrors.Wrap(ev.Content, sentinel)` to preserve the sentinel
- AND NOT use `fmt.Errorf("subturn: %s", ev.Content)` (which loses the sentinel)

#### Scenario: orchestrate_path wraps errors with %w to preserve chain

- GIVEN a validation error from graph validation
- WHEN `orchestrate_path.go:118` wraps it
- THEN the wrapping MUST use `%w` not `%v`
- AND `errors.Is(returned, original)` MUST be true

### Requirement: D7 MUST sanitize all emitted error content

All `emitError` invocations MUST pass sanitized content via `SanitizeForUser`. Raw `err.Error()` MUST NOT be passed to `emitError`.

<!-- T: D7-S2-A06-T24 -->

#### Scenario: emitError content never contains raw API keys

- GIVEN any `err` with `Error()` containing `sk-abc123xxx...` or `/Users/...`
- WHEN `runLoop` calls `emitError(out, sessionID, ..., err)` (via SanitizeForUser)
- THEN the emitted `event.Content` MUST NOT contain the original sensitive string
- AND MUST contain `[REDACTED]` instead

#### Scenario: emitError accepts variadic code parameter (back-compat)

- GIVEN existing call sites `emitError(out, sessionID, content)`
- WHEN the signature is extended to `emitError(out, sessionID, content, code ...string)`
- THEN existing call sites MUST still compile and function identically
- AND new call sites can pass a 4th argument for error code

### Requirement: D7 workmodel TaskManager MUST return (Task, error) on creation

The D7 workmodel `TaskManager.CreateTask` and similar creation methods MUST return `(*Task, error)` instead of `*Task` alone. Silent nil returns on error violate the project's "no silent error swallowing" rule.

<!-- T: D7-S2-A05-T01 -->

#### Scenario: CreateTask returns error on storage failure

- GIVEN `TaskManager.CreateTask` with a storage backend that fails on write
- WHEN CreateTask is called
- THEN it MUST return `(nil, err)` where `err` is non-nil
- AND NOT silently return `nil, nil` or `nil, &Task{}` etc.

#### Scenario: Classifier fallback LLM failure logs warn before rule fallback

- GIVEN a `decisionplanning.classifier_fallback` call where LLM classification fails
- WHEN the fallback path engages
- THEN `slog.Warn` MUST be emitted with session_id and error
- AND the result MUST include `metadata["classify_fallback"] = "rule"`

### Requirement: D7 turn_adapter invariant error MUST be registered in shared/errors

The D7 turn_adapter's private `ErrInvariantViolation` MUST be moved to `internal/shared/errors` and registered with code `AGT_INVARIANT_5013` so cross-package error handling is consistent.

<!-- T: D7-S2-A06-T27 -->

#### Scenario: ErrInvariantViolation is accessible via shared/errors

- GIVEN the turn_adapter package previously defined `ErrInvariantViolation = errors.New(...)` locally
- WHEN the type is moved to `shared/errors`
- THEN `sharederrors.ErrInvariantViolation` MUST be importable
- AND `sharederrors.IsCode(err, sharederrors.CodeAgentInvariantViolation)` MUST work
- AND turn_adapter's local definition MUST be removed (deprecation period: 0 minor release, hard migration)

## MODIFIED

(None)

## REMOVED

(None)
