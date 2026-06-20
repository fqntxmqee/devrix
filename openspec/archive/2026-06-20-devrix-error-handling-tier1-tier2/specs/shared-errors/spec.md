# Shared Errors Library Specification

## ADDED

### Requirement: sharederrors MUST provide SanitizeForUser helper

The shared errors library MUST provide `SanitizeForUser(err error) string` as a single-source-of-truth function for sanitizing error messages before they reach user-facing render paths.

<!-- T: SHARED-ERRORS-A01-T04 -->

#### Scenario: SanitizeForUser redacts Bearer tokens

- GIVEN `err = errors.New("auth failed: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.xxxxx")`
- WHEN `SanitizeForUser(err)` is called
- THEN the result MUST contain `[REDACTED]` instead of the token
- AND the prefix `auth failed: Bearer ` MUST be preserved

#### Scenario: SanitizeForUser redacts sk-xxx API keys

- GIVEN `err = errors.New("LLM returned status 401, key=sk-abc123def456ghi789jkl012mno")`
- WHEN `SanitizeForUser(err)` is called
- THEN the result MUST contain `[REDACTED]` instead of the key
- AND the rest of the message MUST be preserved

#### Scenario: SanitizeForUser redacts absolute file paths

- GIVEN `err = errors.New("tool round failed: read /Users/fukai/.ssh/id_rsa: no such file")`
- WHEN `SanitizeForUser(err)` is called
- THEN the result MUST contain `[REDACTED]` instead of the path
- AND `read [REDACTED]: no such file` MUST be the sanitized form

#### Scenario: SanitizeForUser handles long messages (truncation)

- GIVEN an error with 1000-character message
- WHEN `SanitizeForUser(err)` is called
- THEN the result MUST be at most 240 characters
- AND the result MUST end with `...` if truncated

#### Scenario: SanitizeForUser on nil returns empty string

- GIVEN `err == nil`
- WHEN `SanitizeForUser(err)` is called
- THEN the result MUST be `""`

#### Scenario: SanitizeForUser on already-sanitized error is idempotent

- GIVEN `err = errors.New("failed: [REDACTED]")` (already sanitized)
- WHEN `SanitizeForUser(err)` is called
- THEN the result MUST equal the input (idempotent)

#### Scenario: SanitizeForUser does not redact normal English words

- GIVEN `err = errors.New("tool round failed: read_file: no such file")`
- WHEN `SanitizeForUser(err)` is called
- THEN the result MUST preserve all words
- AND MUST NOT introduce false `[REDACTED]` markers

#### Scenario: SanitizeForUser handles nested wrapped errors

- GIVEN `err = fmt.Errorf("outer: %w", errors.New("inner: sk-key123456789012"))`
- WHEN `SanitizeForUser(err)` is called
- THEN the result MUST contain `[REDACTED]` for the key
- AND the result MUST be derived from the outermost message

### Requirement: sharederrors MUST unify Error type across domains

The shared errors library MUST provide a single `*Error` type that replaces both legacy `LLMError` and `SentinelError`. The legacy types are kept as type aliases for 1 minor release.

<!-- T: SHARED-ERRORS-A01-T01, SHARED-ERRORS-A01-T02, SHARED-ERRORS-A01-T03 -->

#### Scenario: *Error implements error interface

- GIVEN `e := &Error{Code: "X", Message: "test", Err: nil}`
- WHEN `e.Error()` is called
- THEN it MUST return `"test"` (the Message)
- AND `errors.Is(e, ErrSessionNotFound)` MUST work if the code matches

#### Scenario: *Error implements Unwrap

- GIVEN `e := &Error{Code: "X", Message: "test", Err: baseErr}`
- WHEN `errors.Unwrap(e)` is called
- THEN it MUST return `baseErr`

#### Scenario: WithCode constructs *Error correctly

- GIVEN `e := WithCode("X", "msg", baseErr)`
- THEN `e.Code == "X"`, `e.Message == "msg"`, `e.Err == baseErr`

#### Scenario: IsCode returns true when code matches

- GIVEN `err = WithCode("LLM_AUTH_1004", "msg", baseErr)`
- WHEN `IsCode(err, "LLM_AUTH_1004")` is called
- THEN it MUST return `true`

#### Scenario: IsCode returns false for non-Error types

- GIVEN `err = errors.New("plain")`
- WHEN `IsCode(err, "X")` is called
- THEN it MUST return `false`

#### Scenario: IsRetryable works for LLM timeout via unified type

- GIVEN `err = WithCode(CodeLLMTimeout, "msg", baseErr)`
- WHEN `IsRetryable(err)` is called
- THEN it MUST return `true`

#### Scenario: LLMError is a type alias for *Error (back-compat)

- GIVEN `var e *LLMError = &Error{...}` (compile-time)
- WHEN existing call sites do `errors.As(err, &LLMError{})`
- THEN the call MUST succeed (alias is identical type)
- AND the underlying value MUST be inspectable via `e.Code` / `e.Message` / `e.Err`

#### Scenario: SentinelError is a type alias for *Error (back-compat)

- GIVEN `var e *SentinelError = &Error{...}` (compile-time)
- WHEN existing call sites do `errors.Is(err, ErrSessionNotFound)` (where ErrSessionNotFound is `*Error`)
- THEN the call MUST succeed

### Requirement: sharederrors retry helper MUST NOT wrap nil with sentinel

The `sharederrors` retry helper (`protect/retry.go`) MUST NOT construct a sentinel error wrapping a nil cause. When all retry attempts are exhausted without capturing an error, the helper MUST return a fresh error.

<!-- T: SHARED-ERRORS-A01-T06 -->

#### Scenario: All retry attempts exhausted with no captured error returns fresh error

- GIVEN a retry configuration where the underlying executor never sets `lastErr`
- WHEN all attempts are exhausted
- THEN the returned error MUST NOT be a sentinel wrapping nil
- AND the error message MUST contain `"retry attempts exhausted"`
- AND `IsRetryable(returned)` MUST be `false` to prevent infinite retry

#### Scenario: Retry captures last error from underlying executor

- GIVEN a retry configuration where the underlying executor returns a known error
- WHEN the first retry fails and subsequent retries also fail
- THEN the returned error MUST be the last captured error (preserved)
- AND the error MUST NOT be re-wrapped with `NewProviderUnavailableError`

### Requirement: workmodel TaskManager MUST return (*Task, error) from CreateTask

The workmodel `TaskManager.CreateTask` MUST return `(*Task, error)` so creation failures are not silently dropped.

<!-- T: SHARED-ERRORS-A01-T06 (cross-ref D7-S2-A05-T01) -->

#### Scenario: CreateTask with nil error returns valid task

- GIVEN a valid CreateTask call
- WHEN the storage backend writes successfully
- THEN the function MUST return `(task, nil)` where `task` is non-nil

#### Scenario: CreateTask with storage error returns nil and non-nil error

- GIVEN a CreateTask call
- WHEN the storage backend fails
- THEN the function MUST return `(nil, err)` where `err` is non-nil and wraps the storage error
- AND the error MUST be `sharederrors.Wrap`-able so callers can `errors.Is(err, ErrStorage)`

## MODIFIED

(None)

## REMOVED

(None)
