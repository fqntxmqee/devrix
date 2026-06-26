package sessionorchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
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

	// MaxIters caps the ReAct loop. 0 → DefaultWorkItemMaxIters.
	MaxIters int
	// Now is the clock injection point for tests. nil → time.Now.
	Now func() time.Time
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
	_ = itemID // currently unused; reserved for per-item override of Context.

	result := &WorkItemResult{StartedAt: e.now(), StopReason: "started"}
	defer func() { result.EndedAt = e.now() }()

	systemPrompt, tools, prepErr := e.prepareContext(ctx, sessionID, directive)
	if prepErr != nil {
		// Non-fatal: log and continue with empty context. Better to give the
		// user a degraded answer than to fail outright on a Prepare hiccup.
		slog.Warn("workitem executor: prepare context (degraded)",
			"session_id", sessionID, "error", prepErr)
	}

	messages := []types.Message{{
		SessionID: sessionID,
		Role:      types.MessageRoleUser,
		Content:   directive,
	}}

	max := e.maxIters()
	for iter := 0; iter < max; iter++ {
		result.Iterations = iter + 1

		content, toolCalls, finishReason, err := e.streamLLM(ctx, sessionID, systemPrompt, tools, messages)
		if err != nil {
			result.StopReason = "llm_error"
			return result, fmt.Errorf("workitem executor: llm invoke (iter %d): %w", iter+1, err)
		}
		result.Content += content

		// No tool calls → final answer.
		if len(toolCalls) == 0 {
			if finishReason != "" && finishReason != "stop" {
				result.StopReason = "llm_finish_" + finishReason
				return result, fmt.Errorf("workitem executor: llm finish_reason=%s", finishReason)
			}
			result.Done = true
			result.StopReason = "final_answer"
			return result, nil
		}

		result.ToolCalls += len(toolCalls)

		// LLM wants tools. Append the assistant message (so the model sees
		// its own tool_call history on the next iteration) and execute.
		messages = append(messages, buildWorkItemAssistantToolCallMsg(sessionID, toolCalls, content))

		if e.Tools == nil {
			// No tool executor wired — degrade gracefully with what we have
			// so the user isn't left with an empty reply.
			result.StopReason = "tool_no_executor"
			return result, nil
		}

		round, err := e.Tools.ExecuteRound(ctx, ToolRoundRequest{
			SessionID: sessionID,
			ToolCalls: toolCalls,
		})
		if err != nil {
			result.StopReason = "tool_error"
			return result, fmt.Errorf("workitem executor: tool round (iter %d): %w", iter+1, err)
		}
		if len(round.Results) == 0 {
			// Executor returned no results — break with what we have rather
			// than spin forever on a silent failure.
			result.StopReason = "tool_no_results"
			return result, nil
		}

		// Append tool result messages, paired 1:1 by index with the requested
		// tool_calls. If the executor returns fewer results than requested
		// (truncated batch), only append the available pairings.
		for i := range toolCalls {
			if i >= len(round.Results) {
				break
			}
			messages = append(messages, buildWorkItemToolResultMsg(sessionID, round.Results[i]))
		}
	}

	// Cap reached without a tool-call-free final answer. Return the
	// accumulated text so the user sees something rather than nothing.
	result.StopReason = "max_iters"
	return result, nil
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
func (e *DefaultWorkItemExecutor) streamLLM(
	ctx context.Context,
	sessionID, systemPrompt string,
	tools []ToolSchema,
	messages []types.Message,
) (string, []llmgateway.ToolCall, string, error) {
	ch, err := e.LLM.InvokeStream(ctx, orchtypes.LLMInvokeRequest{
		SessionID:    sessionID,
		SystemPrompt: systemPrompt,
		Messages:     messages,
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
		}
		if chunk.Thinking != "" {
			content.WriteString(chunk.Thinking)
		}
		if len(chunk.ToolCalls) > 0 {
			toolCalls = chunk.ToolCalls
		}
		if chunk.FinishReason != "" {
			finishReason = chunk.FinishReason
		}
	}
	return content.String(), toolCalls, finishReason, nil
}

// prepareContext calls ContextPreparer.Prepare to assemble SystemPrompt +
// Tools. Returns empty defaults when ContextPreparer is nil (legacy bare
// call preservation so test fixtures that don't wire Context keep working).
func (e *DefaultWorkItemExecutor) prepareContext(ctx context.Context, sessionID, directive string) (string, []ToolSchema, error) {
	if e.Context == nil {
		return "", nil, nil
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
		return "", nil, err
	}
	return prepared.SystemPrompt, prepared.Tools, nil
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