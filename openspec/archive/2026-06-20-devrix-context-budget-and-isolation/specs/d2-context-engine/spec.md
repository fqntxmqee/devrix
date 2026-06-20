# D2 Context Engine — Spec Delta (2026-06-20-devrix-context-budget-and-isolation)

**Change ID:** 2026-06-20-devrix-context-budget-and-isolation
**Demand ID:** DM-20260620-001
**Base:** `openspec/specs/d2-context-engine/spec.md` v3.x
**Scope:** AC1 (tool result size cap) + AC2 (assistant fold) + AC4 + AC13 (per-iter audit + logging)

---

## NEW: D2-S17-A05 ToolResultStore (AC1)

A new persistence helper under `internal/layers/contextengine/prepare/persist/`
writes oversized tool results to disk and returns a `<persisted-output>`
preview marker. Storage root: `~/.devrix/tool-results/<sessionID>/`.

#### Scenario: AC1 Below-limit passthrough

- GIVEN a tool result shorter than `MaxToolResultChars` (default 12 000)
- WHEN `ToolResultStore.Persist` is called
- THEN the original content is returned unchanged
- AND no file is written to disk

<!-- T: D2-S17-A05-T01 -->

#### Scenario: AC1 Above-limit persists with preview

- GIVEN a tool result longer than `MaxToolResultChars`
- AND the tool name is in the size-cap allowlist (read_file / bash:grep /
  bash:cat / bash:find / bash:head / bash:tail / bash:rg / bash:ls)
- WHEN `ToolResultStore.Persist` is called
- THEN the full content is written to
  `~/.devrix/tool-results/<sessionID>/<stamp>-<id>.txt`
- AND the returned marker contains `<persisted-output>`, a size label,
  the full path, and a `Preview (first N chars):` head

<!-- T: D2-S17-A05-T02 -->

#### Scenario: AC1 Allowlist is enforced

- GIVEN a tool result from `task_create`, `delegate_worker`, or any other
  non-listed tool
- WHEN the orchestrator builds the tool-result message
- THEN the cap is NOT applied
- AND the original content is preserved verbatim

<!-- T: D2-S17-A05-T03 -->

#### Scenario: AC1 Session ID is sanitised

- GIVEN a session ID containing `/`, `\`, or `..`
- WHEN `ToolResultStore.Persist` is called
- THEN the on-disk sub-directory replaces those characters with `_`
- AND the path cannot escape the root

<!-- T: D2-S17-A05-T04 -->

#### Scenario: AC1 Persist failure falls back to head truncation

- GIVEN `ToolResultStore.Persist` returns an I/O error
- WHEN the orchestrator builds the tool-result message
- THEN the message is replaced with a head-truncated version followed
  by a "[truncated, persist failed]" trailer
- AND the turn loop is NOT aborted (transient I/O must not stop the turn)

<!-- T: D2-S17-A05-T05 -->

---

## NEW: D2-S17-A06 FoldAssistantOutput (AC2)

A helper that persists oversized assistant messages head/tail style and
returns a `<prior-output-summary>` marker. Storage root:
`~/.devrix/tool-results/<sessionID>/turn-outputs/`.

#### Scenario: AC2 Below-limit passthrough

- GIVEN an assistant message shorter than `MaxAssistantChars` (default 8 000)
- WHEN `FoldAssistantOutput` is called
- THEN the original content is returned unchanged
- AND no file is written to disk

<!-- T: D2-S17-A06-T01 -->

#### Scenario: AC2 Above-limit folds with head + tail

- GIVEN an assistant message longer than `MaxAssistantChars`
- WHEN `FoldAssistantOutput` is called
- THEN the full content is written to
  `~/.devrix/tool-results/<sessionID>/turn-outputs/t<n>-<stamp>-<id>.txt`
- AND the returned marker contains `<prior-output-summary>`, the first
  800 runes, a `[middle N chars truncated; see PATH]` marker, and the
  last 200 runes

<!-- T: D2-S17-A06-T02 -->

#### Scenario: AC2 tool_calls metadata preserved

- GIVEN a folded assistant message that originally carried
  `Metadata["tool_calls"]`
- WHEN `buildAssistantToolCallMsgFolded` is called
- THEN the `tool_calls` metadata is preserved on the folded message
- AND only the `Content` field is replaced

<!-- T: D2-S17-A06-T03 -->

---

## NEW: D2-S15-A08 TokenAudit (AC4 + AC13)

A lightweight in-band token-budget diagnostic. The orchestrator calls
`AuditMessages` + `ShouldFoldProactively` at the top of every turn
iteration BEFORE invoking the LLM.

#### Scenario: AC4 Audit fires per iteration

- GIVEN any turn iteration
- WHEN `runTokenAudit` is invoked
- THEN the audit computes `TotalTokens`, `SystemTokens`,
  `MessagesTokens`, `LargestMsgTokens`, `LargestMsgIdx`,
  `BudgetPercent`, `OverBudget`
- AND attaches these as `audit.*` attributes on the turn span
- AND emits a structured slog line `orchestrator: token audit`

<!-- T: D2-S15-A08-T01 -->

#### Scenario: AC4 Proactive fold fires at 60% threshold

- GIVEN `TotalTokens / Budget >= 0.6` (the default `DefaultProactiveFoldPercent`)
- AND the largest message exceeds `MaxAssistantChars`
- WHEN `ShouldFoldProactively` is called
- THEN it returns `true`
- AND the orchestrator folds the largest assistant message in-place via
  `FoldAssistantOutput` BEFORE the LLM invoke

<!-- T: D2-S15-A08-T02 -->

#### Scenario: AC4 Proactive fold fires on over-budget

- GIVEN `TotalTokens > Budget`
- WHEN `ShouldFoldProactively` is called
- THEN it returns `true` regardless of the percentage threshold

<!-- T: D2-S15-A08-T03 -->

#### Scenario: AC4 Proactive fold is a no-op when below threshold

- GIVEN `TotalTokens / Budget < 0.6` AND `TotalTokens <= Budget`
- WHEN `ShouldFoldProactively` is called
- THEN it returns `false`
- AND the message buffer is left untouched

<!-- T: D2-S15-A08-T04 -->

#### Scenario: AC4 Largest message under cap → no fold

- GIVEN the largest message is shorter than `MaxAssistantChars`
- WHEN `ShouldFoldProactively` is called
- THEN it returns `false` even if budget is over 60%
- (folding a message that is already under cap is wasted work)

<!-- T: D2-S15-A08-T05 -->
