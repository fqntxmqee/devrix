package query

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/usercontext"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/types"
)

// Loop runs the Claude Code-aligned query loop (while tool_use continue).
//
// DM-020 D2 thin closure: orchestration callbacks (LoopHooks), per-turn
// attachment collection (*attachments.Registry), and Hub-Spoke drain
// (SessionQueue) have all moved out of D2. D7 owns lifecycle hooks
// (D7-S2-A06 RunTurnLoop), per-turn context injection (D7 Prepare), and
// flow event aggregation (D7-S4). The remaining fields are pure D2
// execution primitives: LLM call, tool execution, permission gate,
// per-turn compression, user-context prepend, and observability.
type Loop struct {
	LLM             LLMCaller
	Tools           ToolExecutor
	Permission      PermissionChecker
	Compress        CompressFunc
	CompressFactory func(sessionID string) CompressFunc // lazy-init when sessionID is known
	UserContext     *usercontext.Provider
	PrependUC       func(msgs []types.Message, uc map[string]string) []types.Message
	WrapToolContext func(ctx context.Context, sc *types.SessionContext) context.Context
	// WrapToolStreamContext wraps the tool execution context with a stream emitter
	// so agent tools (call_claude-code etc.) can stream mid-execution events.
	WrapToolStreamContext func(ctx context.Context, emit EmitFunc, sessionID, toolName string) context.Context
	StreamingTools        bool
	// Observability bridge for tracing. When nil, tracing is no-op.
	Observability *observability.Bridge

	// FallbackLLM (TD-QL-03) is the secondary LLMCaller used when the
	// primary `LLM` returns an overload / 5xx-class error. nil disables
	// the fallback path entirely. The primary caller is responsible for
	// wiring an LLMCaller whose Model points at the fallback provider.
	FallbackLLM LLMCaller
	// FallbackOnErr classifies errors that should trigger a switch to
	// FallbackLLM. nil means "never fallback" (the safer default).
	// Production code wires `query.IsOverloadOr5xx` (see recovery.go).
	FallbackOnErr func(err error) bool
}

// Run executes the loop until no tool calls, max turns, cancel, or hook stop.
func (l *Loop) Run(
	ctx context.Context,
	sc *types.SessionContext,
	params Params,
	emit EmitFunc,
) (*Result, error) {
	if l.LLM == nil {
		return nil, fmt.Errorf("query loop: LLM is nil")
	}
	prepend := l.PrependUC
	if prepend == nil && l.UserContext != nil {
		prepend = usercontext.PrependForAPI
	}
	messages := append([]types.Message(nil), params.Messages...)
	messages = conversation.StripSystem(messages)

	var (
		loopSpan         tracer.Span
		assistantText    string
		usage            TokenUsage
		allToolRecords   []types.ToolCallRecord
		toolRoundResults []ToolRoundResult
		turn             int
	)
	if sc != nil {
		_, loopSpan = l.startLoopSpan(ctx, telemetry.OpD2_S10_Query_Loop_Run, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session.id", Value: sc.SessionID},
			tracer.Attribute{Key: "max_turns", Value: fmt.Sprintf("%d", params.MaxTurns)})
	}
	if loopSpan != nil {
		defer func() {
			loopSpan.SetAttributes(tracer.Attribute{Key: "result.turn_count", Value: fmt.Sprintf("%d", turn)})
			loopSpan.End()
		}()
	}

	maxTurns := params.MaxTurns

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if maxTurns > 0 && turn >= maxTurns {
			break
		}

		turnCtx, turnSpan := l.startLoopSpan(ctx, telemetry.OpD2_S10_Query_Loop_Turn, tracer.SpanKindInternal,
			tracer.Attribute{Key: "turn.number", Value: fmt.Sprintf("%d", turn)})
		toolCallCount := 0
		endTurn := func() {
			if turnSpan == nil {
				return
			}
			turnSpan.SetAttributes(tracer.Attribute{Key: "turn.tool_count", Value: fmt.Sprintf("%d", toolCallCount)})
			turnSpan.End()
		}

		if l.Compress == nil && l.CompressFactory != nil && sc != nil {
			l.Compress = l.CompressFactory(sc.SessionID)
		}
		if l.Compress != nil {
			if compressed, err := l.Compress(ctx, messages); err != nil {
				return nil, err
			} else {
				messages = compressed
			}
		}

		uc := params.UserContext
		if l.UserContext != nil && sc != nil {
			uc = l.UserContext.Get(ctx, sc)
			if sc.AgentID != "" { uc = usercontext.OmitClaudeMd(uc) }
		}

		messages = conversation.RepairToolMessageChain(messages)

		apiMessages := messages
		if prepend != nil && len(uc) > 0 {
			apiMessages = prepend(messages, uc)
		}

		_, llmSpan := l.startLoopSpan(turnCtx, telemetry.OpD2_S10_Query_Loop_LLM_Call, tracer.SpanKindClient,
			tracer.Attribute{Key: "llm.model", Value: sc.Model})
		llmStart := time.Now()

		chunks, err := runWithContextLengthRecovery(
			turnCtx, l.LLM,
			LLMRequest{Model: sc.Model, SystemPrompt: params.SystemPrompt, Messages: apiMessages, Tools: params.Tools},
			l.Compress, &messages,
		)
		apiMessages = messages
		if prepend != nil && len(uc) > 0 {
			apiMessages = prepend(messages, uc)
		}
		if err != nil {
			if l.FallbackLLM != nil && l.FallbackOnErr != nil && l.FallbackOnErr(err) && len(toolRoundResults) == 0 {
				chunks, err = l.FallbackLLM.Call(turnCtx, LLMRequest{Model: sc.Model, SystemPrompt: params.SystemPrompt, Messages: apiMessages, Tools: params.Tools})
			}
			if err != nil {
				if llmSpan != nil {
					llmSpan.RecordError(err)
					llmSpan.End()
				}
				return nil, err
			}
		}

		var pending []ToolCall
		assistantText = ""
		var iterUsage TokenUsage
		for chunk := range chunks {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if chunk.Thinking != "" && emit != nil {
				emitThinking(emit, sc.SessionID, chunk.Thinking)
			}
			if chunk.Content != "" {
				assistantText += chunk.Content
				if emit != nil {
					emitText(emit, sc.SessionID, chunk.Content, false)
				}
			}
			if len(chunk.ToolCalls) > 0 {
				pending = chunk.ToolCalls
			}
			if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
				iterUsage = TokenUsage{
					PromptTokens:     chunk.Usage.PromptTokens,
					CompletionTokens: chunk.Usage.CompletionTokens,
				}
			}
		}
		usage.PromptTokens += iterUsage.PromptTokens
		usage.CompletionTokens += iterUsage.CompletionTokens

		if llmSpan != nil {
			llmSpan.SetAttributes(
				tracer.Attribute{Key: "llm.prompt_tokens", Value: fmt.Sprintf("%d", iterUsage.PromptTokens)},
				tracer.Attribute{Key: "llm.completion_tokens", Value: fmt.Sprintf("%d", iterUsage.CompletionTokens)},
				tracer.Attribute{Key: "llm.latency_ms", Value: fmt.Sprintf("%d", time.Since(llmStart).Milliseconds())})
			llmSpan.End()
		}

		if len(pending) == 0 {
			break
		}

		refs := make([]conversation.ToolCallRef, len(pending))
		for i, tc := range pending { refs[i] = conversation.ToolCallRef{ID: tc.ID, Name: tc.Name, Input: tc.Input} }
		refs = conversation.DedupeToolCalls(refs)

		messages = append(messages, conversation.BuildAssistantToolCallsMessage(sc.SessionID, assistantText, refs))

		newRecords, newResults := l.executeToolRefs(turnCtx, sc, refs, emit, endTurn)
		allToolRecords = append(allToolRecords, newRecords...)
		toolRoundResults = append(toolRoundResults[:0], newResults...)

		assistantText = ""
		turn++
		if sc != nil {
			sc.QueryDepth++
		}
	}

	assistantText = strings.TrimSpace(assistantText)
	var outMsgs []types.Message
	if assistantText != "" { outMsgs = []types.Message{{Role: types.MessageRoleAssistant, Content: assistantText, SessionID: sc.SessionID}} }
	return &Result{Messages: outMsgs, AssistantText: assistantText, Usage: usage, TurnCount: turn, ToolCallHistory: allToolRecords}, nil
}

// startLoopSpan creates a child span for Loop operations.
func (l *Loop) startLoopSpan(ctx context.Context, operation string, kind tracer.SpanKind, attrs ...tracer.Attribute) (context.Context, tracer.Span) {
	if l.Observability == nil || !l.Observability.IsEnabled() {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(kind),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(operation, attrs...)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return l.Observability.Tracer().Start(ctx, operation, opts...)
}
