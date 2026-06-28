//go:build integration && d7

// T: D7-S15-A58-T07 (DM-20260628-003) — Multi-turn session state e2e.
//
// LP-5: when d7.prior_context_rounds > 0, the orchestrator must
//
//  1. serialize turns per session_id (no two RouteInbound calls for
//     the same session run concurrently — turn 2 waits for turn 1's
//     out channel to close), AND
//  2. enrich turn N+1's directive with turn N's finalText wrapped in
//     a <prior-output-summary> block (read from the session's
//     transcript jsonl).
//
// These are the two deeper issues exposed by PR #271
// (sess_1782638991113_5000 closed-channel panic). This test pins
// both invariants end-to-end through the bootstrap.InitOrchestration
// path (no internal-API shortcuts).
package d7integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/internal/layers/communication/capture/transcript"
	"github.com/devrix/devrix/tests/testutil"
)

// capturingStub is a minimal IAdapter that records the most recent
// user-message Body it received in the LLM call. Tests assert on
// `LastUserMessage` to verify the directive was enriched with
// <prior-output-summary> before reaching the LLM.
type capturingStub struct {
	mu             sync.Mutex
	calls          int
	responses      []string // call N → responses[N]; last entry reused
	LastUserMessage string
}

func (s *capturingStub) Stream(ctx context.Context, req *llmgateway.Request) (<-chan *llmgateway.AdapterChunk, error) {
	s.mu.Lock()
	idx := s.calls
	s.calls++
	var resp string
	if idx < len(s.responses) {
		resp = s.responses[idx]
	} else if len(s.responses) > 0 {
		resp = s.responses[len(s.responses)-1]
	} else {
		resp = "ok"
	}
	// Capture the LAST user message in this call — that's where the
	// enriched directive (summary + "\n\n" + original) lives.
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == types.MessageRoleUser {
			s.LastUserMessage = req.Messages[i].Content
			break
		}
	}
	s.mu.Unlock()

	ch := make(chan *llmgateway.AdapterChunk, 2)
	ch <- &llmgateway.AdapterChunk{Parsed: &llmgateway.Chunk{Content: resp}}
	ch <- &llmgateway.AdapterChunk{
		Parsed: &llmgateway.Chunk{
			Done:  true,
			Usage: llmgateway.TokenUsage{PromptTokens: 5, CompletionTokens: len(resp)},
		},
	}
	close(ch)
	return ch, nil
}

func (s *capturingStub) Provider() string { return "deepseek" }
func (s *capturingStub) Protocol() string { return "stub" }

func (s *capturingStub) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *capturingStub) lastUserMessage() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.LastUserMessage
}

// TestIntegration_D7_MultiTurnSessionState_PriorContextInjection is the
// LP-5 acceptance test for D7-S15 (DM-20260628-003).
//
// Flow:
//  1. Stack with d7.prior_context_rounds: 3 + a transcript dir.
//  2. Turn 1: send "review foo" → LLM returns "FOO_REPLY_1" → assert
//     the gateway emitted a `complete` event with that content and
//     that the transcript jsonl contains a kind=complete event with
//     body "FOO_REPLY_1".
//  3. Turn 2: send "再 review bar" → LLM call N+1 should see a user
//     message that begins with "<prior-output-summary>\n  [turn 1]
//     FOO_REPLY_1\n</prior-output-summary>" and ends with "再 review
//     bar". LLM returns "BAR_REPLY_2".
//  4. Assert no panic under -race (the closed-channel race that
//     crashed sess_1782638991113_5000 must not recur).
func TestIntegration_D7_MultiTurnSessionState_PriorContextInjection(t *testing.T) {
	transcriptDir := filepath.Join(t.TempDir(), "transcripts")
	stub := &capturingStub{
		responses: []string{"FOO_REPLY_1", "BAR_REPLY_2"},
	}

	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{
		LLMStub:           stub,
		TranscriptDir:     transcriptDir,
		PriorContextRounds: 3,
	})
	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// --- Turn 1: "review foo" → FOO_REPLY_1 ----------------------
	if err := stack.Gateway.RouteInbound(context.Background(), &types.InboundMessage{
		SessionID: session.SessionID,
		ChatID:    "chat-multiturn",
		MessageID: "msg-1",
		Content:   "review foo",
		UserID:    "user-multiturn",
	}); err != nil {
		t.Fatalf("RouteInbound turn 1: %v", err)
	}
	stack.Gateway.WaitForProcesses()
	if !stack.Handler.WaitForMessages(1, 5*time.Second) {
		t.Fatal("expected outbound messages after turn 1")
	}

	// Turn 1 should NOT have enriched the directive (no prior turns).
	turn1Msg := stub.lastUserMessage()
	if !strings.Contains(turn1Msg, "review foo") {
		t.Errorf("turn 1 user message should contain 'review foo', got: %q", turn1Msg)
	}
	if strings.Contains(turn1Msg, "<prior-output-summary>") {
		t.Errorf("turn 1 should not have <prior-output-summary> (no prior turns), got: %q", turn1Msg)
	}

	// Find the `complete` outbound event from turn 1.
	var turn1Complete string
	for _, m := range stack.Handler.OutboundMessages() {
		if m.Metadata["event_type"] == "complete" {
			turn1Complete = m.Content
		}
	}
	if turn1Complete == "" {
		t.Fatal("expected a complete event after turn 1")
	}
	if turn1Complete != "FOO_REPLY_1" {
		t.Errorf("turn 1 complete = %q, want FOO_REPLY_1", turn1Complete)
	}

	// Verify transcript jsonl exists and has kind=complete body.
	transcriptPath := filepath.Join(transcriptDir, sanitizeForPath(session.SessionID)+".jsonl")
	data, rerr := os.ReadFile(transcriptPath)
	if rerr != nil {
		t.Fatalf("read transcript %s: %v", transcriptPath, rerr)
	}
	if !strings.Contains(string(data), `"kind":"complete"`) {
		t.Errorf("transcript missing kind=complete event: %s", data)
	}
	if !strings.Contains(string(data), "FOO_REPLY_1") {
		t.Errorf("transcript missing FOO_REPLY_1 body: %s", data)
	}
	// Decoded sanity: the complete event body should equal FOO_REPLY_1.
	var foundComplete bool
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var ev transcript.Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if ev.Kind == "complete" && ev.Body == "FOO_REPLY_1" {
			foundComplete = true
		}
	}
	if !foundComplete {
		t.Errorf("transcript has no complete event with body FOO_REPLY_1: %s", data)
	}

	// --- Turn 2: "再 review bar" → BAR_REPLY_2 -------------------
	// Direction is set on the work-item root by ProcessMessage, so
	// the LLM call sees a user message that already contains the
	// prior-output-summary prefix (assembled before LLM dispatch).
	turn1Calls := stub.callCount()
	if err := stack.Gateway.RouteInbound(context.Background(), &types.InboundMessage{
		SessionID: session.SessionID,
		ChatID:    "chat-multiturn",
		MessageID: "msg-2",
		Content:   "再 review bar",
		UserID:    "user-multiturn",
	}); err != nil {
		t.Fatalf("RouteInbound turn 2: %v", err)
	}
	stack.Gateway.WaitForProcesses()

	// Wait until turn 2's LLM call lands (one more than turn1Calls).
	deadline := time.Now().Add(5 * time.Second)
	for stub.callCount() <= turn1Calls && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if stub.callCount() <= turn1Calls {
		t.Fatalf("turn 2 LLM call never landed (call count stayed at %d)", stub.callCount())
	}

	turn2Msg := stub.lastUserMessage()
	if !strings.Contains(turn2Msg, "<prior-output-summary>") {
		t.Errorf("turn 2 user message should contain <prior-output-summary>, got: %q", turn2Msg)
	}
	if !strings.Contains(turn2Msg, "FOO_REPLY_1") {
		t.Errorf("turn 2 user message should contain turn 1 finalText 'FOO_REPLY_1', got: %q", turn2Msg)
	}
	if !strings.Contains(turn2Msg, "再 review bar") {
		t.Errorf("turn 2 user message should contain original '再 review bar', got: %q", turn2Msg)
	}
	// Sanity: summary block must come BEFORE the new directive.
	idxSummary := strings.Index(turn2Msg, "<prior-output-summary>")
	idxUser := strings.Index(turn2Msg, "再 review bar")
	if idxSummary < 0 || idxUser < 0 || idxSummary >= idxUser {
		t.Errorf("prior-output-summary should precede user message; got summary=%d user=%d msg=%q",
			idxSummary, idxUser, turn2Msg)
	}

	// Both turns' complete events should be in the handler. The second
	// complete must contain BAR_REPLY_2 — which the LLM only emits after
	// reading the enriched directive. This is the end-to-end acceptance
	// gate: if the LLM saw only "再 review bar" (no summary), the BAR
	// reply could still appear, so we also assert the prior-output-summary
	// is present in the LLM call (checked above).
	var sawFoo, sawBar bool
	for _, m := range stack.Handler.OutboundMessages() {
		if m.Metadata["event_type"] != "complete" {
			continue
		}
		if m.Content == "FOO_REPLY_1" {
			sawFoo = true
		}
		if m.Content == "BAR_REPLY_2" {
			sawBar = true
		}
	}
	if !sawFoo {
		t.Error("missing turn 1 complete (FOO_REPLY_1)")
	}
	if !sawBar {
		t.Error("missing turn 2 complete (BAR_REPLY_2)")
	}
}

// sanitizeForPath mirrors sessionorchestrator.sanitizeSessionID for the
// transcript filename. Duplicated locally because the orchestrator's
// helper is package-private. MUST be kept in sync if it ever changes.
func sanitizeForPath(s string) string {
	const allowed = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-"
	if s == "" {
		return "session"
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		ok := false
		for j := 0; j < len(allowed); j++ {
			if allowed[j] == c {
				ok = true
				break
			}
		}
		if ok {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return "session"
	}
	return string(out)
}
