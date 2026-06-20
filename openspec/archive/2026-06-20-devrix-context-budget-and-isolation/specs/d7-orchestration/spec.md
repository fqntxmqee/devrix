# D7 Orchestration — Spec Delta (2026-06-20-devrix-context-budget-and-isolation)

**Change ID:** 2026-06-20-devrix-context-budget-and-isolation
**Demand ID:** DM-20260620-001
**Base:** `openspec/specs/d7-orchestration/spec.md` v3.x
**Scope:** AC1 + AC2 + AC4 — turn loop integration of the persist + audit
helpers defined in D2.

---

## MODIFIED: D7-S2-A06 RunTurn (Tool Result Construction)

The turn loop's tool-result message construction now consults
`OrchestratorDeps.ToolResultStore` and routes results from size-capped
tools through `buildToolResultMsgWithCap` instead of the legacy
`buildToolResultMsg`.

#### Scenario: AC1 Tool result cap wired in turn loop

- GIVEN `OrchestratorDeps.ToolResultStore != nil` AND
  `OrchestratorDeps.MaxToolResultChars > 0`
- WHEN the turn loop appends a tool-result message
- THEN the message content is routed through `buildToolResultMsgWithCap`
- AND oversized results from allow-listed tools are replaced with a
  preview marker pointing to the persisted file
- AND non-capped tools pass through unchanged

<!-- T: D7-S2-A06-T08 -->

#### Scenario: AC1 Nil store is a no-op

- GIVEN `OrchestratorDeps.ToolResultStore == nil`
- WHEN the turn loop appends a tool-result message
- THEN the legacy `buildToolResultMsg` is used (legacy behaviour
  preserved for tests / non-prod wiring)

<!-- T: D7-S2-A06-T09 -->

---

## MODIFIED: D7-S2-A06 RunTurn (Assistant Message Construction)

The assistant tool-call message is now built via
`buildAssistantToolCallMsgFolded` so assistant text exceeding
`MaxAssistantChars` is folded head/tail style via the shared store.

#### Scenario: AC2 Assistant fold wired in turn loop

- GIVEN `OrchestratorDeps.ToolResultStore != nil` AND
  `OrchestratorDeps.MaxAssistantChars > 0`
- WHEN the LLM emits a response whose text exceeds `MaxAssistantChars`
- AND the response carries tool calls (so the turn loop appends a
  tool-call message)
- THEN the assistant message content is replaced with the head/tail
  folded version
- AND `tool_calls` metadata is preserved

<!-- T: D7-S2-A06-T10 -->

---

## MODIFIED: D7-S2-A06 RunTurn (Per-iteration Audit Hook)

The turn loop now invokes `runTokenAudit` at the top of every
iteration (BEFORE the LLM invoke). The audit attaches `audit.*` span
attributes and emits a structured slog line; the largest assistant
message is folded in-place when `ShouldFoldProactively` returns true.

#### Scenario: AC4 Audit runs at top of iteration

- GIVEN any non-final turn iteration
- WHEN the turn loop body starts
- THEN `runTokenAudit` runs BEFORE the LLM span is opened
- AND the audit result is attached to the iteration's turn span

<!-- T: D7-S2-A06-T11 -->

#### Scenario: AC4 WireD7 bootstrap constructs the store

- GIVEN `internal/bootstrap/wire_coordinator.go` builds the orchestrator
- WHEN the bootstrap runs
- THEN `OrchestratorDeps.ToolResultStore` is non-nil
- AND the store root defaults to `~/.devrix/tool-results`

<!-- T: D7-S2-A06-T12 -->

---

## DEFERRED: AC3 Per-iteration Prepare

`Prepare` is intentionally NOT moved inside the turn loop in Phase A.
The high-leverage piece is the audit + proactive fold; the Prepare
→ LLM → Tool pipeline is expensive to repeat and the systemPrompt +
Tools set is stable across a turn. A follow-up OpenSpec can
re-evaluate the Prepare cadence if production sessions show the
audit-fold isn't enough.
