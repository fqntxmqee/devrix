package sessionorchestrator

import (
	"context"
	"fmt"
	"strings"

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
		var summaries []string

		awaiter := &workmodel.ResolveAwaiter{Manager: o.taskManager}

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
				summaries = append(summaries, msg)
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
				if msg := awaiter.AwaitRunningChildren(ctx, sessionID); msg != "" {
					summaries = append(summaries, msg)
				}
				continue
			}

			round, err := o.itemPipeline.Run(ctx, sessionID, focus, userID)
			if err != nil {
				emitError(ctx, o.sink, out, sessionID, "item_pipeline", err)
				return
			}

			summaries = append(summaries, fmt.Sprintf("[%s] %s → %s (spawn=%s)",
				focus.Kind, focus.Title, round.VerdictKind, round.SpawnPolicy))

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
				if err := o.itemPipeline.RunParallelExplore(ctx, sessionID, focus, round); err != nil {
					emitError(ctx, o.sink, out, sessionID, "parallel_explore", err)
					return
				}
				summaries = append(summaries, round.SpawnRationale)
			case workmodel.SpawnEscalateHuman:
				summaries = append(summaries, "escalated to human review: "+round.SpawnRationale)
			case workmodel.SpawnNone:
				// focus may be terminal; loop picks next item or exits
			}

			if !o.taskManager.Tree().HasOpenWork(sessionID) {
				break
			}
		}

		summary := strings.Join(summaries, "\n")
		if summary == "" {
			summary = "session turn loop: no work items processed"
		}
		emit(ctx, o.sink, out, &contracts.EngineEvent{
			Type: "text", Content: summary, SessionID: sessionID,
		})
		emit(ctx, o.sink, out, &contracts.EngineEvent{
			Type: "complete", Content: summary, SessionID: sessionID,
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
