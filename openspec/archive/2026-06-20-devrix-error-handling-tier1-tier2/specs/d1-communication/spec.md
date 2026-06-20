# D1 Communication Error Sanitization Specification

## ADDED

### Requirement: D1 IM adapter MUST sanitize all user-facing error strings

The D1 communication layer renders error events to IM platforms (Feishu, CLI). All error strings reaching user-facing render paths MUST pass through `sharederrors.SanitizeForUser(err)` to prevent leakage of API keys, file paths, and provider stack traces.

<!-- T: D1-S2-A07-T01, D1-S2-A07-T02 -->

#### Scenario: Feishu error card redacts API key in provider error body

- GIVEN a `LLMError` with message containing `Bearer sk-abc123xxx...` and the devrix error pipeline
- WHEN the orchestrator emits an error event and the feishu adapter renders the error card
- THEN the feishu card's `Markdown(content)` MUST display `[REDACTED]` instead of the original token
- AND the error event's `Metadata["error_code"]` MUST equal `LLM_AUTH_1004`

#### Scenario: Feishu replyAck on RouteInbound failure redacts file paths

- GIVEN a routing error containing `/Users/fukai/.ssh/id_rsa` in its message
- WHEN the feishu adapter's `replyAckToUser` is called via `feishu.go:827`
- THEN the user-facing message MUST replace the file path with `[REDACTED]`
- AND the suffix `\n请重试，或发送 /new 开启新会话。` MUST be preserved

#### Scenario: Long error messages are truncated to 240 chars

- GIVEN a 1000-character error message
- WHEN `SanitizeForUser(err)` is called
- THEN the result MUST be at most 240 characters + `"..."` (243 total)

#### Scenario: Nested error chains are flattened to outermost message

- GIVEN a wrapped error chain `&Error{Code: "X", Err: &Error{Code: "Y", Err: baseErr}}`
- WHEN `SanitizeForUser(err)` is called
- THEN only the outermost message is redacted and returned
- AND nested error details are NOT surfaced to the user

### Requirement: D1 IM adapter MUST NOT call err.Error() directly in render paths

All D1 adapter call sites that render errors to user-facing output (feishu cards, CLI messages) MUST route through `SanitizeForUser` instead of `err.Error()`. Direct `err.Error()` rendering is a security risk.

<!-- T: D1-S2-A07-T01, D1-S2-A07-T02 -->

#### Scenario: Static analysis detects zero remaining direct err.Error() in adapter render paths

- GIVEN the D1 channel adapters under `internal/layers/communication/channel/adapters/`
- WHEN grepping for `Markdown(err.Error())` or `"..."+err.Error()+...` patterns
- THEN zero matches in user-facing render paths (excluding log paths)

## MODIFIED

(None)

## REMOVED

(None)
