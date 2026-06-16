package contextengine

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// loadOrInitSession loads or initializes the session context from snapshot.
// Returns (sc, true) on success, or (nil, false) on unrecoverable error.
func (e *ContextEngine) loadOrInitSession(ctx context.Context, session *types.Session, emit func(*contracts.EngineEvent)) (*types.SessionContext, bool) {
	_, loadSpan := e.startSpan(ctx, telemetry.OpD2_S2_Context_Snapshot_Load, tracer.SpanKindInternal)
	hadSnapshot := session.ContextSnapshot != nil
	sc, err := e.memory.LoadOrInit(session, "")
	if err != nil {
		emit(infoEvent(session.SessionID, "快照已重置，开始新上下文"))
		session.ContextSnapshot = nil
		prompt.ClearDynamicSectionCache(session.SessionID)
		prompt.ClearAgentsCache()
		sc, err = e.memory.LoadOrInit(session, "")
		if err != nil {
			if loadSpan != nil {
				loadSpan.RecordError(err)
				loadSpan.End()
			}
			emit(errorEvent(session.SessionID, errors.NewSnapshotCorruptError(err), false))
			return nil, false
		}
		e.observer.EmitSnapshotRestored(session.SessionID, false)
	}
	if loadSpan != nil {
		loadSpan.SetAttributes(
			tracer.Attribute{Key: "snapshot.message_count", Value: fmt.Sprintf("%d", len(sc.Messages))},
			tracer.Attribute{Key: "snapshot.restored", Value: fmt.Sprintf("%t", hadSnapshot)},
		)
		loadSpan.End()
	}
	return sc, true
}

// recallLongTermMemory performs long-term memory recall for the current message.
// Returns (entries, true) on success, or (nil, false) on error.
func (e *ContextEngine) recallLongTermMemory(
	ctx context.Context,
	sessionID string,
	message string,
	workerLocal bool,
	processSpan tracer.Span,
	emit func(*contracts.EngineEvent),
) ([]memory.MemoryEntry, bool) {
	var memoryEntries []memory.MemoryEntry
	recallCtx, recallSpan := e.startSpan(ctx, telemetry.OpD2_S2_Context_Longterm_Recall, tracer.SpanKindInternal,
		tracer.Attribute{Key: "longterm.enabled", Value: fmt.Sprintf("%t", e.cfg.LongTerm.Enabled)},
		tracer.Attribute{Key: "longterm.recall_topics", Value: strings.Join(e.cfg.LongTerm.Topics, ",")},
	)
	var recallErr error
	if !workerLocal {
		memoryEntries, recallErr = e.memory.RecallLongTermEntries(recallCtx, message)
	}
	if recallSpan != nil {
		if recallErr != nil {
			recallSpan.RecordError(recallErr)
		}
		recallSpan.End()
	}
	if recallErr != nil {
		if processSpan != nil {
			processSpan.RecordError(recallErr)
		}
		var se *errors.SentinelError
		if stderrors.As(recallErr, &se) {
			emit(errorEvent(sessionID, se, false))
		} else {
			emit(errorEvent(sessionID, errors.NewLongTermDBError(recallErr), false))
		}
		return nil, false
	}
	return memoryEntries, true
}

// prepareMessages extracts and optionally compresses the working message list
// from the session context. Returns (msgs, true) on success, or (nil, false) on error.
func (e *ContextEngine) prepareMessages(
	ctx context.Context,
	sc *types.SessionContext,
	sessionID string,
	workerLocal bool,
	emit func(*contracts.EngineEvent),
) ([]types.Message, bool) {
	msgs := conversation.RepairToolMessageChain(conversation.MessagesAfterCompactBoundary(sc.Messages))
	compSystemPrompt := sc.SystemPrompt
	skipEntryCompress := e.cfg.QueryLoop.Enabled && e.cfg.QueryLoop.CompressPerTurn
	if !skipEntryCompress && e.shouldCompress(msgs, sc.TokenBudget) {
		compCtx, compSpan := e.startSpan(ctx, telemetry.OpD2_S2_Context_Compression_Run, tracer.SpanKindInternal,
			tracer.Attribute{Key: "context.tokens_before", Value: fmt.Sprintf("%d", len(msgs))},
		)
		compressed, report, compErr := e.compressionPipeline(sessionID).Run(compCtx, msgs, compSystemPrompt, sc.TokenBudget)
		if compErr != nil {
			if compSpan != nil {
				telemetry.RecordSpanError(compSpan, compErr)
				compSpan.End()
			}
			if se, ok := compErr.(*errors.SentinelError); ok {
				emit(errorEvent(sessionID, se, false))
			}
			return nil, false
		}
		if compSpan != nil {
			ratio := report.Ratio()
			compSpan.SetAttributes(
				tracer.Attribute{Key: "context.tokens_after", Value: fmt.Sprintf("%d", report.CompressedTokens)},
				tracer.Attribute{Key: "context.messages_before", Value: fmt.Sprintf("%d", len(msgs))},
				tracer.Attribute{Key: "context.messages_after", Value: fmt.Sprintf("%d", len(compressed))},
				tracer.Attribute{Key: "context.steps_applied", Value: strings.Join(report.StepsApplied, ",")},
				tracer.Attribute{Key: "compression.trigger_reason", Value: "token_budget_exceeded"},
				tracer.Attribute{Key: "compression.ratio", Value: fmt.Sprintf("%.4f", ratio)},
			)
			compSpan.End()
		}
		if e.compressionRatio != nil {
			e.compressionRatio.Observe(report.Ratio())
		}
		e.observer.EmitContextCompressed(report)
		e.memory.SetCompressedView(sc, compressed)
		emit(infoEvent(sessionID, fmt.Sprintf("上下文已压缩 (%d→%d tokens)", report.OriginalTokens, report.CompressedTokens)))
		msgs = stripSystemMessage(compressed)
	}
	return msgs, true
}
