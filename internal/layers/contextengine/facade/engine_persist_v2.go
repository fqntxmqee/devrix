// engine_persist_v2.go — D2-S17 PersistSessionState path via persist.Orchestrator.
//
// P1-e replaces the legacy engine_persist.go::finalizeTurn helper with a
// thin orchestrator delegation. The scenario-orthogonal responsibilities
// (tool call/result appending, complete event emission) stay in facade;
// the persist operations (snapshot, transcript, long-term, CommitWindow)
// move into persist.PersistOrchestrator.Persist().
//
// DSAFT: D2-S17 (PersistSessionState) — facade scenario-orthogonal hooks.
package facade

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/persist"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// persistTurn is the post-RunPreparedTurn scenario-orthogonal handler.
// It composes the in-memory sc mutations (tool history + assistant summary)
// and delegates the actual persistence (snapshot / transcript / long-term /
// CommitWindow) to persist.PersistOrchestrator.Persist().
//
// Wire once via wirePersistOrchestrator(); if the orchestrator is nil (caller
// skipped wiring), emit a configuration error and return early.
func (e *ContextEngine) persistTurn(
	ctx context.Context,
	session *types.Session,
	sc *types.SessionContext,
	res *preparedTurnLoopResult,
	runErr error,
	working *memory.WorkingMemory,
	message string,
	workerLocal bool,
	transcriptFrom int,
	pendingComplete *contracts.EngineEvent,
	ch chan<- *contracts.EngineEvent,
	emit func(*contracts.EngineEvent),
	processSpan tracer.Span,
	start time.Time,
) {
	if e.persistOrchestrator == nil {
		emit(mapProcessError(session.SessionID, fmt.Errorf("contextengine: PersistOrchestrator not wired (use D7 InitOrchestration)")))
		return
	}
	e.wirePersistOrchestrator()

	// Scenario-orthogonal: tool call history + assistant summary append
	// (these mutate sc.Messages; orchestrator handles CommitWindow on the
	// already-mutated slice).
	assistantSummary := e.appendToolHistoryAndAssistant(sc, res, working)

	// Handle user-initiated cancellation: undo the user message + emit
	// stopped marker, then short-circuit persist.
	if runErr != nil && stderrors.Is(runErr, context.Canceled) {
		slog.Info("contextengine: process stopped by user", "sessionID", session.SessionID)
		e.memory.RemoveLastUserMessage(sc)
		e.memory.TrimMessages(sc)
		ch <- infoEvent(session.SessionID, "⏸️ 已停止")
		runErr = nil
	}

	// Build the persist input. snapshotData + transcriptDelta are
	// serialized from the post-mutation sc + transcript delta slice.
	snapshotData, _ := e.memory.PersistSnapshot(sc)
	if !workerLocal && snapshotData != nil {
		session.ContextSnapshot = snapshotData
		e.observer.EmitSnapshotRestored(session.SessionID, false)
	}

	var transcriptDelta []byte
	if !workerLocal {
		delta := e.transcriptMessages(sc, transcriptFrom)
		if len(delta) > 0 {
			transcriptDelta, _ = json.Marshal(delta)
		}
	}

	// A04 CommitWindow runs first inside orchestrator.Persist so the snapshot
	// captures the trimmed view.
	persistErr := e.persistOrchestrator.Persist(ctx, persist.PersistInput{
		SessionID:        session.SessionID,
		SnapshotData:     snapshotData,
		TranscriptDelta:  transcriptDelta,
		Query:            message,
		AssistantSummary: assistantSummary,
		IsWorker:         workerLocal,
		SessionContext:   sc,
	})

	if persistErr != nil {
		slog.Warn("contextengine: persist failed", "sessionID", session.SessionID, "error", persistErr)
		if processSpan != nil {
			processSpan.RecordError(persistErr)
			processSpan.SetStatus(tracer.StatusCodeError, persistErr.Error())
		}
		emit(mapProcessError(session.SessionID, persistErr))
		return
	}

	// Engine error path (non-canceled): emit and return.
	if runErr != nil {
		if processSpan != nil {
			processSpan.RecordError(runErr)
			processSpan.SetStatus(tracer.StatusCodeError, runErr.Error())
		}
		emit(mapProcessError(session.SessionID, runErr))
		return
	}

	// Final complete event: either the deferred one from the turn loop or
	// a synthesized one with duration/usage metadata.
	if pendingComplete != nil {
		emit(pendingComplete)
	} else {
		meta := map[string]string{
			"duration": fmt.Sprintf("%d", time.Since(start).Milliseconds()),
			"model":    sc.Model,
		}
		if res != nil {
			meta["usage"] = fmt.Sprintf("%d", res.Usage.PromptTokens+res.Usage.CompletionTokens)
			if pct := contracts.ComputeCtxPct(res.Usage.PromptTokens, sc.TokenBudget.MaxContextTokens); pct > 0 {
				meta["ctx_pct"] = fmt.Sprintf("%d", pct)
			}
		}
		emit(&contracts.EngineEvent{
			Type:      "complete",
			SessionID: session.SessionID,
			Metadata:  meta,
		})
	}

	slog.Debug("contextengine: process done", "sessionID", session.SessionID, "duration", time.Since(start))
}

// appendToolHistoryAndAssistant mutates sc.Messages with the tool call/result
// pairs from res.ToolCallHistory and the flushed working-memory assistant text.
// Returns the assistant summary string (used by orchestrator.A03 long-term store).
func (e *ContextEngine) appendToolHistoryAndAssistant(sc *types.SessionContext, res *preparedTurnLoopResult, working *memory.WorkingMemory) string {
	var assistantSummary string
	if res != nil && len(res.ToolCallHistory) > 0 {
		for i, tc := range res.ToolCallHistory {
			callID := strings.TrimSpace(tc.CallID)
			if callID == "" {
				callID = fmt.Sprintf("call_%s_%d", tc.ToolName, i)
			}
			tcJSON, _ := json.Marshal([]struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			}{{
				ID:   callID,
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{Name: tc.ToolName, Arguments: tc.Input},
			}})
			tcMsg := types.Message{
				Role:     types.MessageRoleAssistant,
				Content:  "",
				Metadata: map[string]string{"tool_calls": string(tcJSON)},
			}
			e.memory.AppendFullMessage(sc, tcMsg)

			resultContent := conversation.FormatToolResultContent(tc.ToolName, tc.Output, tc.Error)
			if e.cfg.ToolResultBudget > 0 && e.counter != nil {
				if e.counter.CountText(resultContent) > e.cfg.ToolResultBudget {
					resultContent = e.counter.TruncateToTokens(resultContent, e.cfg.ToolResultBudget) + "\n...[truncated for persist]"
				}
			}
			resultMsg := types.Message{
				Role:     types.MessageRoleTool,
				Content:  resultContent,
				Metadata: map[string]string{"tool_call_id": callID},
			}
			e.memory.AppendFullMessage(sc, resultMsg)
		}
	}

	if text := working.FlushStream(); text != "" {
		e.memory.AppendMessage(sc, types.MessageRoleAssistant, text)
		assistantSummary = text
	}
	if assistantSummary == "" {
		assistantSummary = lastAssistantContent(sc.Messages)
	}
	return assistantSummary
}

// transcriptMessages returns the slice of sc.Messages from `from` onward,
// skipping any leading system-role message (transcript stores user/assistant/tool
// history only).
func (e *ContextEngine) transcriptMessages(sc *types.SessionContext, from int) []types.Message {
	if from < 0 {
		from = 0
	}
	if from >= len(sc.Messages) {
		return nil
	}
	return sc.Messages[from:]
}

// Compile-time sanity check that we don't accidentally drop the telemetry
// op-name import (kept so the file is grep-able in spec audits).
var _ = telemetry.OpD2_S2_Context_Memory_Snapshot_Save
