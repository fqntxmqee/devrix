# Delta: D2 Context Engine — Review Hardening

**Change ID:** `devrix-d2-d7-review-hardening`  
**Demand ID:** DM-20260630-013  
**Affects:** D2-S15, D2-S17, D2-S18

---

## ADDED

### Requirement: D2-S18-A80 PlanModeWriteParity — edit_file 与 write_file 对齐

`edit_file` tool runner SHALL invoke `EnforcePlanModeWrite(ctx, resolvedPath)` after `resolveWorkspacePath` and before any read/write, identical to `write_file` behavior.

#### Scenario: Plan mode blocks edit_file on non-plan path
- GIVEN plan mode active and target file outside plan allowlist
- WHEN LLM invokes `edit_file` with `file_path` pointing to `internal/foo.go`
- THEN tool result MUST be Deny with plan-mode error
- AND file contents MUST NOT change

#### Scenario: Plan mode allows edit_file on plan file
- GIVEN plan mode active and `file_path` is the active plan markdown file
- WHEN `edit_file` is invoked with valid old/new strings
- THEN edit MUST succeed same as `write_file` on plan path

---

### Requirement: D2-S18-A81 SymlinkContainment — resolveWorkspacePath 真实路径约束

`resolveWorkspacePath` SHALL resolve symlinks via `filepath.EvalSymlinks` (or equivalent) and verify the real path remains under `workDir`. Paths escaping workspace after symlink resolution MUST be rejected.

#### Scenario: Symlink pointing outside workspace
- GIVEN `workDir=/project` and `secrets` symlink → `/etc/passwd`
- WHEN tool resolves `file_path=secrets`
- THEN MUST return error `path escapes workspace`
- AND MUST NOT read or write through the symlink

#### Scenario: Symlink inside workspace
- GIVEN `workDir=/project` and `src/link` → `src/real.go` (both under workDir)
- WHEN `read_file` resolves `src/link`
- THEN MUST read `src/real.go` content successfully

---

### Requirement: D2-S15-A80 AutocompactWriteback — 异步摘要写回闭环

When async autocompact is enabled, `CompressionEventSink.EmitAutocompactComplete` MUST be implemented by production wiring to atomically replace the pending placeholder message in `SessionContext.Messages` identified by `asyncToken`. Default MUST NOT be `NoOpCompressionEventSink` when `AsyncAutocompacter` is wired.

#### Scenario: Async complete replaces placeholder
- GIVEN async autocompact triggered with placeholder `status=pending` and token `tok-1`
- WHEN background summarizer completes with summary text S
- THEN `SessionContext.Messages` MUST contain summary message replacing placeholder for `tok-1`
- AND MUST NOT retain permanent `"compressing…"` placeholder in subsequent turns

#### Scenario: Async failure degrades without silent loss
- GIVEN summarizer returns error
- WHEN async compact completes with failure
- THEN observer MUST emit `Degraded` status
- AND middle messages MUST be preserved OR sync fallback compact MUST run
- AND MUST NOT leave irrecoverable pending-only context

---

### Requirement: D2-S18-A82 EnforceFailClosed — 沙箱降级路径

Tool execution policy SHALL be fail-closed in production configuration:

- `sandbox.enabled=false` MUST emit startup warning and metric `sandbox_disabled`
- `bashAST == nil` on production `BuiltinSurface` MUST Deny bash (not Allow)
- `CommandPolicy.Validate` when disabled MUST NOT be the only guard if bash is still exposed

#### Scenario: Nil bashAST denies bash
- GIVEN `BuiltinSurface` constructed without `bashAST` (non-test bootstrap)
- WHEN `CheckPermission` for bash tool is evaluated
- THEN decision MUST be Deny or Ask (never Allow)

#### Scenario: Sandbox disabled warns at startup
- GIVEN config `sandbox.enabled=false`
- WHEN ContextEngine boots in production profile
- THEN `slog.Warn` MUST include `sandbox_disabled`
- AND metric counter MUST increment once

---

### Requirement: D2-S18-A83 BashAuditRedaction — 命令审计脱敏

`tool.bash.audit` logs MUST NOT record full command strings containing credential patterns (Bearer, sk-, ghp_, xoxb-, AKIA, etc.). SHALL redact or truncate command body beyond 256 runes.

#### Scenario: Token redacted in audit log
- GIVEN bash command `curl -H "Authorization: Bearer secret-token" ...`
- WHEN audit log is written
- THEN log MUST NOT contain `secret-token` literal
- AND MUST contain redaction marker or hash

---

### Requirement: D2-S15-A81 CompressedView Mutex — 视图与消息并发安全

`SessionContext` mutations to `CompressedView` and `SystemPrompt` via `SetCompressedView` / `EnrichWithLongTermRecall` SHALL hold the same `messagesMu` (or equivalent) used by `Append*` / `TrimMessages`.

#### Scenario: No race between append and set view
- GIVEN goroutine A appends messages and goroutine B sets CompressedView
- WHEN run under `-race`
- THEN MUST report zero data races on SessionContext fields

---

### Requirement: D2-S15-A82 AsyncCompact Session Context — 生命周期绑定

Async autocompact goroutine MUST use a session-scoped cancelable context derived from the turn/session lifecycle, NOT `context.Background()` alone. Session cancel MUST abort in-flight summarization when possible.

#### Scenario: Session cancel aborts summarizer
- GIVEN async compact in flight for session S
- WHEN session context is cancelled
- THEN summarizer SHOULD stop within timeout
- AND MUST NOT call writeback after session teardown

---

### Requirement: D2-S15-A83 Microcompact Tool Chain Integrity

`microcompact` step MUST NOT merge adjacent messages that would break tool call/result pairing: messages with `tool_calls`, `MessageRoleTool`, or distinct `tool_call_id` chains SHALL be skipped.

#### Scenario: Tool messages not merged
- GIVEN adjacent assistant message with `tool_calls` and following tool result message
- WHEN microcompact runs
- THEN both messages MUST remain separate
- AND provider-facing message order MUST remain valid

---

### Requirement: D2-S17-A80 MaterializeJSONLStrict — 损坏行可观测

`materialize` JSONL `Load` SHALL count skipped malformed lines and return warning error when `badLines > 0`, OR support `strict=true` mode that fails on first bad line.

#### Scenario: Corrupt line reported
- GIVEN JSONL file with 1 invalid line among 10 valid
- WHEN Load runs in default mode
- THEN MUST return `badLines=1` warning or error
- AND MUST NOT silently behave as if file was complete

---

## MODIFIED

### Requirement: D2-S18 BashAST Surface Parse Failure

`BashASTPolicy` v1 parse failure MUST return **Deny** in production wiring (not `DecisionAsk`). Production bootstrap MUST use `NewBashASTPolicyWithBashPolicy`.

#### Scenario: Malformed bash denied
- GIVEN unparseable bash command string
- WHEN surface CheckPermission runs in production policy
- THEN decision MUST be Deny

---

### Requirement: D2-S18 PerRisk Unknown Threshold

`per_risk` filter with unknown `RiskThreshold` MUST default to strictest level (LOW) or Deny, not pass-through.

---

## REMOVED

(None)

---

## L5 Test Points (register in t-registry at S4)

| ID | Description | Priority |
|----|-------------|----------|
| D2-S18-A80-T01 | edit_file plan mode deny non-plan path | P0 |
| D2-S18-A80-T02 | edit_file plan mode allow plan file | P0 |
| D2-S18-A81-T01 | symlink escape denied | P0 |
| D2-S18-A81-T02 | intra-workspace symlink allowed | P1 |
| D2-S15-A80-T01 | async writeback replaces placeholder | P0 |
| D2-S15-A80-T02 | async failure degraded preserves middle | P0 |
| D2-S18-A82-T01 | nil bashAST deny | P1 |
| D2-S18-A82-T02 | sandbox disabled startup warn | P1 |
| D2-S18-A83-T01 | bash audit redacts bearer token | P1 |
| D2-S15-A81-T01 | CompressedView race-free | P1 |
| D2-S15-A82-T01 | session cancel stops async summarizer | P1 |
| D2-S15-A83-T01 | microcompact preserves tool chain | P1 |
| D2-S17-A80-T01 | JSONL corrupt line counted | P2 |
| D2-S18-A84-T01 | bashAST parse fail deny | P1 |
| D2-S18-A85-T01 | unknown risk threshold strict | P2 |
