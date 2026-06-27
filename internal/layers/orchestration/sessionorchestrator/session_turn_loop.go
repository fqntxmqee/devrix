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

		awaiter := &workmodel.ResolveAwaiter{Manager: o.taskManager}

		// Seed: ensure a session root WorkItem exists before the focus loop
		// (Phase C ingress gap fix, 2026-06-26). Without this, a fresh user
		// message finds an empty work tree, GetPipelineFocus returns nil, and
		// the loop emits a 50-byte stub. EnsureGoal follows design D5
		// ("单 session 单根" / "EnsureGoal 现有语义"); a locked (terminal) goal
		// gets a fresh root while the original children stay attached.
		if _, seedErr := o.taskManager.EnsureGoal(sessionID, req.Message); seedErr != nil {
			emitError(ctx, o.sink, out, sessionID, "ensure_goal", seedErr)
			return
		}

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
				break
			}
		}

		// 2026-06-26 hotfix: previously the loop emitted a `text` event
		// carrying the D7 internal pipeline summary ([Goal] title →
		// VerdictKind (spawn=...)) before the `complete`. The feishu
		// reply card treats both as user-facing content, so the user saw
		// the LLM's actual answer plus a D7 metadata line appended. The
		// LLM streaming path already delivers the real answer; the
		// pipeline summary is internal and now stays internal.
		//
		// The `complete` event is kept (gateway needs it to finalize the
		// session and LP-1 Auto-Close to run), but Content is empty so
		// feishu.finalizeStructuredSession does not render a 任务总结 card
		// from D7 metadata — the LLM's own final paragraph (already on
		// the reply card via streaming) is the conclusion the user sees.
		emit(ctx, o.sink, out, &contracts.EngineEvent{
			Type: "complete", SessionID: sessionID,
		})
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
