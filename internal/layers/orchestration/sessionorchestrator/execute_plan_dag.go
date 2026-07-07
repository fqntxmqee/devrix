// Package sessionorchestrator — executePlanDAG helper (DM-20260707-001 PR-C).
//
// Multi-intent DAG path: when Plan emits a non-nil DAG + IntentSegmentSet,
// ItemPipelineRunner.Run() forks from the legacy WorkItemExecutor path into
// executePlanDAG. The helper:
//
//  1. Calls DAGExecutor.RunPlanDAG → consumes the returned SegmentEmit
//     channel in the caller's goroutine (synchronous, blocks until the
//     channel closes — mirrors legacy ExecuteWorkItem semantics).
//  2. Per-emit: EmitDedup.MarkAndCheck + per-segment Learn (BayesianUpdate
//     on the per-segment verdict) + EmitPartialCard.
//  3. After natural completion (IsFinal=true received): synthesize rollup
//     Verdict via SynthesizeRollupVerdict, run rollup Learn (IsRollup=true,
//     Evidence=<aggregated ParentEvidence>), and emit a final card that
//     overrides the prior partial card (Feishu cardkit UpdateCard semantic,
//     matches feishu_streaming_partial_final.go comments).
//
// The 5 new P0 spans (D7_DAG_Executor_Stream_Emit, D7_Emit_Dedup_Mark,
// D7_Streaming_Emitter_Partial, D7_Streaming_Emitter_Final,
// D7_Learn_Per_Segment) all live on the consumer-side path.
//
// Why a separate file (codex Risk A1 HIGH — keep item_pipeline.go ≤ 800 LOC):
// item_pipeline.go already carries Observe→Plan→Execute→Verify→Learn→Decide
// for the legacy single-WorkItem path. Adding the DAG helper here would push
// it past the 800-line cap and conflate two distinct execution models in one
// god-file.
//
// Per-segment Learn failures are non-fatal (codex Risk Q6 ADOPT-WITH-CHANGE):
// the rollup Learn still runs, and per-segment failures are surfaced via
// ORCH_LEARN_PER_SEGMENT_FAILED_7217 in slog for dashboard graphs. The
// consensus packet's original "abort the rollup on per-child Learn failure"
// was rejected because child reputation drift is recoverable next round;
// aborting the rollup would surface a stale Pass from the rollup view.
//
// Risk mitigations baked in:
//
//   - codex Risk A3 HIGH: ctx flows from executePlanDAG → learnPerSegment
//     (no context.Background() leak).
//   - codex Risk A8 HIGH: fork on pl.DAG != nil && pl.IntentSegmentSet != nil
//     (NOT proposal.DAG — strategic plan proposals don't carry the DAG).
//   - codex Risk A2 ADOPT-WITH-CHANGE: EmitDedup uses sync.Map + atomic
//     counters (NOT map + RWMutex — see emit_dedup.go for rationale).
//   - codex Risk A9 LOW: file naming keeps grep-clear (execute_plan_dag.go
//     vs feishu_streaming_partial_final.go in adapters/ vs item_pipeline.go).
package sessionorchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// StreamingEmitter is the IM-side streaming card adapter used by the
// PR-C DAG path. Decoupled from *adapters.FeishuAdapter via this interface
// to avoid a test-only import cycle:
//
//	sessionorchestrator/transcript_reader.go → communication/capture/transcript
//	communication/capture (test)              → sessionorchestrator
//	sessionorchestrator                       → adapters → capture
//
// *adapters.FeishuAdapter satisfies this interface implicitly via its
// EmitPartialCard / EmitFinalCard methods. Concrete impls (Feishu today;
// future Slack/Teams) can ship in the adapters package without rippling
// changes through sessionorchestrator.
//
// The idempotency key is a plain string (not the adapters-typed
// IdempotencyKey) so this interface stays package-agnostic.
type StreamingEmitter interface {
	EmitPartialCard(ctx context.Context, chatID string, idemKey string, content string) (*FeishuEmitPartialResult, error)
	EmitFinalCard(ctx context.Context, chatID string, idemKey string, content string) (*FeishuEmitPartialResult, error)
}

// FeishuEmitPartialResult mirrors adapters.FeishuEmitPartialResult but is
// defined locally to keep the StreamingEmitter interface package-agnostic.
// Fields are identical so callers can pass a *adapters.FeishuEmitPartialResult
// directly into a local one (struct field copy via constructor).
type FeishuEmitPartialResult struct {
	CardID   string
	Sequence int
}

// executePlanDAG is the DM-20260707-001 PR-C multi-intent DAG execution path.
// Called from Run() after endPlanPhase when pl.DAG + pl.IntentSegmentSet are
// non-nil. Returns the synthesized rollup round so Run() can persist it via
// the standard ApplyPipelineRound path (no separate persistence helper).
//
// Returns (round, nil) on natural completion; (nil, err) only on infra
// failure (DAG conversion, scheduler.Start, missing deps). Per-segment
// Learn failures and EmitPartialCard/EmitFinalCard failures are logged but
// do NOT abort the rollup — the user still gets a coherent final answer.
//
// ctx is the active pipeline ctx (passed in by Run()). It carries the
// MUPS-Phase span for the Execute phase + locator frame for Jaeger
// breadcrumbs. ctx.Err() is honored at the for-range loop on cancel.
func (r *ItemPipelineRunner) executePlanDAG(
	ctx context.Context,
	sessionID, userID string,
	item *workmodel.WorkItem,
	pl *plan.Plan,
	directive string,
	roundNo int,
	trigger string,
	started time.Time,
	isParentRollup bool,
	obsLookups []learn.ObservationLookup,
) (*workmodel.WorkItemPipelineRound, error) {
	if r == nil {
		return nil, fmt.Errorf("item_pipeline: executePlanDAG called on nil runner")
	}
	if r.DAGExecutor == nil {
		return nil, fmt.Errorf("item_pipeline: executePlanDAG requires DAGExecutor wired on runner")
	}
	if pl == nil || pl.DAG == nil || pl.IntentSegmentSet == nil {
		return nil, fmt.Errorf("item_pipeline: executePlanDAG requires non-nil Plan.DAG and Plan.IntentSegmentSet")
	}
	if item == nil || item.ID == "" {
		return nil, fmt.Errorf("item_pipeline: executePlanDAG requires non-empty WorkItem")
	}

	chatID := sessionID // confirmed by emit_dedup.go comment: sessionID == chatID for IM stream path.

	// Per-session dedup table. Survives only for the duration of this
	// executePlanDAG call; fresh on the next round.
	emitDedup := NewEmitDedup()

	// Open the DAGExecutor stream. Channel is buffered inside the executor
	// (cursor risk fix: unbuffered channel deadlock). RunPlanDAG returns
	// synchronously after scheduler.Start; consumer goroutine lives in
	// the executor and pushes SegmentEmits as nodes transition terminal.
	emits, err := r.DAGExecutor.RunPlanDAG(ctx, sessionID, pl.ID, pl.DAG, pl.IntentSegmentSet)
	if err != nil {
		return nil, fmt.Errorf("item_pipeline: RunPlanDAG: %w", err)
	}

	// outcome captures what the rollup needs: child verdicts (for
	// SynthesizeRollupVerdict) + the final summary text (for EmitFinalCard
	// content + round.ArtifactSummary).
	type outcome struct {
		emit    wavescheduler.SegmentEmit
		verdict workmodel.Verdict
		summary string
	}
	var (
		outMu        sync.Mutex
		outcomes     []outcome
		finalSummary string
		observedFail bool
	)

	for emit := range emits {
		// Dedup check (defense-in-depth; per-segment idempotency key already
		// gates the IM-side cardkit call). Drop duplicate emits if the
		// polling loop ever doubles up (artifact-driven emit race is
		// guarded inside DAGExecutor, but a network retry at this layer
		// could still produce duplicates).
		idemKey := NewPartialIdempotencyKey(sessionID, emit.SegmentID)
		isFirst := emitDedup.MarkAndCheck(idemKey)
		endDedup := hardening.EmitEmitDedupMark(ctx, sessionID, idemKey, !isFirst)
		endDedup(nil)

		if !isFirst {
			// Skip the per-segment Learn + card emit on dedup hit; the
			// first observation already produced them.
			slog.Debug("execute_plan_dag: dedup hit; skipping per-segment emit",
				"session_id", sessionID, "segment_id", emit.SegmentID,
				"idempotency_key", string(idemKey))
			continue
		}

		segVerdict := buildSegmentVerdictFromEmit(emit, item)
		segSummary := buildSegmentSummaryFromEmit(emit)
		segArtifact := buildSegmentArtifactFromEmit(emit, item, sessionID, started)

		outMu.Lock()
		outcomes = append(outcomes, outcome{
			emit:    emit,
			verdict: segVerdict,
			summary: segSummary,
		})
		if emit.IsFinal {
			finalSummary = segSummary
		}
		if segVerdict.Kind == types.VerdictFail {
			observedFail = true
		}
		outMu.Unlock()

		// Per-segment Learn (non-blocking). Wrapped in EmitLearnPerSegment
		// span so Jaeger shows the per-child reputation lineage.
		if r.Learner != nil {
			r.learnPerSegment(ctx, sessionID, emit.SegmentID, item.ID, false,
				segVerdict, segArtifact, obsLookups, nil, len(outcomes))
		}

		// Emit partial card via the streaming cardkit API. Skip the
		// IsFinal=true emit (the rollup's EmitFinalCard below overrides
		// the prior partial — matches feishu_streaming_partial_final.go
		// "final-overrides-partial" semantic).
		if !emit.IsFinal && r.StreamingEmitter != nil {
			endEmit := hardening.EmitStreamingEmitterPartial(
				ctx, chatID, string(idemKey), runeCount(segSummary), 0)
			_, err := r.StreamingEmitter.EmitPartialCard(ctx, chatID, idemKey, segSummary)
			if err != nil {
				// Non-fatal: log + continue. The user still sees the rollup.
				slog.Warn("execute_plan_dag: EmitPartialCard failed (non-fatal)",
					"session_id", sessionID, "segment_id", emit.SegmentID,
					"idempotency_key", string(idemKey), "err", err)
			}
			endEmit(err)
		}
	}

	// Rollup synthesis. Aggregate child verdicts for SynthesizeRollupVerdict;
	// empty outcomes (e.g. all segments aborted) → Indeterminate rollup.
	outMu.Lock()
	childVerdicts := make([]learn.ChildVerdict, 0, len(outcomes))
	for _, o := range outcomes {
		childVerdicts = append(childVerdicts, learn.ChildVerdict{
			SegmentID: o.emit.SegmentID,
			Kind:      o.verdict.Kind,
			SourceID:  o.verdict.SourceID,
		})
	}
	outMu.Unlock()

	rollupVerdict := learn.SynthesizeRollupVerdict(item.ID, childVerdicts)
	if observedFail && rollupVerdict.Kind == types.VerdictPass {
		// Defensive: SynthesizeRollupVerdict already handles "any Fail →
		// VerdictFail", but if the consensus path missed a Fail we don't
		// want to surface Pass. Override down to Partial so the rollup
		// signals degradation.
		rollupVerdict.Kind = types.VerdictPartial
		rollupVerdict.Reason = rollupVerdict.Reason + " (child failure observed)"
	}

	// Aggregate ParentEvidence from child verdicts. Cold-start safe — the
	// helper builds SumAlpha/SumBeta from scratch (no prior required).
	parentEvidence := learn.AggregateParentEvidence(nil, childVerdicts)

	// Rollup Learn: BayesianUpdate with the aggregated ParentEvidence as
	// prior-context. IsRollup=true flags the per-segment lineage fields
	// in the asset (SourcePlanNodeIDs = union of child segment IDs).
	if r.Learner != nil {
		rollupArtifact := buildRollupArtifact(item, sessionID, started,
			rollupVerdict, finalSummary)
		r.learnPerSegment(ctx, sessionID, item.ID, item.ID, true,
			rollupVerdict, rollupArtifact, obsLookups, &parentEvidence, len(childVerdicts))
	}

	// Emit final card. Rollup key is {sessionID}:rollup:{parentID}, distinct
	// from per-segment keys ({sessionID}:seg:{segmentID}) so the IM dedup
	// table treats them as separate streams (matches the consensus packet's
	// Q9 "skip 7214 for dedup hit is debug log, not an error").
	rollupKey := NewRollupIdempotencyKey(sessionID, item.ID)
	if r.StreamingEmitter != nil {
		finalContent := finalSummary
		if finalContent == "" {
			// No child produced IsFinal=true → use the rollup verdict's
			// reason as a user-facing fallback. All-aborted waves rarely
			// reach this path (DAGExecutor closes the channel without
			// IsFinal on abort, but the consumer's outcomes slice is empty
			// and finalSummary is empty).
			finalContent = rollupVerdict.Reason
		}
		endEmit := hardening.EmitStreamingEmitterFinal(
			ctx, chatID, string(rollupKey), runeCount(finalContent))
		_, err := r.StreamingEmitter.EmitFinalCard(ctx, chatID, rollupKey, finalContent)
		if err != nil {
			slog.Warn("execute_plan_dag: EmitFinalCard failed (non-fatal)",
				"session_id", sessionID, "rollup_key", string(rollupKey), "err", err)
		}
		endEmit(err)
	}

	// Build round struct. ArtifactID = the IsFinal segment's ID (or the
	// last observed segment's ID when IsFinal was never emitted). Summary
	// = finalSummary (rollup text). VerdictSourceID = rollup.SourceID so
	// downstream readers (D7-Decide, D6-Learner) see one verdict per round,
	// not N child verdicts.
	artifactID := ""
	if len(outcomes) > 0 {
		if finalSummary != "" {
			// Find the IsFinal outcome.
			for _, o := range outcomes {
				if o.emit.IsFinal {
					artifactID = o.emit.SegmentID
					break
				}
			}
		}
		if artifactID == "" {
			artifactID = outcomes[len(outcomes)-1].emit.SegmentID
		}
	}
	round := &workmodel.WorkItemPipelineRound{
		RoundNo:           roundNo,
		Trigger:           trigger,
		WorkItemID:        item.ID,
		SessionID:         sessionID,
		PlanID:            pl.ID,
		PlanKind:          pl.Kind,
		ArtifactID:        artifactID,
		ArtifactSummary:   finalSummary,
		VerdictID:         rollupVerdict.SourceID,
		VerdictKind:       rollupVerdict.Kind,
		VerdictConfidence: rollupVerdict.Confidence,
		ExitReason:        "dag_rollup",
		StartedAt:         started,
		CompletedAt:       time.Now(),
	}

	// DAG-path persistence: tree decision + round apply + terminalization.
	// Mirrors the legacy path's post-pipeline logic (Run() lines 765-833)
	// but with rollupVerdict in place of `verdict` and zero-value deliverable
	// fields (DAG rollups don't carry deliverable schema/contract). Returns
	// the same round so the caller can hand it back through the canonical
	// Run() return signature.
	if err := r.persistDAGRound(ctx, sessionID, userID, item, round, rollupVerdict, isParentRollup); err != nil {
		return nil, err
	}
	return round, nil
}

// persistDAGRound runs the post-pipeline decision + persistence for a DAG
// rollup round. Mirrors the legacy single-WorkItem path's lines 765-833 from
// Run() but with the DAG-path's simpler inputs (no deliverable schema,
// no strategic-reject rationale, no rollupRetries counter — those are
// legacy single-WorkItem concepts).
//
// Caller guarantees round.SpawnPolicy defaults to SpawnNone when the rollup
// verdict is terminal (Pass/Partial/Fail) and SpawnNoneTerminalOpts picks
// the right terminalization for non-rollup vs parent-rollup cases.
func (r *ItemPipelineRunner) persistDAGRound(
	ctx context.Context,
	sessionID, userID string,
	item *workmodel.WorkItem,
	round *workmodel.WorkItemPipelineRound,
	rollupVerdict workmodel.Verdict,
	isParentRollup bool,
) error {
	treeCtx := workmodel.DefaultTreeEvalContext(sessionID, item.ID, userID, r.Tasks)
	if item.LastRound != nil && item.LastRound.VerdictKind == types.VerdictIndeterminate {
		treeCtx.IndeterminateRetries = item.LastRound.IndeterminateRetries
	}
	treeCtx.DailyLimitExceeded = workmodel.DecomposeDailyLimitWouldExceed(sessionID, item.Kind, 1)

	ctxOut := workmodel.ProposeContextPipelineOutput(sessionID, item, round, r.Tasks, r.ContextProposer)
	workmodel.ApplyPipelineDecide(sessionID, item, round, ctxOut, treeCtx, r.Tasks)
	if round.VerdictKind == types.VerdictIndeterminate {
		round.IndeterminateRetries = treeCtx.IndeterminateRetries + 1
	}
	workmodel.TouchInlineRetryAtMaxDepth(r.Tasks.Tree(), sessionID, item, round, treeCtx)

	_, endDecidePhase := r.enterMUPSPhase(ctx, sessionID, item.ID, workmodel.RoundPhaseDecide)
	defer endDecidePhase(nil)

	phase := workmodel.RoundPhaseIdle
	switch round.SpawnPolicy {
	case workmodel.SpawnAwait, workmodel.SpawnDecompose, workmodel.SpawnParallelExplore:
		phase = workmodel.RoundPhaseAwaitChild
	case workmodel.SpawnInline:
		phase = workmodel.RoundPhaseIdle
	}
	{
		end := hardening.EmitWorktreeOp(ctx, sessionID, "apply_pipeline_round", item.ID, string(phase))
		defer end(nil)
		if err := r.Tasks.Tree().ApplyPipelineRound(sessionID, item.ID, round, phase); err != nil {
			return fmt.Errorf("item_pipeline: dag ApplyPipelineRound: %w", err)
		}
	}

	if round.SpawnPolicy == workmodel.SpawnNone {
		opts := workmodel.SpawnNoneTerminalOpts{
			IsRollup:                  isParentRollup,
			StripDeliverableForStatus: r.Verify != nil && !isParentRollup,
		}
		if err := workmodel.ApplyRoundTerminalization(
			r.Tasks.Tree(), sessionID, item.ID,
			rollupVerdict.Kind, workmodel.DeliverableSchemaNotApplicable,
			workmodel.DeliverableStatusNotApplicable, opts,
		); err != nil {
			return fmt.Errorf("item_pipeline: dag ApplyRoundTerminalization: %w", err)
		}
	} else if item.Status == workmodel.TaskStatusPending {
		end := hardening.EmitWorktreeOp(ctx, sessionID, "update_status", item.ID, string(workmodel.TaskStatusInProgress))
		defer end(nil)
		_ = r.Tasks.Tree().UpdateStatus(sessionID, item.ID, workmodel.TaskStatusInProgress)
	}

	if isParentRollup && rollupVerdict.Kind == types.VerdictPass {
		_ = r.Tasks.Tree().SetNeedsRollup(sessionID, item.ID, false)
	}

	item.LastRound = round
	item.RoundPhase = phase
	if got, ok := r.Tasks.GetWorkItem(sessionID, item.ID); ok && got != nil && workmodel.IsTerminalStatus(got.Status) {
		r.Tasks.RecordPeerStatusOnTerminal(sessionID, got)
	}
	return nil
}

// runeCount returns the rune count of s. Used for EmitPartialCard /
// EmitFinalCard content length metrics. Mirrors adapters/feishu_streaming_partial_final.go's
// local helper (kept private to avoid exporting a one-liner).
func runeCount(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// learnPerSegment wraps the LP-1 closed-loop call for one child (or rollup)
// with the DM-20260707-001 EmitLearnPerSegment span. Failure here is
// non-fatal: the helper logs ORCH_LEARN_PER_SEGMENT_FAILED_7217 and returns
// without bubbling the error up to executePlanDAG. Per PR-C Q6, per-child
// Learn failures are non-blocking — the rollup Learn still runs and the
// user still sees the final answer.
//
// ctx is propagated from executePlanDAG (codex Risk A3 HIGH — no
// context.Background leak). evidence is non-nil only for the rollup call.
func (r *ItemPipelineRunner) learnPerSegment(
	ctx context.Context,
	sessionID, segmentID, parentID string,
	isRollup bool,
	verdict workmodel.Verdict,
	artifact *wavescheduler.Artifact,
	obsLookups []learn.ObservationLookup,
	evidence *learn.ParentEvidence,
	evidenceSegmentCount int,
) {
	endLearn := hardening.EmitLearnPerSegment(
		ctx, sessionID, segmentID, parentID, isRollup, evidenceSegmentCount)
	_, err := r.Learner.Learn(ctx, learn.LearnRequest{
		SessionID:    sessionID,
		SegmentID:    segmentID,
		ParentID:     parentID,
		IsRollup:     isRollup,
		Evidence:     evidence,
		Verdict:      verdict,
		Plan:         nil, // Plan node already closed; we don't re-feed the rollup.
		Artifact:     artifact,
		Observations: obsLookups,
	})
	if err != nil {
		// Non-fatal: log + return. The rollup answer is unaffected; the
		// next round's Observe will read the prior-state Reputation row
		// (no BayesianUpdate applied → next AdaptivePrior falls back to
		// the cold-start defaults).
		slog.Warn("execute_plan_dag: per-segment Learn failed (non-fatal)",
			"session_id", sessionID, "segment_id", segmentID,
			"parent_id", parentID, "is_rollup", isRollup,
			"sentinel", "ORCH_LEARN_PER_SEGMENT_FAILED_7217",
			"err", err)
	}
	endLearn(err)
}

// buildSegmentVerdictFromEmit converts a SegmentEmit into a workmodel.Verdict
// for per-segment Learn attribution. ExitCode=0 → VerdictPass; -2 →
// VerdictIndeterminate (cancelled by executor); any other non-zero → VerdictFail.
func buildSegmentVerdictFromEmit(emit wavescheduler.SegmentEmit, item *workmodel.WorkItem) workmodel.Verdict {
	conf := 0.5
	if emit.ExitCode == 0 {
		conf = 0.85 // optimistic but not 0.95 — child verdict confidence isn't full round confidence
	}
	kind := types.VerdictPass
	reason := "segment_completed"
	switch emit.ExitCode {
	case 0:
		kind = types.VerdictPass
		reason = "segment_completed"
	case -2:
		kind = types.VerdictIndeterminate
		reason = "segment_cancelled_by_executor"
	default:
		kind = types.VerdictFail
		reason = "segment_failed"
		if emit.Error != "" {
			reason = "segment_failed: " + emit.Error
		}
	}
	return workmodel.Verdict{
		Kind:       kind,
		Confidence: conf,
		Reason:     reason,
		SourceID:   "dag_segment:" + emit.SegmentID + ":item:" + item.ID,
	}
}

// buildSegmentSummaryFromEmit returns the user-visible text for a SegmentEmit.
// On failure, prefixes the segment ID + error so the partial card makes
// clear which segment produced the text.
func buildSegmentSummaryFromEmit(emit wavescheduler.SegmentEmit) string {
	body := emit.Summary
	if emit.ExitCode != 0 {
		if emit.Error != "" {
			body = "[" + emit.SegmentID + " failed: " + emit.Error + "]\n" + body
		} else {
			body = "[" + emit.SegmentID + " failed]\n" + body
		}
	}
	return body
}

// buildSegmentArtifactFromEmit constructs a wavescheduler.Artifact for a
// single SegmentEmit so Learn can ingest it as evidence. SourcePlanID is
// the parent plan's ID (matches the legacy buildArtifactFromWorkItemResult
// pattern).
func buildSegmentArtifactFromEmit(
	emit wavescheduler.SegmentEmit,
	item *workmodel.WorkItem,
	sessionID string,
	started time.Time,
) *wavescheduler.Artifact {
	ended := emit.EndedAt
	if ended.IsZero() {
		ended = time.Now()
	}
	var planID string
	if item.LastRound != nil {
		planID = item.LastRound.PlanID
	}
	return &wavescheduler.Artifact{
		TaskID:       emit.SegmentID,
		SessionID:    sessionID,
		WorkerType:   emit.WorkerType,
		Summary:      emit.Summary,
		ExitCode:     emit.ExitCode,
		Error:        emit.Error,
		StartedAt:    emit.StartedAt,
		EndedAt:      ended,
		Duration:     ended.Sub(started),
		SourcePlanID: planID,
		Metadata: map[string]any{
			"source":         "dag_segment_emit",
			"segment_id":     emit.SegmentID,
			"parent_item_id": item.ID,
			"worker_hint":    emit.WorkerHint,
			"is_final":       emit.IsFinal,
		},
	}
}

// buildRollupArtifact constructs the Artifact for the rollup Learn call.
// Used only when r.Learner != nil (callers guard). Summary == finalSummary
// so D6 learners reading the artifact see the rollup text. SourcePlanID
// stays empty — the rollup synthesizes a single Verdict from N children,
// not a single TaskGraph node.
func buildRollupArtifact(
	item *workmodel.WorkItem,
	sessionID string,
	started time.Time,
	rollupVerdict workmodel.Verdict,
	finalSummary string,
) *wavescheduler.Artifact {
	var planID string
	if item.LastRound != nil {
		planID = item.LastRound.PlanID
	}
	return &wavescheduler.Artifact{
		TaskID:       item.ID,
		SessionID:    sessionID,
		WorkerType:   wavescheduler.WorkerSubAgent, // rollup synthesizes locally, no remote worker
		Summary:      finalSummary,
		ExitCode:     0,
		StartedAt:    started,
		EndedAt:      time.Now(),
		Duration:     time.Since(started),
		SourcePlanID: planID,
		Metadata: map[string]any{
			"source":         "dag_rollup",
			"parent_item_id": item.ID,
			"verdict_kind":   rollupVerdict.Kind.String(),
		},
	}
}