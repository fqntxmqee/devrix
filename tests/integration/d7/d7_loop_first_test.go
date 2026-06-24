//go:build integration && d7

package d7integration

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/tests/testutil"
)

// T: D7-S2-L5-01 — loop_first greeting uses Turn, no ingress Orchestrate.
func TestIntegration_D7LoopFirst_GreetingNoWave(t *testing.T) {
	stub := &testutil.D7LLMStub{Response: "你好！"}
	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{LLMStub: stub})

	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	routeAndWait(t, stack, session.SessionID, "你好")

	if got := stub.CallCount.Load(); got == 0 {
		t.Fatal("greeting should invoke Turn LLM at least once")
	}
	for _, msg := range stack.Handler.OutboundMessages() {
		if strings.Contains(msg.Content, "plan_formed") || strings.Contains(msg.Content, "wave_started") {
			t.Fatalf("unexpected wave content in outbound: %q", msg.Content)
		}
	}
}

// T: D7-S2-L5-02 — delegate_wave tool gates OrchestratePath under loop_first.
func TestIntegration_D7LoopFirst_DelegateWaveTool(t *testing.T) {
	fake := &fakeWaveScheduler{
		artifacts: []wavescheduler.Artifact{{
			TaskID:  "task_1",
			Summary: "loop-first wave summary",
			ExitCode: 0,
		}},
	}
	stub := &testutil.SequenceLLMStub{
		Responses: [][]llmgateway.Chunk{{
			{ToolCalls: []llmgateway.ToolCall{{
				Name:  "delegate_wave",
				ID:    "tc1",
				Input: `{"goal":"design auth refactor && add tests"}`,
			}}},
			{Done: true, Usage: llmgateway.TokenUsage{PromptTokens: 1, CompletionTokens: 1}},
		}},
	}
	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{
		LLMStub: stub,
		OverrideOrchestratePath: sessionorchestrator.NewOrchestratePath(
			decisionplanning.NewTaskDecomposer(),
			fake,
			nil,
		),
	})

	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	routeAndWait(t, stack, session.SessionID, "please orchestrate this complex goal")

	if !fake.startCalled() {
		t.Fatal("delegate_wave must call WaveScheduler.Start")
	}
	var sawSummary bool
	for _, msg := range stack.Handler.OutboundMessages() {
		if strings.Contains(msg.Content, "loop-first wave summary") {
			sawSummary = true
		}
	}
	if !sawSummary {
		t.Fatalf("expected wave summary in outbound, got: %+v", stack.Handler.OutboundMessages())
	}
}
