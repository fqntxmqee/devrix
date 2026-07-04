package sessionorchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/devrix/devrix/internal/layers/orchestration/escape"
	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// RunSessionTurnLoop drives GetPipelineFocus → RunItemPipeline → spawn/await
// until the session work set is closed or anomaly/escape/spawn signals say
// stop (Phase C, G5). RH-D7-05: no fixed iteration budget — termination is
// driven by HasOpenWork, SpawnEscalateHuman, EscapeEngine, and verify
// anomaly detectors (empty_conclusion / deliverable stagnation).
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
		// RH-D7-01 (DM-20260630-013): emitFn is now passed per-invocation
		// via ItemPipelineRunOpts instead of being installed on the shared
		// runner struct. Concurrent sessions each carry their own closure,
		// eliminating cross-session event leakage.

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
		var lastRound *workmodel.WorkItemPipelineRound

		turnNo := 0
		if o.turnState != nil {
			turnNo = o.turnState.TurnNo(sessionID)
		}
		loopTick := 0

		for {
			loopTick++
			if ctx.Err() != nil {
				emit(ctx, o.sink, out, &contracts.EngineEvent{
					Type: "error", Content: ctx.Err().Error(), SessionID: sessionID,
				})
				return
			}

			var escDecision escape.EscapeDecision
			if o.escapeEngine != nil {
				loopCtx := buildEscapeLoopContextFromRound(sessionID, lastRound)
				escDecision = o.escapeEngine.Evaluate(ctx, loopCtx)
				if term, augErr := o.processEscapeDecision(escDecision, nil); term {
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
				if _, triggered := workmodel.MaybeDecomposeParentRollup(sessionID, o.taskManager); triggered {
					continue
				}
				if _, triggered := workmodel.MaybeRootRollupFallback(sessionID, o.taskManager); triggered {
					continue
				}
				// RH-MUPS-03 (DM-20260701-001): before exiting silently,
				// surface any rollup parent that exhausted retries as a
				// human_review event. Without this, the session loop would
				// break with the rollup parent still InProgress and the user
				// sees no signal at all.
				if item, reason := findExhaustedRollupParent(sessionID, o.taskManager); item != nil {
					msg := fmt.Sprintf("Rollup verification failed after retry limit: %s — human review required for work item %s", reason, item.ID)
					emit(ctx, o.sink, out, &contracts.EngineEvent{
						Type:      "human_review",
						Content:   item.Directive,
						SessionID: sessionID,
					})
					emit(ctx, o.sink, out, &contracts.EngineEvent{
						Type:      "error",
						Content:   msg,
						SessionID: sessionID,
						Metadata:  map[string]string{"work_item_id": item.ID, "reason": "rollup_retries_exhausted"},
					})
					emit(ctx, o.sink, out, &contracts.EngineEvent{
						Type:      "text",
						Content:   msg,
						SessionID: sessionID,
					})
					emit(ctx, o.sink, out, &contracts.EngineEvent{
						Type:      "complete",
						Content:   msg,
						SessionID: sessionID,
					})
					return
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
				// RH-D7-04 (DM-20260630-013): AwaitRunningChildren returns a
				// summary string of the just-completed children. Previously
				// dropped via `_`, which made the await-into-continue
				// path invisible to both the user and the trace. Surface
				// the summary via slog (and the resolve event) so the loop
				// restart is observable in Jaeger.
				if summary := awaiter.AwaitRunningChildren(ctx, sessionID); summary != "" {
					slog.Info("session_turn_loop: running children resolved before next focus",
						"session_id", sessionID, "work_item_id", focus.ID, "summary_len", len(summary))
				}
				continue
			}

			round, err := o.itemPipeline.Run(ctx, sessionID, focus, userID, ItemPipelineRunOpts{
				Emit:     emitFn,
				TurnNo:   turnNo,
				LoopTick: loopTick,
			})
			if err != nil {
				emitError(ctx, o.sink, out, sessionID, "item_pipeline", err)
				return
			}
			lastRound = round

			// WorkItemExecutor (inside ItemPipelineRunner.Run) drives the
			// per-WorkItem ReAct loop; round.ArtifactSummary carries the
			// LLM's final answer. Emit it as a text event so the gateway
			// (feishu reply card) sees user-visible content.
			if round.ArtifactSummary != "" {
				content := round.ArtifactSummary
				if workmodel.ShouldSuppressFindingsArtifactStream(round) {
					if formatted := workmodel.SalvageDeliverableFromRound(round); formatted != "" {
						content = formatted
					} else {
						content = ""
					}
				}
				if content != "" {
					lastArtifactSummary = content
					emit(ctx, o.sink, out, &contracts.EngineEvent{
						Type:      "text",
						Content:   content,
						SessionID: sessionID,
					})
				}
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

			exit := evaluateSessionLoopExitAfterRound(ctx, sessionID, o.taskManager, round, escDecision)
			switch exit.Kind {
			case SessionLoopExitEscalate:
				userMsg := buildUserFacingEscalationSummary(o.taskManager, sessionID)
				emit(ctx, o.sink, out, &contracts.EngineEvent{
					Type: "human_review", Content: focus.Directive, SessionID: sessionID,
				})
				emit(ctx, o.sink, out, &contracts.EngineEvent{
					Type: "text", Content: userMsg, SessionID: sessionID,
				})
				emit(ctx, o.sink, out, &contracts.EngineEvent{
					Type: "complete", Content: userMsg, SessionID: sessionID,
				})
				return
			case SessionLoopExitAnomaly:
				if _, triggered := workmodel.MaybeDecomposeParentRollup(sessionID, o.taskManager); triggered {
					continue
				}
				goto sessionLoopDone
			}

			if !o.taskManager.Tree().HasOpenWork(sessionID) {
				if _, triggered := workmodel.MaybeDecomposeParentRollup(sessionID, o.taskManager); triggered {
					continue
				}
				if _, triggered := workmodel.MaybeRootRollupFallback(sessionID, o.taskManager); triggered {
					continue
				}
				goto sessionLoopDone
			}

			if sessionNoForwardProgress(sessionID, o.taskManager) {
				if _, triggered := workmodel.MaybeDecomposeParentRollup(sessionID, o.taskManager); triggered {
					continue
				}
				goto sessionLoopDone
			}
		}
	sessionLoopDone:

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

// findExhaustedRollupParent returns the first rollup WorkItem in the session
// whose LastRound.RollupRetries reached DefaultMaxRollupRetries and whose
// status is still non-terminal. The session loop calls this immediately
// before exiting with focus==nil so unresolved rollups surface as a
// human_review event instead of disappearing silently (RH-MUPS-03).
//
// Returns (item, reason) — reason is the prior round's verdict reason when
// available, else a default "rollup retries exhausted". The session loop
// surfaces reason verbatim in the emitted error event so users can see why
// the rollup could not converge.
func findExhaustedRollupParent(sessionID string, tm *workmodel.TaskManager) (*workmodel.WorkItem, string) {
	if tm == nil {
		return nil, ""
	}
	items := tm.Tree().List(sessionID)
	for _, item := range items {
		if item == nil || !item.NeedsRollup {
			continue
		}
		if workmodel.IsTerminalStatus(item.Status) {
			continue
		}
		if item.LastRound == nil {
			continue
		}
		if item.LastRound.RollupRetries < workmodel.DefaultMaxRollupRetries {
			continue
		}
		reason := "rollup retries exhausted"
		if r := item.LastRound.VerdictKind; r != 0 {
			reason = fmt.Sprintf("rollup retries exhausted (last verdict=%s)", r)
		}
		return item, reason
	}
	return nil, ""
}
