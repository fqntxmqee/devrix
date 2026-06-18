# Diagnostic Tracker Specification

**Surface ID:** D5-S23-A02
**Phase:** 1 (P0)
**Status:** S3_Designed

<!-- T: D5-S23-A02-T01, D5-S23-A02-T02, D5-S23-A02-T03, D5-S23-A02-T04, D5-S23-A02-T05 -->

## ADDED

### Requirement: Edit-Time Diff Collection

#### Scenario: Successful Edit Triggers Tracker
- GIVEN LLM calls `edit_file` on `/workspace/src/main.go`
- WHEN the edit is committed
- THEN the DiagnosticTracker asynchronously records the pre-edit snapshot
- AND records the post-edit diff
- AND emits a span `d5.diagnostic.diff` with `{file, change_kind, lines_added, lines_removed}`

<!-- T: D5-S23-A02-T01 -->

#### Scenario: Tracker Does Not Block Edit
- GIVEN the tracker is slow (e.g., 500ms diff collection)
- WHEN LLM calls `edit_file`
- THEN the edit completes within the original tool execution time (< 100ms)
- AND the tracker fires fire-and-forget via async channel

#### Scenario: Multiple Edits to Same File
- GIVEN the same file is edited 3 times in a session
- WHEN all 3 edits complete
- THEN the tracker records 3 separate diff entries
- AND each is queryable by edit timestamp

### Requirement: LRU Deduplication

#### Scenario: Identical Edit Deduplicated
- GIVEN an edit with content `[A, B, C]` → `[A, B, D]` (changed line 3)
- WHEN the same edit is repeated within the LRU window (1000 entries)
- THEN only the first edit is recorded
- AND the duplicate is silently dropped
- AND a metric `d5.diagnostic.dedup_hit` is incremented

<!-- T: D5-S23-A02-T02 -->

#### Scenario: LRU Eviction at Capacity
- GIVEN the LRU has 1000 entries (at capacity)
- WHEN a new edit is recorded
- THEN the least recently accessed entry is evicted
- AND the new entry takes its place

### Requirement: Linter Integration (Future)

#### Scenario: Async Linter Invocation
- GIVEN the tracker has recorded an edit
- WHEN the async trigger fires
- THEN the linter is invoked on the edited file
- AND the linter result is stored alongside the diff
- AND a span `d5.diagnostic.lint` is emitted with linter findings

<!-- T: D5-S23-A02-T03 -->

> Note: Initial implementation may not include linter integration. Spec is forward-looking.

### Requirement: LTL-Lite Invariants

<!-- T: D5-S23-A02-F*-T04 -->

The DiagnosticTracker MUST satisfy:

- `edit_completion => !blocked_by_tracker` (tracker never blocks edit)
- `lru_capacity <= 1000` (memory bound)
- `async_trigger_is_buffered` (channel has buffer ≥ 100)
- `dedup_within_window` (identical edits within LRU are deduplicated)

### Requirement: EditSurface Integration

#### Scenario: EditSurface Triggers Tracker
- GIVEN an EditSurface is about to execute `edit_file` or `write_file`
- WHEN the edit completes successfully
- THEN the tracker receives a `OnEdit(workDir, toolName, input)` call
- AND the call is non-blocking (uses goroutine or channel)

<!-- T: D5-S23-A02-T05 -->

## MODIFIED

### Requirement: EditSurface (D2-S18 builtin) Now Depends on Tracker

The EditSurface gains a new dependency on DiagnosticTracker. The dependency is injected via constructor:

```go
func NewEditSurface(tracker *diagnose.DiagnosticTracker) *EditSurface {
    return &EditSurface{tracker: tracker}
}
```

The surface is backward-compatible: if tracker is nil, the edit completes without tracking.

## REMOVED

(None)
