//go:build integration && d7

package d7integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/tests/testutil"
)

// T: D7-S2-A06-T02 — Multi-turn tool conversation (DM-020 v1.0-c Turn Loop).
//
// Verifies the most common user flow: LLM calls a tool in the first turn,
// receives tool results, then produces a final text reply in the second turn.
// This exercises the full PREPARE → LLM → TOOL_ROUND → LLM → PERSIST state
// machine in TurnOrchestrator.runLoop.
func TestIntegration_D7FastPath_MultiTurnToolConversation(t *testing.T) {
	seq := &testutil.SequenceLLMStub{
		Responses: [][]llmgateway.Chunk{
			// Turn 1: LLM returns a tool call (read_file).
			{
				{
					ToolCalls: []llmgateway.ToolCall{
						{ID: "call_1", Name: "read_file", Input: `{"path":"/tmp/test.txt"}`},
					},
				},
				{Done: true, Usage: llmgateway.TokenUsage{PromptTokens: 10, CompletionTokens: 5}},
			},
			// Turn 2: LLM returns final text after receiving tool result.
			{
				{Content: "I've read the file, here's what I found"},
				{Done: true, Usage: llmgateway.TokenUsage{PromptTokens: 20, CompletionTokens: 8}},
			},
		},
	}

	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{
		LLMStub: seq,
	})
	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// "hi" → IntentFast → RunSessionTurnLoop → WorkItemExecutor multi-turn ReAct.
	routeAndWait(t, stack, session.SessionID, "hi")

	// Verify: exactly 2 LLM calls (tool call + final reply).
	if got := seq.CallCount.Load(); got != 2 {
		t.Fatalf("expected 2 LLM invocations (tool call + final reply), got %d", got)
	}

	// Item pipeline ingress consolidates tool telemetry into pipeline_round;
	// D1 outbound carries text + complete. Assert the final answer surfaced.
	var sawText bool
	for _, msg := range stack.Handler.OutboundMessages() {
		if strings.Contains(msg.Content, "I've read the file") {
			sawText = true
			break
		}
	}
	if !sawText {
		t.Errorf("expected final text in outbound, got: %+v", stack.Handler.OutboundMessages())
	}
}

// T: D7-S2-A06-T03 — Multi-turn LLM max turns cap.
//
// Verifies that the TurnOrchestrator stops after maxTurns and emits a complete
// event even when the LLM keeps returning tool calls.
func TestIntegration_D7FastPath_MaxTurnsCap(t *testing.T) {
	// Build responses that always return a tool call (never text-only).
	// MaxTurns defaults to 8; we provide 10 tool-call responses to verify
	// the cap stops at 8.
	responses := make([][]llmgateway.Chunk, 10)
	for i := range responses {
		responses[i] = []llmgateway.Chunk{
			{
				ToolCalls: []llmgateway.ToolCall{
					{ID: "call_loop", Name: "read_file", Input: `{"path":"/tmp/test.txt"}`},
				},
			},
			{Done: true, Usage: llmgateway.TokenUsage{PromptTokens: 1, CompletionTokens: 1}},
		}
	}
	seq := &testutil.SequenceLLMStub{Responses: responses}

	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{
		LLMStub: seq,
	})
	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	routeAndWait(t, stack, session.SessionID, "hi")

	if got := seq.CallCount.Load(); got > 9 {
		t.Fatalf("expected ≤ 9 LLM calls (max 8 turns + 1 initial), got %d", got)
	}

	// Verify: complete event emitted despite max turns.
	var sawComplete bool
	for _, msg := range stack.Handler.OutboundMessages() {
		if msg.Metadata["event_type"] == "complete" {
			sawComplete = true
		}
	}
	if !sawComplete {
		t.Error("expected complete event even at max turns")
	}
}

// T: D7-S2-A06-T04 — Turn handles cancellation during slow LLM.
//
// Verifies that StopProcess cancels an in-flight Turn gracefully.
func TestIntegration_D7FastPath_ContextCancellation(t *testing.T) {
	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{
		LLMStub: &testutil.D7LLMStub{
			Response: "slow reply",
			Delay:    2 * time.Second,
		},
	})
	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = stack.Gateway.RouteInbound(context.Background(), &types.InboundMessage{
			SessionID: session.SessionID,
			ChatID:    "chat-cancel",
			MessageID: "msg-cancel",
			Content:   "hi",
			UserID:    "user-d7",
		})
	}()

	time.Sleep(100 * time.Millisecond)
	if err := stack.Gateway.StopProcess(session.SessionID); err != nil {
		t.Fatalf("StopProcess: %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RouteInbound did not return after StopProcess")
	}
	stack.Gateway.WaitForProcesses()
}
