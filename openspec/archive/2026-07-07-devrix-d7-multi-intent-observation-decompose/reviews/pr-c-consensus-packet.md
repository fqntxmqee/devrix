# PR-C Consensus Packet — ItemPipelineRunner wiring + Stream emit + Learn per-segment + Feishu idempotency

**Date:** 2026-07-07
**PR:** PR-C (DM-20260707-001, 7-PR split, step 4/7)
**Author:** Claude (Sonnet 4.6)
**Reviewers:** codex (MiniMax-M3), cursor-agent
**Predecessors:** PR-A1 #451, PR-A2 #452, PR-B #453 (consensus 100% adopted, including #454 review-fixes)

---

## 1. PROBLEM

PR-A1/A2/B shipped:
- **PR-A1**: IntentSegment + PlanDAG grammar + validateDAG (no runtime, no wiring).
- **PR-A2**: SegmenterDispatcher that produces IntentSegmentSet at Observe time.
- **PR-B**: DAGExecutor runtime (plan.PlanDAG → wavescheduler.TaskGraph) with
  `<-chan SegmentEmit` streaming surface, `run_start`/`run_done`/`run_abort`
  audit, artifact-driven emit + re-drain race defense.

Now **PR-C** ships the **wire-up + streaming + reputation integration**:
1. ItemPipelineRunner.Run() must **detect** `StrategicPlanProposal.DAG != nil`
   and **forward** to DAGExecutor.RunPlanDAG (T22).
2. Each child completion → IM partial card (T23).
3. Parent rollup → IM final card (overwrites prior partial) (T24).
4. Learn per-segment: WorkItemID + ParentEvidence aggregator (T25, T26).
5. Feishu streaming API + in-memory dedup table (T27, T28).

PR-C is the "thinnest slice end-to-end" — minimal scope that demonstrates
multi-intent decompose → parallel Execute → IM partial+final cards → per-segment
Learn — without introducing the gating/e2e/verify-archive or full Learn
22-scenario (those are PR-D, PR-E, PR-F).

---

## 2. CONSTRAINTS

### 2.1 Hard constraints
1. **ItemPipelineRunner single-source-of-truth** — Run() is the 5-node MUPS
   pipeline. The DAG fork is a NEW branch inside EnterMUPSPhase(Execute);
   it must NOT bypass Observe/Plan/Verify/Learn/Decide phases.
2. **Idempotency key = `(session_id, segment_id)`** for child partial emits;
   parent rollup gets `(session_id, parent_id)` (or `"rollup:" + parent_id`
   per proposal.md §4 question 4 — see Q1 below).
3. **No raw LLM calls in sessionorchestrator** — Feishu streaming lives in
   `communication/feishu/`, NOT in `orchestration/`. Sessionorchestrator
   emits `*contracts.EngineEvent` only.
4. **Single-writer per (sessionID, segmentID) at a time** — concurrent
   partial emits for the same segment are impossible because the DAG
   executor emits one SegmentEmit per terminal node. If a duplicate ever
   appears (e.g. from reentry), the dedup table catches it.
5. **LearnRequest is per-segment** — T25 adds `WorkItemID` field to
   `mups/learn/learner.go`'s `LearnRequest`. The existing per-round
   Learn (one call per WorkItem) is REPLACED by per-segment calls
   (one per child + one for parent rollup).
6. **Backward compatibility** — when `Plan.DAG == nil`, the existing
   single-round Learn path stays unchanged. PR-C's per-segment Learn
   is opt-in via `Plan.DAG != nil`.
7. **No `mups/learn` directory rename** — keep file structure stable
   (PR-B's package migration already moved execute/ + learn/ to mups/).

### 2.2 Soft constraints
1. Adapter code in sessionorchestrator ≤ 200 LOC (excluding dedup table).
2. Feishu streaming adapter ≤ 250 LOC.
3. Coverage ≥ 80% on the new code paths.
4. New error sentinels use `sharederrors.SentinelError` (consistency with
   PR-B's 7210-7213 family); proposed 7214-7217.
5. New `Orchestrator ID` for new spans: `D7-S15-PRC-*`.

---

## 3. SCOPE

### 3.1 MODIFIED: `sessionorchestrator/item_pipeline.go` — DAG fork (T22, partial of T23)

After `endPlanPhase()` and before `enterMUPSPhase(...Execute)`:

```go
// New: detect Plan.DAG and fork to DAG executor
if proposal != nil && proposal.DAG != nil {
    return r.executePlanDAG(ctx, sessionID, item, proposal, opts)
}
```

New helper `executePlanDAG(...)`:
1. Get `dagExecutor` from runner (added in deps).
2. Call `dagExecutor.RunPlanDAG(ctx, sessionID, item.ID, proposal.DAG, proposal.IntentSegmentSet)`.
3. Spawn consumer goroutine:
   - For each non-final emit → `opts.Emit(&EngineEvent{Type: "partial", Content: e.Summary, Metadata: {"idempotency_key": e.IdempotencyKey(), "segment_id": e.SegmentID, "is_final": "false"}})`
   - For final emit → `opts.Emit(&EngineEvent{Type: "complete", Content: e.Summary, Metadata: {"idempotency_key": "rollup:"+item.ID, "segment_id": "", "is_final": "true"}})`
4. After channel close: construct round with all partial + final emits as
   artifact.Summary (joined by newline).
5. Bypass the existing per-WorkItem ReAct loop; `endExecutePhase` is called
   directly with the new round.

**Important**: PR-C does NOT touch the Verify/Learn/Decide pipeline downstream
of Execute. The DAG's child completions still flow through Verify+Learn+Decide
via the per-segment LearnRequest (T25).

### 3.2 MODIFIED: `sessionorchestrator/emit_dedup.go` (NEW) — Dedup table (partial T23, T28)

```go
package sessionorchestrator

type EmitDedup struct {
    mu   sync.RWMutex
    seen map[string]bool // idempotency_key → seen
}

func NewEmitDedup() *EmitDedup { return &EmitDedup{seen: make(map[string]bool)} }

// MarkAndCheck returns true if this is the first time we see the key
// (and the caller should emit). false → dedup hit, drop.
func (d *EmitDedup) MarkAndCheck(key string) bool {
    d.mu.Lock()
    defer d.mu.Unlock()
    if d.seen[key] {
        return false
    }
    d.seen[key] = true
    return true
}

// Reset clears the table (used on session end).
func (d *EmitDedup) Reset() {
    d.mu.Lock()
    defer d.mu.Unlock()
    d.seen = make(map[string]bool)
}
```

Wired into `executePlanDAG`: every emit checks dedup first. Dedup instance
is per-session (stored in `sessionOrchestrator.sessions[id].emitDedup`).

**Sentinel for dedup hit**: 7214 `ORCH_EMIT_DEDUP_HIT_7214` (debug-level,
not error).

### 3.3 MODIFIED: `mups/learn/learner.go` — WorkItemID + per-segment (T25)

```go
type LearnRequest struct {
    // ... existing fields ...
    WorkItemID  string `json:"work_item_id"`         // NEW: per-segment attribution
    ParentID    string `json:"parent_id,omitempty"`  // NEW: set when learning parent rollup
    IsRollup    bool   `json:"is_rollup"`           // NEW: true for parent rollup
    Evidence    *ParentEvidence `json:"evidence,omitempty"` // NEW: child aggregations for rollup
}
```

Add helper:
```go
func LearnPerSegment(ctx context.Context, learner Learner, req LearnRequest) error {
    // Wrap error with WorkItemID for traceability.
}
```

### 3.4 NEW: `mups/learn/reputation/parent_evidence.go` — ParentEvidence aggregator (T26)

```go
type ParentEvidence struct {
    SumAlpha     float64 `json:"sum_alpha"`     // Σ child α
    SumBeta      float64 `json:"sum_beta"`      // Σ child β
    ChildCount   int     `json:"child_count"`   // N children
    FailureCount int     `json:"failure_count"` // K failed children
    AdaptivePrior *AdaptivePrior `json:"adaptive_prior,omitempty"` // folded prior
}

func AggregateParentEvidence(childEvidences []Evidence) ParentEvidence {
    // Sum α/β; fold failure ratio into AdaptivePrior.
}
```

### 3.5 NEW: `communication/feishu/streaming.go` — Feishu streaming API (T27)

```go
package feishu

type StreamingEmitter interface {
    // EmitPartial sends a partial card with the idempotency key. If a
    // card with this key exists, it UPDATES the existing card (not
    // creates a new one). Returns ErrFeishuCardUpdateFailed on
    // transient errors; caller may retry up to 2 times.
    EmitPartial(ctx context.Context, key IdempotencyKey, content string, metadata map[string]string) error
    // EmitFinal sends the final card. Same idempotency key semantics.
    // On success, the prior partial card (if any) is replaced.
    EmitFinal(ctx context.Context, key IdempotencyKey, content string, metadata map[string]string) error
}

type IdempotencyKey string

func NewIdempotencyKey(sessionID, segmentID string) IdempotencyKey {
    return IdempotencyKey(fmt.Sprintf("%s:seg:%s", sessionID, segmentID))
}

func NewRollupIdempotencyKey(sessionID, parentID string) IdempotencyKey {
    return IdempotencyKey(fmt.Sprintf("%s:rollup:%s", sessionID, parentID))
}
```

Backed by Feishu's existing `card_instance` API:
- `POST /im/v1/messages` for new cards.
- `PATCH /im/v1/messages/{message_id}` for updates.

Implementation: keep a `map[IdempotencyKey]string` (key → feishu message_id)
within the StreamingEmitter so updates resolve to the right message.

### 3.6 NEW: `communication/feishu/streaming_dedup.go` — Emitter-side dedup (T28)

Same pattern as `sessionorchestrator/emit_dedup.go` but at the IM adapter
level. Defense-in-depth: even if sessionorchestrator's dedup is bypassed
(e.g. legacy Emit call path), the IM adapter drops duplicates.

Note: PR-B consensus Q7 ruled out executor-side dedup. The session-level
dedup (T28 here) is a different layer (IM-adapter), so it's allowed and
useful for network-retry resilience.

### 3.7 NEW sentinels (extend `internal/shared/errors/` or per-file)

```go
// ORCH_EMIT_DEDUP_HIT_7214 — debug-level; not user-facing
ErrEmitDedupHit = errors.New(...)

// ORCH_FEISHU_STREAMING_UPDATE_FAILED_7215
ErrFeishuStreamingUpdateFailed = errors.New(...)

// ORCH_FEISHU_STREAMING_CARD_NOT_FOUND_7216 — partial card expired/deleted
ErrFeishuStreamingCardNotFound = errors.New(...)

// ORCH_LEARN_PER_SEGMENT_FAILED_7217
ErrLearnPerSegmentFailed = errors.New(...)
```

All as `sharederrors.SentinelError` wrap helpers.

### 3.8 NOT in scope (deferred to PR-D, PR-E, PR-F)
- **PR-D**: `devrix.d7.dag_executor.enabled` config flag + e2e LP-3 + verify-archive
- **PR-E**: Learn 22-scenario coverage + ReputationRow DB migration + AsyncLearner
- **PR-F**: Plan 26-scenario coverage (P12-P17 Parse Reject handling)

### 3.9 Integration with Feishu v1 vs v2 streaming API (out-of-scope, flagged)
PR-C uses Feishu's existing `card_instance` API (v1, synchronous update).
Feishu v2 streaming API exists but PR-C does NOT adopt it — per
proposal.md §v2-2 "v2 触发条件: v1 灰度后". PR-C is the v1 streaming
path that runs in production today.

---

## 4. OPEN QUESTIONS

### Q1. Rollup idempotency key namespace — shared or isolated?
**(A)** Same keyspace: `rollup:` prefix on the parent ID; one shared dedup table.
**(B)** Different keyspace: separate `rollupDedup` table.
**(C)** No rollup dedup — IM adapter always replaces the latest card.

→ **Recommendation: (A)** — `rollup:<parent_id>` prefix is human-readable,
single dedup table, no risk of collision with `seg:<segment_id>` keys.
Proposal.md §4 Q4 already leans this way.

### Q2. Where does the dedup table live — sessionorchestrator, IM adapter, or both?
**(A)** sessionorchestrator only (single-source-of-truth, simple)
**(B)** IM adapter only (closer to network, defends against retry storms)
**(C)** Both (defense-in-depth)

→ **Recommendation: (C)** — IM adapter has its own dedup for network retry
resilience; sessionorchestrator has its own for in-process reentry safety.
Two layers, two failure modes. The cost is one extra O(1) lookup per emit.

### Q3. Partial card update rate-limiting
**(A)** No rate limit — emit as fast as DAG completes (1 partial per child, max 4-8 per session)
**(B)** Feishu API rate limit (10 RPS per chat_id) — throttle to 200ms spacing
**(C)** Per-segment coalesce — wait 500ms after first emit before subsequent updates

→ **Recommendation: (B)** — Feishu's documented rate limit is 10 RPS; with
4-worker hard cap, max 4 emits per session is well under this. Still, add
the throttle defensively for sessions with high child counts. Use
`golang.org/x/time/rate` limiter with token bucket 10 RPS.

### Q4. `LearnRequest.WorkItemID` field name conflict
The existing `WorkItem` type uses `ID` as the canonical identifier. `WorkItemID`
in LearnRequest might collide with `WorkItem.ID` when the request is for the
parent itself. Options:
**(A)** `WorkItemID` string + `IsRollup bool` — explicit, no collision.
**(B)** Reuse `WorkItemID` for parent only; per-segment uses `SegmentID string`.
**(C)** Single `SubjectID string` + `SubjectKind enum`.

→ **Recommendation: (A)** — `WorkItemID` matches the existing field name in
decisionplanning; explicit `IsRollup` flag is unambiguous.

### Q5. ParentEvidence folding rule for AdaptivePrior
The aggregator sums α/β across children. How does failure count fold into
AdaptivePrior?
**(A)** `FailureRatio = FailureCount / ChildCount` → bump prior toward "untrusted"
**(B)** Skip folding; reputation is independent
**(C)** Weighted by child confidence: `Σ(confidence * 1) / ChildCount`

→ **Recommendation: (A)** — simpler, matches the existing BayesianUpdate
semantics (`prior = basePrior * (1 - failureRatio)`). Confidence weighting
deferred to PR-E Learn 22-scenario.

### Q6. Failure semantics: per-child Learn failures don't block others?
**(A)** Yes — per-child Learn failures are logged + slog.Error, but other
children continue. PR-C implements this.
**(B)** No — first Learn failure aborts the whole DAG.

→ **Recommendation: (A)** — Learn is async post-completion; failure of one
shouldn't affect the others. Matches existing mups/learn semantics.

### Q7. Tests internal package or external?
**(A)** Internal `package sessionorchestrator` for DAG fork tests (need
   unexported `executePlanDAG` access)
**(B)** External `package sessionorchestrator_test` for emit-dedup tests
   (no internal access needed)

→ **Recommendation: (A)** for DAG fork + emit dedup; (B) for Feishu
streaming adapter (it's a public interface).

### Q8. Coverage target?
**(A)** ≥ 80% (project standard, devrix/testing.md)

→ **Recommendation: (A)** — 80% is the standard. The DAG fork has
~5 unit-testable functions; Feishu streaming has ~10. Net ~15 tests
across PR-C.

### Q9. PR-C scope reduction: defer Feishu streaming to PR-D?
**(A)** Keep streaming in PR-C (covers T27-T28)
**(B)** Defer to PR-D alongside e2e LP-3 (smaller PR-C)

→ **Recommendation: (A)** — Feishu streaming is the IM contract surface
that PR-C's emit-dedup depends on. Splitting hurts review clarity. PR-D
keeps config flag + e2e + verify-archive only.

### Q10. Single-PR or split-PR?
**(A)** Single PR-C covering all of T22-T28 (7 tasks, ~5 人天)
**(B)** Split into PR-C1 (Run() fork, T22-T24, 2.0d) + PR-C2 (Learn+Streaming, T25-T28, 3.0d)

→ **Recommendation: (A)** — single PR keeps the end-to-end slice visible
in one review. Splitting creates review fragmentation without reducing
risk; the components are interdependent (LearnRequest.WorkItemID needs
the DAG fork to drive it).

---

## 5. DELIVERABLES

1. `sessionorchestrator/item_pipeline.go` — `executePlanDAG` helper + DAG fork (~120 LOC)
2. `sessionorchestrator/emit_dedup.go` (NEW) — `EmitDedup` table (~50 LOC)
3. `mups/learn/learner.go` — `LearnRequest.WorkItemID/ParentID/IsRollup/Evidence` fields + `LearnPerSegment` helper (~80 LOC)
4. `mups/learn/reputation/parent_evidence.go` (NEW) — `ParentEvidence` + aggregator (~80 LOC)
5. `communication/feishu/streaming.go` (NEW) — `StreamingEmitter` interface + Feishu API client (~220 LOC)
6. `communication/feishu/streaming_dedup.go` (NEW) — IM-side dedup (~50 LOC)
7. Tests: 6 files, ~15 new tests, race-clean
8. Coverage ≥ 80% on new code paths
9. `go vet` clean, `go build ./...` clean
10. `go test -race ./internal/layers/orchestration/sessionorchestrator/...` passes
11. `go test -race ./internal/layers/communication/feishu/...` passes
12. `go test -race ./internal/layers/orchestration/mups/learn/...` passes
13. New sentinels 7214-7217 as `sharederrors.SentinelError`
14. New span IDs `D7-S15-PRC-*` registered

---

## 6. TEST MATRIX

| Scenario | Expected | Test |
|----------|----------|------|
| Single directive (Plan.DAG == nil) | Existing single-round Learn path unchanged | TestExecutePlanDAG_NilDAG_FallsThrough |
| Multi-intent 2 segments (success) | 2 partial emits + 1 rollup emit; 3 dedup hits total | TestExecutePlanDAG_2Segments_PartialAndRollup |
| Multi-intent 2 segments (1 fails) | 1 partial fail emit + 1 rollup fail emit; 2 dedup hits | TestExecutePlanDAG_2Segments_OneFails |
| Dedup hit | Repeat emit same key → no-op + slog.Debug | TestEmitDedup_DuplicateKey_NoOp |
| Dedup reset on session end | Reset() clears table | TestEmitDedup_ResetClearsTable |
| Feishu EmitPartial idempotency | Repeated call updates same card | TestFeishuStreaming_EmitPartial_Idempotent |
| Feishu EmitFinal overwrites partial | After EmitFinal, partial card no longer reachable | TestFeishuStreaming_EmitFinal_OverwritesPartial |
| Feishu rate limit | 11 rapid EmitPartial in 1s → 10 succeed, 1 throttled | TestFeishuStreaming_RateLimit_Throttles |
| LearnRequest WorkItemID | Per-segment attribution visible in Learn log | TestLearnPerSegment_WorkItemIDSet |
| ParentEvidence aggregator | Sum α/β across children | TestParentEvidence_SumAcrossChildren |
| Parent rollup Learn | Single Learn call with `IsRollup=true` + Evidence | TestLearnPerSegment_RollupHasEvidence |
| Per-child Learn failure | Other children continue; error logged | TestLearnPerSegment_OneFailureDoesNotBlockOthers |
| Span attributes D7-S15-PRC-* | Set on each emit | TestExecutePlanDAG_SpanAttributes |
| Empty DAG fork | Run() detects Plan.DAG with 0 nodes → returns nil round + noop | TestExecutePlanDAG_EmptyDAG_NoOp |

---

## 7. RISK REGISTER

| Risk | Severity | Mitigation |
|------|----------|-----------|
| EmitDedup key collision across reentry | Med | Dedup is per-session; reentry cancels prior wave (PR-B guarantee) |
| Feishu API rate limit exceeded under burst | Low | Token-bucket limiter 10 RPS |
| StreamingEmitter interface bloat | Low | 2 methods only (Partial, Final) — keep interface small |
| ParentEvidence aggregation race | Med | All children emit before rollup (DAG executor invariant); aggregator runs once after rollup emit |
| Per-child Learn ctx cancellation | Low | LearnPerSegment uses context.Background()-derived ctx; parent ctx cancel only via session end |
| Idempotency key for "" segmentID (rollup) | Low | Q1 recommendation: "rollup:" prefix; no collision with "seg:" prefix |
| Backward compat: existing tests breaking | Med | Plan.DAG nil → existing path unchanged; regression test on sessionorchestrator suite |
| Feishu v1 vs v2 streaming API drift | Low | v1 stable; v2 deferred to PR-F+; isolated behind StreamingEmitter interface |

---

## 8. CONSENSUS QUESTIONS (for codex/cursor)

Please respond with **ACCEPT / ADOPT-WITH-CHANGE / REJECT** for each:

1. **Q1**: Rollup idempotency key prefix `"rollup:"` (shared dedup table, no collision)?
2. **Q2**: Two-layer dedup (sessionorchestrator + Feishu adapter)?
3. **Q3**: Token-bucket rate limit 10 RPS for Feishu EmitPartial?
4. **Q4**: `LearnRequest.WorkItemID` + `IsRollup bool` (no SubjectID enum)?
5. **Q5**: ParentEvidence failure ratio `FailureCount/ChildCount` folds into AdaptivePrior?
6. **Q6**: Per-child Learn failures are non-blocking (logged, others continue)?
7. **Q7**: Tests split (internal for DAG fork, external for Feishu)?
8. **Q8**: Coverage ≥ 80%?
9. **Q9**: Sentinel codes 7214-7217 in `sharederrors.SentinelError`?
10. **Q10**: Single PR-C (not split into PR-C1 + PR-C2)?

---

## 9. REVIEW DELIVERABLES

After review, expected output:
- `reviews/pr-c-codex-consensus-2026-07-07.md`
- `reviews/pr-c-cursor-consensus-2026-07-07.md`
- Implementation follows adopted consensus