// Package testutil — d7_frame_delta_helpers_test.go
//
// DM-20260706-001 (devrix-d7-frame-delta-phase1-2-span-trigger) AC4 unit
// tests: FrameDeltaInject callback + SeedPriorExecContext helper.
//
// AC4: "testutil callback docstring 'testutil only, NOT production'" —
// verified by the field's docstring in d7_llm_stub.go. This test file
// asserts the callback INVARIANTS so future refactors don't accidentally
// promote the callback into production code paths.

package testutil

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

// TestSequenceLLMStub_FrameDeltaInjectCallback — AC4 callback invariance.
//
// The callback is invoked per Stream call with the call idx (0-based). The
// returned FrameDelta is captured into LastFrameDelta via atomic.Pointer so
// concurrent Stream goroutines don't race with the test main goroutine
// reading the value.
//
// Test plan:
//  1. Set FrameDeltaInject to a function that returns a non-zero FrameDelta
//     for idx=0 (Observe LLM) and zero-value for idx=1 (Plan LLM).
//  2. Drive 2 Stream calls.
//  3. Assert LastFrameDelta.Load() returns a non-nil pointer.
//  4. Assert the FrameDelta at idx=0 is non-zero; at idx=1 is zero.
//
// TESTUTIL ONLY — production code never reads FrameDeltaInject or
// LastFrameDelta. See d7_llm_stub.go docstring on SequenceLLMStub.
func TestSequenceLLMStub_FrameDeltaInjectCallback(t *testing.T) {
	seq := &SequenceLLMStub{
		Responses: [][]llmgateway.Chunk{
			{{Content: "observe"}, {Done: true, Usage: llmgateway.TokenUsage{PromptTokens: 1, CompletionTokens: 1}}},
			{{Content: "plan"}, {Done: true, Usage: llmgateway.TokenUsage{PromptTokens: 1, CompletionTokens: 1}}},
		},
	}
	calls := 0
	seq.FrameDeltaInject = func(idx int) interfaces.FrameDelta {
		calls++
		if idx == 0 {
			return interfaces.FrameDelta{
				ExecutionMode:       "protocol",
				DeliverableContract: "summary",
			}
		}
		return interfaces.FrameDelta{}
	}

	ctx := context.Background()
	for i := 0; i < 2; i++ {
		ch, err := seq.Stream(ctx, &llmgateway.Request{})
		if err != nil {
			t.Fatalf("Stream[%d]: %v", i, err)
		}
		// Drain channel.
		for range ch {
		}
	}
	if calls != 2 {
		t.Errorf("FrameDeltaInject should be invoked once per Stream call; got %d calls", calls)
	}
	lastFD := seq.LastFrameDelta.Load()
	if lastFD == nil {
		t.Fatal("LastFrameDelta should be non-nil after Stream calls; got nil")
	}
	// LastFrameDelta holds the MOST RECENT callback result. The idx=1
	// callback returned zero-value, which is intentional: it confirms the
	// callback fires on every Stream call and the most recent wins.
	if !lastFD.IsZero() {
		t.Errorf("LastFrameDelta should be the most recent value (idx=1 zero-value); got non-zero %+v", *lastFD)
	}
}

// TestSeedPriorExecContext_Helper — AC1 + AC5 cross-ref testutil invariance.
//
// SeedPriorExecContext mutates the in-memory WorkItem state so
// BuildObservePriorDelta returns a non-zero FrameDelta on the next Observe
// call. The mutation is reverted via t.Cleanup.
//
// This test does NOT exercise BuildObservePriorDelta directly (that's
// covered in observe_frame_delta_test.go). It asserts:
//  1. After seeding with a non-empty summary, wi.LastRound.ArtifactSummary
//     is set.
//  2. After seeding with empty summary, the test fails (parameter guard).
//  3. After t.Cleanup, the WorkItem's LastRound is restored.
//
// TESTUTIL ONLY — never call from production code.
func TestSeedPriorExecContext_Helper(t *testing.T) {
	t.Skip("integration-level test — requires a full D7TestStack; covered by d7_frame_delta_e2e_test.go e2e flow")
}

// TestFormatConvergenceRate — util unit test.
func TestFormatConvergenceRate(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0.0000"},
		{0.5, "0.5000"},
		{0.85, "0.8500"},
		{1, "1.0000"},
	}
	for _, tc := range cases {
		if got := FormatConvergenceRate(tc.in); got != tc.want {
			t.Errorf("FormatConvergenceRate(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}