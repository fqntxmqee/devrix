# PR-C Consensus Review — codex (MiniMax-M3), 2026-07-07

**Change ID:** `devrix-d7-multi-intent-observation-decompose`
**Reviewer:** codex exec (MiniMax-M3)
**Packet:** `reviews/pr-c-consensus-packet.md`
**Tokens:** 96,559
**Status:** Single-reviewer consensus (cursor quota lockout until 2026-07-20)

---

## Q1 — Rollup idempotency key prefix `"rollup:"` (shared dedup table)

**ACCEPT.** Proposal.md §4 Q4 already leans this way; the `"rollup:" + parent_id` prefix is human-readable, single dedup table, and collision-free against `"seg:" + segment_id` keys. The per-session dedup lifecycle (allocated in `executePlanDAG`, reset on session end) covers the only reentry path.

No change.

## Q2 — Two-layer dedup (sessionorchestrator + Feishu adapter)

**ACCEPT-WITH-CHANGE.** Two-layer dedup is correct for different failure modes:
- sessionorchestrator layer: defends against in-process reentry (PR-B's
  `RunPlanDAG` reentry guarantees the prior wave is cancelled, but the
  cancel may race with the partial emit, so a redundant partial emit is
  possible and the dedup catches it).
- Feishu adapter layer: defends against network retries / partial-card
  failures from feishu_stream_throttle.

**Change**: rename `streaming_dedup.go` → `feishu_dedup.go` to avoid the
collision risk with the existing `feishu_stream_throttle.go` (same package,
similar name → grep confusion). Also: `EmitDedup.Reset()` has a race against
in-flight `MarkAndCheck` — `Reset` must hold the write lock AND wait for
in-flight readers to complete, OR use sync.Map. Recommend `sync.Map.LoadOrStore`
+ atomic counter for last-seen-at; `Reset()` just swaps in a fresh map.

## Q3 — Token-bucket rate limit 10 RPS for Feishu EmitPartial

**REJECT.** Two errors in the packet:

1. **Wrong surface**: `golang.org/x/time/rate` is not in `go.mod`. Codebase
   ships `internal/layers/communication/channel/ratelimit/limiter.go:39-47`
   with `channel/ratelimit.RateLimiter` (100 RPM, 10 burst) and
   `feishu_stream_throttle.go` (400ms / 24-rune content throttle). PR-C must
   reuse the existing throttler, not introduce a parallel one.

2. **Wrong limit**: Feishu cardkit is **per-card 10 QPS**, not per-chat 10 RPS.
   Per-chat limits are much higher.

**Adopted**: PR-C wraps the existing `feishu_stream_throttle.go`'s
`StreamRateLimiter`; it already gates per-card content emission with the
correct semantics. No new rate-limit primitive.

## Q4 — `LearnRequest.WorkItemID` + `IsRollup bool`

**ACCEPT-WITH-CHANGE.** `WorkItemID` matches the existing field name in
`decisionplanning`; `IsRollup bool` is unambiguous.

**Change**: rename `WorkItemID` → `SegmentID` in the per-segment request
struct, since `WorkItemID` is the parent's ID and `SegmentID` is the
child segment. The two are distinct in the multi-intent world:
- `LearnRequest{ParentID: "wi-123", SegmentID: "seg_a", IsRollup: false}` — per-child Learn
- `LearnRequest{ParentID: "wi-123", IsRollup: true}` — parent rollup Learn (no SegmentID)

This avoids the confusion in §3.3 where `WorkItemID` was used for both parent
and child.

## Q5 — ParentEvidence failure ratio folds into AdaptivePrior

**REJECT.** `prior * (1 - failureRatio)` scalar-multiplies `BetaPrior.Alpha/Beta`,
which doesn't commute with the Wilson interval math at
`reputation/evidence.go:148`. The BayesianUpdate is order-sensitive.

**Adopted**: synthesize a single `Verdict{Kind: VerdictFail if FailureCount > 0}`
at rollup time and call `BayesianUpdate` once with the rollup prior. Drop
the `AdaptivePrior` field from `ParentEvidence` (PR-C scope reduction).

Per-child failure already flows through the existing `BayesianUpdate` via
`Beta++` on each child's `VerdictPass → fail` flip; rollup is just an
aggregation layer on top.

## Q6 — Per-child Learn failures are non-blocking

**ACCEPT.** Matches existing `mups/learn` semantics: `DefaultLearner` swallows
per-call errors with `slog.Error` and continues. PR-C's `LearnPerSegment`
helper wraps the same pattern.

No change.

## Q7 — Tests split (internal for DAG fork, external for Feishu)

**ACCEPT.** Internal-package tests for `executePlanDAG` (needs unexported
helper access); external-package for `FeishuAdapter` (public interface only).
Existing `sessionorchestrator/*_test.go` files are internal; matches the
project pattern.

No change.

## Q8 — Coverage ≥ 80%

**ACCEPT.** Project standard. PR-C adds ~15 tests across DAG fork (5),
Learn per-segment (4), Feishu streaming (4), and helpers (2). 80% target
is achievable.

No change.

## Q9 — Sentinel codes 7214-7217 in `sharederrors.SentinelError`

**ACCEPT-WITH-CHANGE.** Codes are consistent with PR-B's 7210-7213 family.

**Change**: skip 7214 (`ORCH_EMIT_DEDUP_HIT_7214`) — dedup hit is a debug
log, not an error. Reserve 7214 for a future real error sentinel. So PR-C
ships 3 sentinels (7215-7217) for the Feishu streaming + Learn per-segment
errors; 7214 stays available.

## Q10 — Single PR-C (not split into PR-C1 + PR-C2)

**ACCEPT.** Single PR keeps the end-to-end slice visible. The components are
interdependent: `LearnRequest.SegmentID` needs the DAG fork to drive it;
`EmitDedup` keys need both DAG fork and Feishu adapter. Splitting creates
review fragmentation without reducing risk.

No change.

---

## New risks beyond §7 RISK REGISTER

### Risk A1 (HIGH) — `card_instance` API does not exist

**Code:** `internal/layers/communication/` directory tree.
**Issue:** The packet's §3.5 references `card_instance` (`POST/PATCH /im/v1/messages`)
which does not exist in this codebase. Production uses
`internal/layers/communication/channel/adapters/feishu_cardkit.go`
(`/open-apis/cardkit/v1/cards` + `StreamElementContent` + `UpdateCard`).
**Mitigation:** Drop `streaming.go` + `streaming_dedup.go` (the new files
proposed in §3.5 and §3.6) and add two methods to the existing `FeishuAdapter`:
- `EmitPartialCard(ctx, chatID, idempotencyKey, content) error`
- `EmitFinalCard(ctx, chatID, idempotencyKey, content) error`

Both methods internally call `cardkit.v1.cards` (POST for new,
PATCH for update) and reuse the existing `feishu_progress.go::finalizeReplyCardStreaming`
as the integration point.

### Risk A2 (HIGH) — `LearnRequest.SegmentID` does not reach AssetBuilder

**Code:** `internal/layers/orchestration/mups/learn/asset/asset_builder.go:102-128`.
**Issue:** The packet's T25 adds `LearnRequest.SegmentID` but `AssetBuilder.Build`
does not consume it. Per-segment attribution goal is unmet.
**Mitigation:** Add `LearningAsset.SourcePlanNodeIDs []string` field; wire
`Build()` to populate it from `LearnRequest.SegmentID` + rollup aggregation;
propagate through `WithSourceVerdictIDs` so the reputation store records
which segments contributed to the rollup.

### Risk A3 (HIGH) — `LearnPerSegment` ctx leak

**Code:** Packet §7 "Per-child Learn ctx cancellation" row.
**Issue:** Packet §7 says `LearnPerSegment` derives ctx from
`context.Background()`. That leaks goroutines on session-end.
**Mitigation:** `LearnPerSegment` MUST derive ctx from `executePlanDAG`'s ctx
(per-segment Learn is fire-and-forget BUT inherits cancellation so session-end
triggers a clean shutdown of all in-flight Learn calls).

### Risk A4 (MEDIUM) — `BayesianUpdate` nil-prior deref

**Code:** `internal/layers/orchestration/mups/learn/reputation/evidence.go:115`
`next := *prior` unconditional; `InMemoryReputationStore.Get` returns `(nil, nil)`
on cold start.
**Issue:** `DefaultLearner` guards this at `learner.go:296-308` but the guard
is shallow — any direct `BayesianUpdate` caller inherits the panic risk.
PR-C's per-segment calls amplify this latent bug.
**Mitigation:** Signature change to `(*ReputationEvidence, error)` with
explicit nil check; `DefaultLearner` returns `(nil, nil)` from `Get` and
`Learn` callers handle the cold-start case via `BetaPrior{Alpha:1, Beta:1}`
default. Plus regression test on the existing per-WorkItem Learn path.

### Risk A6 (MEDIUM) — Span naming convention drift

**Code:** `internal/layers/orchestration/d7spans/` or similar.
**Issue:** Packet §2.2 proposes `D7-S15-PRC-*` span IDs; t-registry uses
`D7_<Component>_<Action>` format. Mixing the two breaks Jaeger queries.
**Mitigation:** Five new span ops must register in `spans.go`:
- `D7_DAG_Executor_Stream_Emit`
- `D7_Emit_Dedup_Mark`
- `D7_Streaming_Emitter_Partial`
- `D7_Streaming_Emitter_Final`
- `D7_Learn_Per_Segment`

T-points (D7-S15-A50-T01..T07) go in `t-registry.md` separately.

### Risk A7 (MEDIUM) — Verify→Learn invariant violation

**Code:** `internal/layers/orchestration/sessionorchestrator/item_pipeline.go:554`
**Issue:** Per-segment Learn firing immediately after each child completion
violates the existing Verify→Learn invariant (Verify must run before Learn
to attach AC verdicts).
**Mitigation:** Two-phase Learn:
- **Phase 1 (PR-C)**: synchronous-after-emit, ExitCode-only Verdict per child.
  This is the "early Learn" for partial-card observability — it shows users
  "child X completed" but the full AC verdict is deferred.
- **Phase 2 (PR-E)**: full Learn with AC verdicts after Verify completes.
  Uses `ReputationStore.ReverseUpdate` to undo Phase 1's premature verdict
  and re-apply with the correct verdict.

PR-C ships Phase 1 only; Phase 2 deferred to PR-E Learn 22-scenario.

### Risk A8 (HIGH) — `StrategicPlanProposal` has no `DAG` field

**Code:** `internal/layers/orchestration/plan/strategic_plan_proposer.go:342-360`
**Issue:** The packet's §3.1 fork `proposal.DAG != nil` and tasks.md T22
(`Run() 检测 StrategicPlanProposal.DAG != nil`) cannot compile.
The DAG lives on `plan.Plan.DAG` (`plan_struct.go:58`), not on
`StrategicPlanProposal`.
**Mitigation:** Correct fork: `pl.DAG != nil && pl.IntentSegmentSet != nil`
(matching PR-B's `dag_executor_errors.go:39` for the segment-set presence
check). Update packet §3.1 and tasks.md T22 wording.

### Risk A9 (LOW) — `streaming_dedup.go` naming collision

**Code:** New file proposal.
**Issue:** Naming collision with existing `feishu_stream_throttle.go` creates
grep confusion.
**Mitigation:** Rename to `feishu_dedup.go` (matches Q2 ADOPT-WITH-CHANGE).

---

## Adopted (summary)

- Q1: ACCEPT — `"rollup:" + parent_id` prefix
- Q2: ACCEPT-WITH-CHANGE — `sync.Map` for thread-safety + rename to `feishu_dedup.go`
- Q3: REJECT — reuse `feishu_stream_throttle.go`; no new rate-limit primitive
- Q4: ACCEPT-WITH-CHANGE — rename `WorkItemID` → `SegmentID` in per-segment struct
- Q5: REJECT — synthesize single rollup Verdict; drop `AdaptivePrior` field
- Q6: ACCEPT — non-blocking per-child Learn
- Q7: ACCEPT — internal/external test split
- Q8: ACCEPT — 80% coverage
- Q9: ACCEPT-WITH-CHANGE — sentinels 7215-7217 (skip 7214, reserve for future)
- Q10: ACCEPT — single PR-C

## Cursor status

Cursor-agent quota lockout continues from PR-A1 (resets 2026-07-20).
Single-reviewer consensus. No blockers for PR-C start.

---

## Implementation TODOs (adopted)

- [ ] `sessionorchestrator/item_pipeline.go` — `executePlanDAG` helper (~120 LOC), fork on `pl.DAG != nil && pl.IntentSegmentSet != nil` (NOT `proposal.DAG` per Risk A8)
- [ ] `sessionorchestrator/emit_dedup.go` (NEW) — `EmitDedup` using `sync.Map` + atomic counter (NOT plain `map + sync.RWMutex` per Risk A2)
- [ ] `mups/learn/learner.go` — `LearnRequest.SegmentID/ParentID/IsRollup/Evidence` fields; rename `WorkItemID` → `SegmentID` for per-segment
- [ ] `mups/learn/reputation/parent_evidence.go` (NEW) — `ParentEvidence` aggregator WITHOUT `AdaptivePrior` field (per Risk A5)
- [ ] `mups/learn/reputation/evidence.go` — change `BayesianUpdate` signature to `(*ReputationEvidence, error)` with explicit nil check (Risk A4)
- [ ] `mups/learn/asset/asset_builder.go` — add `LearningAsset.SourcePlanNodeIDs` field; wire `Build()` to populate from `LearnRequest.SegmentID` (Risk A2)
- [ ] `communication/channel/adapters/feishu_cardkit.go` — add `EmitPartialCard` + `EmitFinalCard` methods to existing `FeishuAdapter` (Risk A1 — NO new `streaming.go` file)
- [ ] `communication/channel/adapters/feishu_dedup.go` (NEW, renamed from `streaming_dedup.go`) — IM-side dedup using `sync.Map`
- [ ] Span registration in `d7spans/spans.go`: `D7_DAG_Executor_Stream_Emit`, `D7_Emit_Dedup_Mark`, `D7_Streaming_Emitter_Partial`, `D7_Streaming_Emitter_Final`, `D7_Learn_Per_Segment` (Risk A6)
- [ ] Two-phase Learn: Phase 1 synchronous-after-emit (PR-C); Phase 2 with AC verdicts (PR-E)
- [ ] `LearnPerSegment` derives ctx from `executePlanDAG` ctx (Risk A3)
- [ ] Sentinels 7215-7217 as `sharederrors.SentinelError` wrap helpers (skip 7214 per Q9)
- [ ] Tests: 15 new unit tests across sessionorchestrator, mups/learn, communication/feishu
- [ ] Regression: existing tests on `sessionorchestrator/`, `mups/learn/`, `communication/feishu/` stay green
- [ ] Coverage ≥ 80% on new code paths
- [ ] `go vet` clean, `go build ./...` clean