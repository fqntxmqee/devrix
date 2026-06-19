package contextengine

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// finalizeTurn handles post-query-loop processing: persist tool history,
// transcript, long-term store, snapshot, and complete event emission.
func (e *ContextEngine) finalizeTurn(
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
	var assistantSummary string
	if runErr == nil {
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
		e.appendMainTranscript(session.SessionID, sc.Messages[transcriptFrom:], workerLocal)
		e.memory.TrimMessages(sc)
		e.commitActiveWindow(ctx, sc, session.SessionID)
		if assistantSummary == "" {
			assistantSummary = lastAssistantContent(sc.Messages)
		}
	storeCtx, storeSpan := e.startSpan(ctx, telemetry.OpD2_S2_Context_Longterm_Store, tracer.SpanKindInternal)
		storeErr := e.memory.AutoStoreLongTerm(storeCtx, sc, message, assistantSummary)
		if storeSpan != nil {
			if storeErr != nil {
				storeSpan.RecordError(storeErr)
			}
			storeSpan.End()
		}
		if storeErr != nil {
			slog.Warn("contextengine: longterm auto_store failed", "error", storeErr)
		}
	} else if stderrors.Is(runErr, context.Canceled) {
		slog.Info("contextengine: process stopped by user", "sessionID", session.SessionID)
		e.memory.RemoveLastUserMessage(sc)
		e.memory.TrimMessages(sc)
		ch <- infoEvent(session.SessionID, "⏸️ 已停止")
		runErr = nil
	} else {
		if processSpan != nil {
			processSpan.RecordError(runErr)
			processSpan.SetStatus(tracer.StatusCodeError, runErr.Error())
		}
		emit(mapProcessError(session.SessionID, runErr))
	}

	_, saveSpan := e.startSpan(ctx, telemetry.OpD2_S2_Context_Memory_Snapshot_Save, tracer.SpanKindInternal,
		tracer.Attribute{Key: "snapshot.message_count", Value: fmt.Sprintf("%d", len(sc.Messages))},
	)
	if !workerLocal {
		if data, persistErr := e.memory.PersistSnapshot(sc); persistErr == nil {
			session.ContextSnapshot = data
			e.observer.EmitSnapshotRestored(session.SessionID, false)
			if saveSpan != nil {
				saveSpan.SetAttributes(tracer.Attribute{Key: "snapshot.size_bytes", Value: fmt.Sprintf("%d", len(data))})
				saveSpan.End()
			}
		} else {
			slog.Warn("contextengine: persist snapshot failed", "error", persistErr)
			if saveSpan != nil {
				saveSpan.RecordError(persistErr)
				saveSpan.End()
			}
		}
	} else if saveSpan != nil {
		saveSpan.End()
	}

	if pendingComplete != nil {
		emit(pendingComplete)
	} else if runErr == nil {
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

func (e *ContextEngine) appendMainTranscript(sessionID string, delta []types.Message, workerLocal bool) {
	if e.mainTranscript == nil || workerLocal || len(delta) == 0 {
		return
	}
	if err := e.mainTranscript.AppendBatch(sessionID, delta); err != nil {
		slog.Warn("contextengine: main transcript append failed", "sessionID", sessionID, "error", err)
	}
}

func (e *ContextEngine) commitActiveWindow(ctx context.Context, sc *types.SessionContext, sessionID string) {
	active := conversation.RepairToolMessageChain(conversation.MessagesAfterCompactBoundary(sc.Messages))
	max := e.cfg.Compression.MaxMessages
	if max <= 0 {
		max = 50
	}
	overMessages := len(active) > max
	overTokens := e.shouldCompress(active, sc.TokenBudget)
	if !overMessages && !overTokens {
		return
	}
	compressed, report, err := e.compressionPipeline(sessionID).Run(ctx, active, "", sc.TokenBudget)
	if err != nil || len(report.StepsApplied) == 0 {
		return
	}
	committed := conversation.RepairToolMessageChain(stripSystemMessage(compressed))
	e.memory.SetActiveMessages(sc, committed)
	e.memory.TrimMessages(sc)
}
