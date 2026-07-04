package sessionorchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/materialize"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// DefaultWorkItemMaxIters is the safety cap for one WorkItem's ReAct loop.
//
// Per-WorkItem scope is intentionally smaller than per-Turn scope: a single
// user instruction should resolve in ≤5 LLM↔Tool iterations. Tuned to leave
// room for tool-chain questions (e.g. read_file → grep → read_file → answer)
// without letting a stuck LLM burn tokens indefinitely.
//
// Per-WorkItem MAX_TURNS is a hard cap (unlike TurnOrchestrator's soft
// MaxTurns safety net) because WorkItemExecutor has no natural turn count
// signal — a single user message either resolves or it doesn't.
const DefaultWorkItemMaxIters = 5

// DefaultWorkItemTokenBudget is the default Materialize token budget for WorkItem execute.
const DefaultWorkItemTokenBudget = 8000

// WorkItemSourceLabel is the SourcePlanID prefix used when ItemPipelineRunner
// bypasses the Planner and feeds the directive straight into WorkItemExecutor.
// Downstream Verify/Learn keys off this to distinguish ReAct-origin artifacts
// from Planner-Channel-origin artifacts (item_pipeline.go).
const WorkItemSourceLabel = "workitem_executor"

// WorkItemExecutor is the per-WorkItem execution contract.
//
// ItemPipelineRunner holds a WorkItemExecutor (not a concrete type) so unit
// tests can inject a stub without spinning up a real LLMInvoker /
// ToolRoundExecutor. The production implementation is DefaultWorkItemExecutor.
type WorkItemExecutor interface {
	// ExecuteWorkItem runs the per-WorkItem ReAct loop and returns the
	// result. Implementations MUST be safe to call from a single goroutine
	// (ItemPipelineRunner does not pool executors).
	//
	//	sessionID — gateway session id, threaded through D2 boundary calls.
	//	itemID    — used for Artifact tracing; does NOT feed into the LLM context.
	//	directive — the WorkItem's user-facing intent (typically item.Directive
	//	            or item.Title). Becomes the initial user message.
	ExecuteWorkItem(ctx context.Context, sessionID, itemID, directive string) (*WorkItemResult, error)
}

// WorkItemResult is the executor's output, ready for Verify/Learn to
// consume and for Artifact construction.
type WorkItemResult struct {
	// Content is the LLM's accumulated text across all iterations. When
	// Done=true this is the final answer; when Done=false (cap hit) this
	// is whatever the LLM produced before the loop terminated.
	Content string
	// Done is true iff the LLM returned a tool-call-free final answer.
	Done bool
	// Iterations counts LLM calls actually issued (including the final one).
	Iterations int
	// ToolCalls counts total tool invocations executed across iterations.
	ToolCalls int
	// StopReason labels the termination: final_answer | max_iters |
	// tool_error | tool_no_executor | tool_no_results | llm_error.
	StopReason string
	// StartedAt / EndedAt bracket the entire ReAct loop for Artifact
	// duration accounting.
	StartedAt time.Time
	EndedAt   time.Time
}

// DefaultWorkItemExecutor is the production WorkItemExecutor. It drives one
// WorkItem to completion by calling the LLM with workspace context + tools,
// executing any tool_calls the LLM returns, and looping until the LLM
// produces a tool-call-free final answer or the iteration cap is reached.
//
// Lifecycle is per-WorkItem (NOT per-Turn). PersistTurn is intentionally
// NOT called here — WorkItem-round persistence is handled by the parent
// ItemPipelineRunner via Tasks.ApplyPipelineRound, avoiding double-write.
//
// D7→D2/D3 boundary respect:
//
//	D7→D2: ContextPreparer.Prepare, ToolRoundExecutor.ExecuteRound
//	D7→D3: LLMInvoker.InvokeStream
//
// D7 owns the LLM call authority (D7-S2-A07). D2→D3 is BANNED (DM-020).
//
// DM-20260626-009 follow-up.
type DefaultWorkItemExecutor struct {
	LLM     orchtypes.LLMInvoker
	Context ContextPreparer
	Tools   ToolRoundExecutor
	// Materializer is the D2 Materialize adapter (DM-20260627-003). nil → legacy Prepare only.
	Materializer materialize.Materializer
	// TokenBudget caps Materialize private chain size. 0 → DefaultWorkItemTokenBudget.
	TokenBudget int

	// MaxIters caps the ReAct loop. 0 → DefaultWorkItemMaxIters.
	MaxIters int
	// Now is the clock injection point for tests. nil → time.Now.
	Now func() time.Time
	// Emit forwards intermediate engine events (text / thinking / tool_call /
	// tool_result) to the gateway so the user-visible stream mirrors what
	// gateway feishu cards. nil → silent
	// (legacy / test fixtures). ItemPipelineRunner sets this in
	// RunSessionTurnLoop so per-WorkItem tool calls land in feishu reply
	// cards; Wave path already does this via subagent.streamEmit.
	//
	// Hotfix (2026-06-27): without this hook, ItemPipelineRunner's ReAct
	// loop ran 4 tool.bash calls but emitted only the final ArtifactSummary
	// as a `text` event — feishu cards showed only the LLM's last paragraph
	// with no tool-call evidence. See sessionorchestrator.workitem_executor.go
	// history; the regression was introduced when ItemPipelineRunner became
	// the default execution surface (DM-20260626-009).
	Emit func(*contracts.EngineEvent)
	// userContextPrepend is set per ExecuteWorkItem from ContextPreparer
	// (API-boundary AGENTS.md when user_context.mode=prepend|both).
	userContextPrepend map[string]string
}

// Compile-time interface check.
var _ WorkItemExecutor = (*DefaultWorkItemExecutor)(nil)

// NewWorkItemExecutor constructs a production executor. LLM is required;
// Context and Tools are optional (nil Context degrades to a bare LLM call
// matching PR #249's degraded behaviour; nil Tools skips tool execution).
func NewWorkItemExecutor(llm orchtypes.LLMInvoker, ctx ContextPreparer, tools ToolRoundExecutor) *DefaultWorkItemExecutor {
	return &DefaultWorkItemExecutor{LLM: llm, Context: ctx, Tools: tools}
}

// ExecuteWorkItem runs the per-WorkItem ReAct loop. See WorkItemExecutor
// interface for parameter semantics.
func (e *DefaultWorkItemExecutor) ExecuteWorkItem(ctx context.Context, sessionID, itemID, directive string) (*WorkItemResult, error) {
	if e == nil {
		return nil, fmt.Errorf("workitem executor: nil receiver")
	}
	if e.LLM == nil {
		return nil, fmt.Errorf("workitem executor: LLMInvoker required")
	}
	directive = strings.TrimSpace(directive)
	if directive == "" {
		return nil, fmt.Errorf("workitem executor: directive required")
	}
	_ = itemID // reserved for span attrs; Materialize partition comes from ctx.

	llmDirective := directive
	if ec, ok := WorkItemExecContextFrom(ctx); ok {
		if ec.DeliverableContract.ContractApplicable() {
			llmDirective = AppendDeliverableContractExecuteHint(directive, ec.DeliverableContract)
		} else {
			llmDirective = AppendDeliverableExecuteHint(directive, ec.DeliverableSchema)
		}
		// RH-MUPS-10 (DM-20260701-001): on a retry of a non-Pass round, the
		// producer must see WHY the prior attempt failed so it can self-
		// correct. We prepend a "PriorVerifyReason" section to the LLM
		// directive (separate from the schema-tag acceptance bar) — empty
		// when no prior failure, so the message is unchanged for first-pass
		// items. The D7 orchestrator already routes the retry through
		// SpawnInline so we only land here when the policy decided another
		// attempt is worthwhile.
		if ec.PriorVerifyReason != "" {
			llmDirective = strings.TrimSpace(llmDirective) +
				"\n\nPriorVerifyReason: " + ec.PriorVerifyReason +
				"\n(Adjust your approach to address the above; the previous attempt failed verification for this reason.)"
		}
	}

	result := &WorkItemResult{StartedAt: e.now(), StopReason: "started"}
	defer func() { result.EndedAt = e.now() }()

	systemPrompt, tools, messages, userContextPrepend, prepErr := e.prepareContext(ctx, sessionID, itemID, llmDirective)
	baseMessageCount := len(messages)
	if prepErr != nil {
		// Non-fatal: log and continue with empty context. Better to give the
		// user a degraded answer than to fail outright on a Prepare hiccup.
		slog.Warn("workitem executor: prepare context (degraded)",
			"session_id", sessionID, "error", prepErr)
	}
	if len(messages) == 0 {
		messages = []types.Message{{
			SessionID: sessionID,
			Role:      types.MessageRoleUser,
			Content:   llmDirective,
		}}
	}

	max := e.maxIters()
	if ec, ok := WorkItemExecContextFrom(ctx); ok && ec.MaxItersOverride > 0 {
		max = ec.MaxItersOverride
	}
	for iter := 0; iter < max; iter++ {
		result.Iterations = iter + 1
		ctx = workmodel.WithLocatorPhase(ctx, string(workmodel.RoundPhaseExecute))
		ctx = workmodel.WithLocatorIter(ctx, iter+1)

		iterTools := tools
		iterCtx := ctx
		if ec, ok := WorkItemExecContextFrom(ctx); ok && iter == max-1 {
			if ec.Item != nil && ec.Item.NeedsRollup && !workmodel.RequiresSynthesisTurn(ec.DeliverableContract) {
				iterTools = nil
				messages = append(messages, types.Message{
					SessionID: sessionID,
					Role:      types.MessageRoleUser,
					Content:   RollupSynthesisTurnExecuteHint(),
				})
			} else if workmodel.RequiresSynthesisTurn(ec.DeliverableContract) {
				iterTools = nil
				iterCtx = WithSuppressExecuteTextEmit(ctx)
				var synth strings.Builder
				if hint := workmodel.DeliverableFinalAnswerHint(ec.DeliverableContract); hint != "" {
					synth.WriteString(hint)
				}
				if extra := SynthesisTurnExecuteHint(ec.DeliverableContract); extra != "" {
					if synth.Len() > 0 {
						synth.WriteString("\n")
					}
					synth.WriteString(extra)
				}
				if synth.Len() > 0 {
					messages = append(messages, types.Message{
						SessionID: sessionID,
						Role:      types.MessageRoleUser,
						Content:   synth.String(),
					})
				}
			} else if hint := workmodel.DeliverableFinalAnswerHint(ec.DeliverableContract); hint != "" {
				messages = append(messages, types.Message{
					SessionID: sessionID,
					Role:      types.MessageRoleUser,
					Content:   hint,
				})
			}
		}

		emit := emitFromExecContext(ctx)
		stopReason, iterErr, newMessages, finishReason := e.stepOneIter(iterCtx, sessionID, systemPrompt, iterTools, messages, userContextPrepend, emit, itemID, iter+1, result)
		// Span per iter (DM-20260626-009 follow-up): a ReAct iter can stall
		// for many seconds (LLM + tool round); without this span "why did
		// this WorkItem take 16s?" required reading code instead of
		// inspecting Jaeger. finishReason comes from the LLM (stop / tool_calls
		// / length / ...) so the trace shows where each iter ended.
		// stopReason is the executor's own label (final_answer / tool_error /
		// ...), surfaced as the iter's exit state.
		endSpan := hardening.EmitSubTurnIteration(ctx, sessionID, itemID, iter+1, finishReason, stopReason)
		endSpan(iterErr)

		switch {
		case iterErr != nil:
			e.appendPrivateChainDelta(ctx, sessionID, itemID, messages[baseMessageCount:])
			return result, iterErr
		case stopReason == "final_answer" || stopReason == "tool_no_executor" || stopReason == "tool_no_results":
			e.appendPrivateChainDelta(ctx, sessionID, itemID, messages[baseMessageCount:])
			return result, nil
		}
		if len(newMessages) > 0 {
			messages = append(messages, newMessages...)
		}
	}

	// Cap reached without a tool-call-free final answer. Return the
	// accumulated text so the user sees something rather than nothing.
	result.StopReason = "max_iters"
	hardening.EmitSubTurnIteration(ctx, sessionID, itemID, max+1, "tool_calls", "max_iters")(nil)
	e.appendPrivateChainDelta(ctx, sessionID, itemID, messages[baseMessageCount:])
	return result, nil
}

// stepOneIter runs one ReAct iteration: stream LLM → optionally execute
// tool round → return outcome. The four return values are:
//   - stopReason: labels the iter's terminal state ("llm_error" /
//     "final_answer" / "llm_finish_<X>" / "tool_no_executor" / "tool_error" /
//     "tool_no_results" / "ok").
//   - iterErr: non-nil iff the iter produced a hard error (caller returns).
//   - newMessages: assistant + tool result messages to feed the next iter;
//     empty when the iter terminated.
//   - finishReason: the LLM's own finish reason for this iter ("stop" /
//     "tool_calls" / "length" / ...); threaded back so EmitSubTurnIteration
//     can surface it as the subturn.finish_reason span attribute. Empty
//     string when the LLM did not report one (e.g. tool round failure path
//     that bypassed the LLM call).
//
// stepOneIter is split out from ExecuteWorkItem so the per-iter span can
// wrap a single function call rather than bookend 6 inline return paths.
func (e *DefaultWorkItemExecutor) stepOneIter(
	ctx context.Context,
	sessionID, systemPrompt string,
	tools []ToolSchema,
	messages []types.Message,
	userContextPrepend map[string]string,
	emit func(*contracts.EngineEvent),
	itemID string,
	iter int,
	result *WorkItemResult,
) (string, error, []types.Message, string) {
	content, toolCalls, finishReason, err := e.streamLLM(ctx, sessionID, systemPrompt, tools, messages, userContextPrepend, emit)
	toolCalls = dedupeToolCalls(toolCalls)
	if err != nil {
		result.StopReason = "llm_error"
		return "llm_error", fmt.Errorf("workitem executor: llm invoke (iter %d): %w", iter, err), nil, ""
	}
	result.Content += content

	// No tool calls → final answer.
	if len(toolCalls) == 0 {
		if finishReason != "" && finishReason != "stop" {
			result.StopReason = "llm_finish_" + finishReason
			return "llm_finish_" + finishReason, fmt.Errorf("workitem executor: llm finish_reason=%s", finishReason), nil, finishReason
		}
		result.Done = true
		result.StopReason = "final_answer"
		return "final_answer", nil, nil, finishReason
	}

	result.ToolCalls += len(toolCalls)

	// LLM wants tools. Append the assistant message (so the model sees
	// its own tool_call history on the next iteration) and execute.
	newMessages := []types.Message{
		buildWorkItemAssistantToolCallMsg(sessionID, toolCalls, content),
	}

	if e.Tools == nil {
		// No tool executor wired — degrade gracefully with what we have
		// so the user isn't left with an empty reply.
		result.StopReason = "tool_no_executor"
		return "tool_no_executor", nil, nil, finishReason
	}

	round, err := e.Tools.ExecuteRound(ctx, ToolRoundRequest{
		SessionID: sessionID,
		ToolCalls: toolCalls,
	})
	if err != nil {
		result.StopReason = "tool_error"
		return "tool_error", fmt.Errorf("workitem executor: tool round (iter %d): %w", iter, err), nil, finishReason
	}
	// Emit a `tool_result` event per result so the feishu card shows the
	// tool's return value ("tool_call" → result pairing). Append-only — downstream messages
	// below carry the result content for the next LLM iter, but the
	// gateway needs the event for live card rendering. Look up tool name
	// from the originating toolCalls via ToolCallID since ToolResult itself
	// only carries ID/Output/Error.
	nameByID := make(map[string]string, len(toolCalls))
	for _, tc := range toolCalls {
		nameByID[tc.ID] = tc.Name
	}
	for _, r := range round.Results {
		body := r.Output
		if r.Error != "" {
			body = r.Error
		}
		name := nameByID[r.ToolCallID]
		emitEvent(ctx, emit, &contracts.EngineEvent{
			Type:      "tool_result",
			Content:   body,
			ToolName:  name,
			SessionID: sessionID,
			Metadata: map[string]string{
				"tool_name":    name,
				"tool_call_id": r.ToolCallID,
			},
		})
	}
	if len(round.Results) == 0 {
		// Executor returned no results — break with what we have rather
		// than spin forever on a silent failure.
		result.StopReason = "tool_no_results"
		return "tool_no_results", nil, nil, finishReason
	}

	// Append tool result messages, one per declared tool_call (by ID).
	resultByID := make(map[string]ToolResult, len(round.Results))
	for _, r := range round.Results {
		if id := strings.TrimSpace(r.ToolCallID); id != "" {
			resultByID[id] = r
		}
	}
	for _, tc := range toolCalls {
		id := strings.TrimSpace(tc.ID)
		r, ok := resultByID[id]
		if !ok {
			r = ToolResult{ToolCallID: id, Error: "tool execution did not return a result"}
		}
		newMessages = append(newMessages, buildWorkItemToolResultMsg(sessionID, r))
	}
	return "ok", nil, newMessages, finishReason
}

// streamLLM runs one LLMInvokeStream call and assembles content + tool_calls
// from the streaming chunks. Returns the assembled content, the last-batch
// tool calls, and the final finish reason.
//
// Thinking chunks are folded into Content so the user sees the model's
// reasoning alongside its text. TurnOrchestrator emits separate `thinking`
// events for richer observability; for WorkItemExecutor (which has no event
// stream of its own), folding into Content keeps the user-visible output
// coherent in one place.
//
// Emit hook: each chunk's text / thinking / tool_call stream is forwarded
// as an EngineEvent so the gateway can render intermediate state on feishu cards.
// nil Emit → no-op (legacy / tests).
func (e *DefaultWorkItemExecutor) streamLLM(
	ctx context.Context,
	sessionID, systemPrompt string,
	tools []ToolSchema,
	messages []types.Message,
	userContextPrepend map[string]string,
	emit func(*contracts.EngineEvent),
) (string, []llmgateway.ToolCall, string, error) {
	apiMessages := messagesForLLMInvoke(messages, userContextPrepend)
	ch, err := e.LLM.InvokeStream(ctx, orchtypes.LLMInvokeRequest{
		SessionID:    sessionID,
		SystemPrompt: systemPrompt,
		Messages:     apiMessages,
		Tools:        tools,
	})
	if err != nil {
		return "", nil, "", err
	}

	var content strings.Builder
	var toolCalls []llmgateway.ToolCall
	var finishReason string
	for chunk := range ch {
		if chunk.Content != "" {
			content.WriteString(chunk.Content)
			if !SuppressExecuteTextEmit(ctx) {
				emitEvent(ctx, emit, &contracts.EngineEvent{
					Type:      "text",
					Content:   chunk.Content,
					SessionID: sessionID,
				})
			}
		}
		if chunk.Thinking != "" {
			content.WriteString(chunk.Thinking)
			emitEvent(ctx, emit, &contracts.EngineEvent{
				Type:      "thinking",
				Content:   chunk.Thinking,
				SessionID: sessionID,
			})
		}
		if len(chunk.ToolCalls) > 0 {
			toolCalls = chunk.ToolCalls
		}
		if chunk.FinishReason != "" {
			finishReason = chunk.FinishReason
		}
	}
	toolCalls = dedupeToolCalls(toolCalls)
	for _, tc := range toolCalls {
		emitEvent(ctx, emit, &contracts.EngineEvent{
			Type:      "tool_call",
			ToolName:  tc.Name,
			ToolInput: tc.Input,
			SessionID: sessionID,
			Metadata: map[string]string{
				"tool_name": tc.Name,
				"input":     tc.Input,
				"call_id":   tc.ID,
			},
		})
	}
	return content.String(), toolCalls, finishReason, nil
}

// emit forwards an EngineEvent to the configured Emit hook. nil Emit is a
// no-op so legacy / test fixtures that don't wire a sink keep working.
// Used for intermediate LLM chunk (text/thinking/tool_call) + tool-result
// events; caller-driven control instead of subagent-driven streaming.
func emitEvent(ctx context.Context, emit func(*contracts.EngineEvent), ev *contracts.EngineEvent) {
	if emit == nil || ev == nil {
		return
	}
	if ctx != nil && ctx.Err() != nil {
		return
	}
	emit(ev)
}

func emitFromExecContext(ctx context.Context) func(*contracts.EngineEvent) {
	if ec, ok := WorkItemExecContextFrom(ctx); ok {
		return ec.Emit
	}
	return nil
}

// prepareContext calls ContextPreparer.Prepare or D2 Materialize to assemble
// SystemPrompt + Tools + initial messages. Returns empty defaults when both
// paths are unavailable (legacy bare call preservation).
func (e *DefaultWorkItemExecutor) prepareContext(ctx context.Context, sessionID, itemID, directive string) (string, []ToolSchema, []types.Message, map[string]string, error) {
	if ShouldMaterializeWorkItem(ctx, sessionID, itemID) && e.Materializer != nil {
		ec, _ := WorkItemExecContextFrom(ctx)
		req := BuildMaterializeRequest(sessionID, ec.Item, ec.Tasks, directive, e.tokenBudget())
		mat, err := e.Materializer.Materialize(ctx, req)
		// DM-20260630-011 AC3: emit the materialize span AFTER the call so
		// message_count / token_est reflect actual mat values (start-time
		// 0/0 would be misleading). When the materializer returned 0/0,
		// also emit a sibling empty-yield span so dashboards can filter by
		// the empty-yield condition independently.
		end := hardening.EmitContextMaterialize(ctx, sessionID, itemID, string(req.Policy.Mode), mat.MessageCount, mat.TokenEst)
		end(err)
		if mat.MessageCount == 0 && mat.TokenEst == 0 {
			endEmpty := hardening.EmitMaterializeEmptyYield(ctx, sessionID, itemID, string(req.Policy.Mode))
			endEmpty(nil)
		}
		if err != nil {
			return "", nil, nil, nil, err
		}
		tools := filterPipelineTools(toolSchemasFromDescriptors(mat.Tools))
		var userContextPrepend map[string]string
		if e.Context != nil {
			prepared, prepErr := e.Context.Prepare(ctx, PrepareRequest{
				SessionID: sessionID,
				Message: types.Message{
					SessionID: sessionID,
					Role:      types.MessageRoleUser,
					Content:   directive,
				},
			})
			if prepErr == nil {
				userContextPrepend = prepared.UserContextPrepend
				rollupSynth := ec.Item != nil && ec.Item.NeedsRollup
				if len(tools) == 0 && !rollupSynth {
					tools = filterPipelineTools(prepared.Tools)
				}
			}
		}
		return mat.SystemPrompt, tools, mat.Messages, userContextPrepend, nil
	}
	if e.Context == nil {
		return "", nil, nil, nil, nil
	}
	prepared, err := e.Context.Prepare(ctx, PrepareRequest{
		SessionID: sessionID,
		Message: types.Message{
			SessionID: sessionID,
			Role:      types.MessageRoleUser,
			Content:   directive,
		},
	})
	if err != nil {
		return "", nil, nil, nil, err
	}
	systemPrompt := appendWorkItemFormatHints(prepared.SystemPrompt, ctx, sessionID, itemID)
	msgs := []types.Message{{
		SessionID: sessionID,
		Role:      types.MessageRoleUser,
		Content:   directive,
	}}
	return systemPrompt, filterPipelineTools(prepared.Tools), msgs, prepared.UserContextPrepend, nil
}

func appendWorkItemFormatHints(systemPrompt string, ctx context.Context, sessionID, itemID string) string {
	ec, ok := WorkItemExecContextFrom(ctx)
	if !ok || ec.Item == nil || ec.Item.Kind != workmodel.WorkKindGoal {
		return systemPrompt
	}
	depth := 0
	if ec.Tasks != nil {
		depth = ec.Tasks.Tree().Depth(sessionID, itemID)
	}
	if depth > 0 {
		return systemPrompt
	}
	if strings.Contains(systemPrompt, "<scope_contract>") {
		return systemPrompt
	}
	if strings.TrimSpace(systemPrompt) != "" {
		systemPrompt += "\n"
	}
	return systemPrompt + materialize.WorkItemOutputFormatHints
}

func (e *DefaultWorkItemExecutor) appendPrivateChainDelta(ctx context.Context, sessionID, itemID string, msgs []types.Message) {
	if e == nil || e.Materializer == nil || !ShouldMaterializeWorkItem(ctx, sessionID, itemID) || len(msgs) == 0 {
		return
	}
	msgs = filterEphemeralExecuteMessages(msgs)
	msgs = conversation.RepairToolMessageChain(msgs)
	if len(msgs) == 0 {
		return
	}
	partition := ResolvePartitionForWorkItem(sessionID, &workmodel.WorkItem{ID: itemID})
	_ = e.Materializer.Append(ctx, partition, msgs)
}

// filterEphemeralExecuteMessages drops one-shot synthesis hints from the
// persisted private chain. They belong only on the in-flight LLM request;
// persisting them breaks merge/compress on the next NeedsRollup round.
func filterEphemeralExecuteMessages(msgs []types.Message) []types.Message {
	out := make([]types.Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == types.MessageRoleUser && isEphemeralExecuteHint(m.Content) {
			continue
		}
		out = append(out, m)
	}
	return out
}

func isEphemeralExecuteHint(content string) bool {
	c := strings.TrimSpace(content)
	if c == "" {
		return false
	}
	return strings.Contains(c, "<deliverable_format>") ||
		strings.Contains(c, "Final iteration: tools are disabled") ||
		strings.Contains(c, "Rollup synthesis: tools are disabled")
}

func (e *DefaultWorkItemExecutor) tokenBudget() int {
	if e != nil && e.TokenBudget > 0 {
		return e.TokenBudget
	}
	return DefaultWorkItemTokenBudget
}

func toolSchemasFromDescriptors(desc []materialize.ToolDescriptor) []ToolSchema {
	if len(desc) == 0 {
		return nil
	}
	out := make([]ToolSchema, 0, len(desc))
	for _, d := range desc {
		out = append(out, ToolSchema{
			Name:        d.Name,
			Description: d.Description,
			Parameters:  d.Schema,
		})
	}
	return out
}

func (e *DefaultWorkItemExecutor) maxIters() int {
	if e.MaxIters > 0 {
		return e.MaxIters
	}
	return DefaultWorkItemMaxIters
}

func (e *DefaultWorkItemExecutor) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

// buildWorkItemAssistantToolCallMsg serialises tool_calls into the message's
// Metadata["tool_calls"] JSON blob, matching the canonical devrix pattern
// (turn_orchestrator.go:1077 buildAssistantToolCallMsg). types.Message has
// no ToolCalls field, so the gateway adapter re-hydrates from Metadata on
// the way out to the provider.
//
// The `workitem` prefix on the helper name avoids a Go duplicate-symbol
// collision with turn_orchestrator.go's same-name helper while preserving
// the on-the-wire format both share.
func buildWorkItemAssistantToolCallMsg(sessionID string, calls []llmgateway.ToolCall, text string) types.Message {
	type serializedCall struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}
	refs := make([]serializedCall, len(calls))
	for i, c := range calls {
		id := c.ID
		if id == "" {
			id = fmt.Sprintf("call_%d", i)
		}
		refs[i].ID = id
		refs[i].Type = "function"
		refs[i].Function.Name = c.Name
		args := c.Input
		if args == "" {
			args = "{}"
		}
		refs[i].Function.Arguments = args
	}
	raw, _ := json.Marshal(refs)
	return types.Message{
		SessionID: sessionID,
		Role:      types.MessageRoleAssistant,
		Content:   text,
		Metadata:  map[string]string{"tool_calls": string(raw)},
	}
}

// buildWorkItemToolResultMsg formats one tool execution result as a tool-
// role message, matching the canonical devrix pattern (turn_orchestrator.go
// buildToolResultMsg at line 1202). The tool_call_id is carried in
// Metadata["tool_call_id"] so the gateway adapter can pair it with the
// originating assistant tool_call on the next iteration.
//
// The `workitem` prefix mirrors buildWorkItemAssistantToolCallMsg above.
func buildWorkItemToolResultMsg(sessionID string, r ToolResult) types.Message {
	content := r.Output
	if r.Error != "" {
		content = r.Error
	}
	return types.Message{
		SessionID: sessionID,
		Role:      types.MessageRoleTool,
		Content:   content,
		Metadata:  map[string]string{"tool_call_id": r.ToolCallID},
	}
}
