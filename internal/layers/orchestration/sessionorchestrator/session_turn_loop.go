package sessionorchestrator

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
)

const defaultSessionTurnLoopMax = 16

// RunSessionTurnLoop drives GetPipelineFocus → RunItemPipeline → spawn/await
// until the session work set is closed or limits hit (Phase C, G5).
func (o *SessionOrchestrator) RunSessionTurnLoop(
	ctx context.Context,
	req orchtypes.ProcessRequest,
	_ orchtypes.IntentClassification,
) (<-chan *contracts.EngineEvent, error) {
	if o == nil {
		return nil, fmt.Errorf("orchestrator: nil")
	}
	if o.itemPipeline == nil {
		return nil, fmt.Errorf("orchestrator: item pipeline not wired (WithItemPipelineRunner)")
	}
	if o.taskManager == nil {
		return nil, fmt.Errorf("orchestrator: task manager required for session turn loop")
	}

	out := make(chan *contracts.EngineEvent, 16)
	go func() {
		defer close(out)
		sessionID := req.SessionID
		userID := effectiveUserID(ctx, req)

		// DM-20260628-003 (D7-S15): reserve a turn slot for this session.
		// WaitTurn at ProcessMessage entry has already verified no prior
		// turn is in-flight; BeginTurn here claims the slot for THIS
		// goroutine. defer EndTurn releases it when channel closes.
		//
		// nil turnState (legacy path / not wired) → no-op, equivalent to
		// pre-D7-S15 behavior. This preserves backward compat for
		// CommandHandler-direct and tests that don't wire TurnState.
		if o.turnState != nil {
			if err := o.turnState.BeginTurn(sessionID); err != nil {
				// Should not happen because WaitTurn ran at ProcessMessage
				// entry — but defensive: another goroutine may have raced
				// in via CommandHandler etc. Emit a clear error event so
				// the gateway can surface "⏳ 上一条还在处理中".
				emitError(ctx, o.sink, out, sessionID, "begin_turn", err)
				return
			}
			defer o.turnState.EndTurn(sessionID)
		}

		// Emit hook for ItemPipelineRunner / WorkItemExecutor so per-WorkItem
		// tool calls show up on feishu cards. Events are forwarded from this
		// goroutine's `out` channel in arrival order so the gateway renders
		// them live on feishu cards.
		emitFn := func(ev *contracts.EngineEvent) {
			if ev == nil {
				return
			}
			if ev.SessionID == "" {
				ev.SessionID = sessionID
			}
			emit(ctx, o.sink, out, ev)
		}
		o.itemPipeline.Emit = emitFn

		awaiter := &workmodel.ResolveAwaiter{Manager: o.taskManager}

		// Seed a session root WorkItem ONLY when none exists yet (Phase C
		// ingress gap fix, 2026-06-26). When called via ProcessMessage
		// the orchestrator has already called EnsureGoal (and possibly
		// enriched the directive with <prior-output-summary> for turn
		// N+1, see DM-20260628-003 / D7-S15). Overwriting an existing
		// goal's directive with the raw req.Message here would clobber
		// that enrichment, so we only seed when the work tree is empty
		// for this session — i.e. when callers bypass ProcessMessage
		// and call RunSessionTurnLoop directly (e.g. unit tests).
		if focus, _ := o.taskManager.Tree().GetPipelineFocus(sessionID); focus == nil {
			if _, seedErr := o.taskManager.EnsureGoal(sessionID, req.Message); seedErr != nil {
				emitError(ctx, o.sink, out, sessionID, "ensure_goal", seedErr)
				return
			}
		}

		// DM-20260628-003 (D7-S15): track the most recent per-round
		// ArtifactSummary so the terminal `complete` event carries the
		// CURRENT turn's LLM response, not the locked turn-1 root's stale
		// summary. With PriorContextRounds > 0, turn N+1's EnsureGoal
		// creates a NEW root goal (the previous root is Locked after turn
		// 1 completes). ExtractSessionDeliverable returns the FIRST root
		// in tree iteration order, which is turn 1's locked root → the
		// `complete` event repeated FOO_REPLY_1 for every turn, masking
		// turn 2's actual BAR_REPLY_2. Fall back to ExtractSessionDeliverable
		// when the loop never ran a pipeline round (direct caller / no
		// focus / skip path).
		var lastArtifactSummary string

		for iter := 0; iter < defaultSessionTurnLoopMax; iter++ {
			if ctx.Err() != nil {
				emit(ctx, o.sink, out, &contracts.EngineEvent{
					Type: "error", Content: ctx.Err().Error(), SessionID: sessionID,
				})
				return
			}

			if o.escapeEngine != nil {
				loopCtx := o.buildEscapeLoopContext(sessionID, 0, "")
				decision := o.escapeEngine.Evaluate(ctx, loopCtx)
				if term, augErr := o.processEscapeDecision(decision, nil); term {
					msg := "escape_force_exit"
					if augErr != nil {
						msg = augErr.Error()
					}
					emit(ctx, o.sink, out, &contracts.EngineEvent{
						Type: "error", Content: msg, SessionID: sessionID,
					})
					return
				}
			}

			focus, err := o.taskManager.Tree().GetPipelineFocus(sessionID)
			if err != nil {
				emitError(ctx, o.sink, out, sessionID, "get_focus", err)
				return
			}
			if focus == nil {
				if _, triggered := workmodel.MaybeRootRollupFallback(sessionID, o.taskManager); triggered {
					continue
				}
				break
			}

			if workmodel.IsHumanReviewItem(focus) && focus.Status == workmodel.TaskStatusPending {
				msg := fmt.Sprintf("Human review required for work item %s — use /task review approve %s",
					focus.ID, focus.ID)
				emit(ctx, o.sink, out, &contracts.EngineEvent{
					Type:      "human_review",
					Content:   focus.Directive,
					SessionID: sessionID,
				})
				emit(ctx, o.sink, out, &contracts.EngineEvent{
					Type: "text", Content: msg, SessionID: sessionID,
				})
				emit(ctx, o.sink, out, &contracts.EngineEvent{
					Type: "complete", Content: msg, SessionID: sessionID,
				})
				return
			}

			if stats := runningChildCount(o.taskManager, sessionID, focus.ID); stats > 0 {
				_ = awaiter.AwaitRunningChildren(ctx, sessionID)
				continue
			}

			round, err := o.itemPipeline.Run(ctx, sessionID, focus, userID)
			if err != nil {
				emitError(ctx, o.sink, out, sessionID, "item_pipeline", err)
				return
			}

			// WorkItemExecutor (inside ItemPipelineRunner.Run) drives the
			// per-WorkItem ReAct loop; round.ArtifactSummary carries the
			// LLM's final answer. Emit it as a text event so the gateway
			// (feishu reply card) sees user-visible content.
			if round.ArtifactSummary != "" {
				lastArtifactSummary = round.ArtifactSummary
				emit(ctx, o.sink, out, &contracts.EngineEvent{
					Type:      "text",
					Content:   round.ArtifactSummary,
					SessionID: sessionID,
				})
			}

			emit(ctx, o.sink, out, &contracts.EngineEvent{
				Type:      "pipeline_round",
				Content:   round.SpawnRationale,
				SessionID: sessionID,
			})

			if err := workmodel.ApplySpawnPolicy(sessionID, focus, round, o.taskManager); err != nil {
				emitError(ctx, o.sink, out, sessionID, "spawn_policy", err)
				return
			}

			if focus.ParentID != "" {
				workmodel.ReevaluateParentAfterChild(sessionID, focus.ID, o.taskManager)
			}

			switch round.SpawnPolicy {
			case workmodel.SpawnParallelExplore:
				// SubWorktree span (DM-20260626-009 follow-up): wraps the
				// RunParallelExplore call so a parallel-explore round shows
				// up as a child span under the surrounding MUPS Pipeline
				// root. RunParallelExplore is currently a no-op stub; the
				// span is wired here so the trace shape is in place when a
				// successor that drives ephemeral probes via
				// WorkItemExecutor lands.
				endSpan := hardening.EmitSubWorktreeRun(ctx, sessionID, focus.ID, "", "spawn_parallel_explore")
				if err := o.itemPipeline.RunParallelExplore(ctx, sessionID, focus, round); err != nil {
					endSpan(err)
					emitError(ctx, o.sink, out, sessionID, "parallel_explore", err)
					return
				}
				endSpan(nil)
			case workmodel.SpawnEscalateHuman, workmodel.SpawnNone:
				// focus may be terminal; loop picks next item or exits
			}

			if !o.taskManager.Tree().HasOpenWork(sessionID) {
				if _, triggered := workmodel.MaybeRootRollupFallback(sessionID, o.taskManager); triggered {
					continue
				}
				break
			}
		}

		// Prefer rollup deliverable + quality gate (DM-20260630-012).
		emit(ctx, o.sink, out, buildSessionCompleteEvent(ctx, sessionID, o.taskManager, lastArtifactSummary))
	}()

	return out, nil
}

func runningChildCount(tm *workmodel.TaskManager, sessionID, parentID string) int {
	if tm == nil {
		return 0
	}
	n := 0
	for _, c := range tm.Tree().ListChildren(sessionID, parentID) {
		if c == nil || c.Kind == workmodel.WorkKindChecklist {
			continue
		}
		if c.Status == workmodel.TaskStatusInProgress {
			n++
		}
	}
	return n
}
