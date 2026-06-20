package turn

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	derrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestSubTurnRunner_RunSubTurn(t *testing.T) {
	llm := &stubLLM{chunks: []llmgateway.Chunk{textChunk("sub answer"), doneChunk()}}
	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: &stubContext{}, Tools: &stubTools{}, Persist: &stubPersist{}, MaxTurns: 4,
	})
	runner := NewSubTurnRunner(orch, SubTurnConfig{DefaultMode: "full"})

	res, err := runner.RunSubTurn(context.Background(), contracts.SubTurnRequest{
		SessionID:    "sess-sub",
		SystemPrompt: "explore",
		Messages:     []types.Message{{Role: types.MessageRoleUser, Content: "scan"}},
		MaxTurns:     2,
		Scope:        contracts.SubTurnScopeSubQuery,
		ChildContext: &types.SessionContext{SessionID: "sess-sub", AgentID: "explore_1"},
	})
	if err != nil {
		t.Fatalf("RunSubTurn: %v", err)
	}
	if res == nil || res.AssistantText == "" {
		t.Fatalf("expected assistant text, got %+v", res)
	}
}

// captureStubLLM records the messages that the orchestrator actually sent
// to the LLM. Used by Phase B mode tests (T14-T17) to assert PreloadedMessages
// shape per dispatch mode.
type captureStubLLM struct {
	mu       atomic.Int64
	captured atomic.Pointer[[]types.Message]
	chunks   []llmgateway.Chunk
}

func (c *captureStubLLM) InvokeStream(_ context.Context, req LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
	c.mu.Add(1)
	msgs := append([]types.Message(nil), req.Messages...)
	c.captured.Store(&msgs)
	ch := make(chan llmgateway.Chunk, len(c.chunks))
	for _, chk := range c.chunks {
		ch <- chk
	}
	close(ch)
	return ch, nil
}

func (c *captureStubLLM) lastMessages() []types.Message {
	p := c.captured.Load()
	if p == nil {
		return nil
	}
	return *p
}

// buildSubTurnFixture wires an orchestrator that records LLM messages so
// tests can assert what SubTurnRunner actually dispatched.
func buildSubTurnFixture(t *testing.T) (*captureStubLLM, *SubTurnRunner) {
	t.Helper()
	llm := &captureStubLLM{chunks: []llmgateway.Chunk{textChunk("ok"), doneChunk()}}
	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: &stubContext{}, Tools: &stubTools{}, Persist: &stubPersist{}, MaxTurns: 4,
	})
	return llm, NewSubTurnRunner(orch, SubTurnConfig{DefaultMode: "brief", MaxDepth: 3})
}

// --- T14: mode dispatch ---

// TestSubTurnRunner_BriefMode_PreloadedMessagesNil (D7-S2-A06-T14) —
// mode=brief should drop parent history: LLM sees only the last user message,
// no preloaded assistant / tool_result messages.
func TestSubTurnRunner_BriefMode_PreloadedMessagesNil(t *testing.T) {
	llm, runner := buildSubTurnFixture(t)
	parent := []types.Message{
		{Role: types.MessageRoleAssistant, Content: "old assistant turn"},
		{Role: types.MessageRoleUser, Content: "old user turn", Metadata: map[string]string{"tool_results": `[{"call_id":"t1","content":"old tool"}]`}},
		{Role: types.MessageRoleUser, Content: "fresh directive"},
	}
	_, err := runner.RunSubTurn(context.Background(), contracts.SubTurnRequest{
		SessionID: "s",
		Messages:  parent,
		Mode:      contracts.SubAgentModeBrief,
	})
	if err != nil {
		t.Fatalf("RunSubTurn: %v", err)
	}
	got := llm.lastMessages()
	if len(got) != 1 {
		t.Fatalf("brief: expected 1 message (last user only), got %d: %+v", len(got), got)
	}
	if got[0].Role != types.MessageRoleUser || got[0].Content != "fresh directive" {
		t.Fatalf("brief: expected fresh directive user message, got %+v", got[0])
	}
}

// TestSubTurnRunner_ForkMode_DispatchesAsFork (D7-S2-A06-T14 variant) —
// mode=fork should emit BuildForkedMessages output (2 messages: cloned
// assistant + directive user with placeholder tool_results), preserving
// cache-friendly prefix stability.
func TestSubTurnRunner_ForkMode_DispatchesAsFork(t *testing.T) {
	llm, runner := buildSubTurnFixture(t)
	parent := []types.Message{
		{Role: types.MessageRoleUser, Content: "old user"},
		{Role: types.MessageRoleAssistant, Content: "old asst", Metadata: map[string]string{"tool_calls": `[{"id":"t1","type":"function","function":{"name":"grep","arguments":"{}"}}]`}},
		{Role: types.MessageRoleUser, Content: "old tool result", Metadata: map[string]string{"tool_results": `[{"call_id":"t1","content":"match"}]`}},
		{Role: types.MessageRoleUser, Content: "fork directive"},
	}
	_, err := runner.RunSubTurn(context.Background(), contracts.SubTurnRequest{
		SessionID: "s",
		Messages:  parent,
		Mode:      contracts.SubAgentModeFork,
	})
	if err != nil {
		t.Fatalf("RunSubTurn: %v", err)
	}
	got := llm.lastMessages()
	if len(got) != 3 {
		t.Fatalf("fork: expected 3 messages (cloned assistant + directive user + last user), got %d: %+v", len(got), got)
	}
	if got[0].Role != types.MessageRoleAssistant {
		t.Fatalf("fork: expected first message to be assistant (cloned), got %s", got[0].Role)
	}
	if got[1].Role != types.MessageRoleUser {
		t.Fatalf("fork: expected second message to be user (directive), got %s", got[1].Role)
	}
	if !strings.Contains(got[1].Content, "fork directive") {
		t.Fatalf("fork: expected directive 'fork directive' in user message, got %q", got[1].Content)
	}
	if !strings.Contains(got[1].Content, "Fork started — processing in background") {
		t.Fatalf("fork: expected ForkPlaceholderResult in user message, got %q", got[1].Content)
	}
	if got[2].Role != types.MessageRoleUser || got[2].Content != "fork directive" {
		t.Fatalf("fork: expected last user message to be the original directive, got %+v", got[2])
	}
}

// TestSubTurnRunner_FullMode_BackwardCompat (D7-S2-A06-T15) —
// mode=full should reproduce legacy behavior: PreloadedMessages = parent
// messages minus the last user message.
func TestSubTurnRunner_FullMode_BackwardCompat(t *testing.T) {
	llm, runner := buildSubTurnFixture(t)
	parent := []types.Message{
		{Role: types.MessageRoleAssistant, Content: "old asst"},
		{Role: types.MessageRoleUser, Content: "old user", Metadata: map[string]string{"tool_results": `[{"call_id":"t1","content":"tr"}]`}},
		{Role: types.MessageRoleUser, Content: "directive"},
	}
	_, err := runner.RunSubTurn(context.Background(), contracts.SubTurnRequest{
		SessionID: "s",
		Messages:  parent,
		Mode:      contracts.SubAgentModeFull,
	})
	if err != nil {
		t.Fatalf("RunSubTurn: %v", err)
	}
	got := llm.lastMessages()
	if len(got) != 3 {
		t.Fatalf("full: expected 3 messages (2 preloaded + last user), got %d: %+v", len(got), got)
	}
	if got[0].Role != types.MessageRoleAssistant {
		t.Fatalf("full: expected first preloaded to be assistant, got %s", got[0].Role)
	}
	if got[1].Role != types.MessageRoleUser {
		t.Fatalf("full: expected second preloaded to be user (tool_result), got %s", got[1].Role)
	}
	if got[2].Content != "directive" {
		t.Fatalf("full: expected last user message to be directive, got %q", got[2].Content)
	}
}

// TestSubTurnRunner_FullMode_EquivalentToLegacy (D2-S15-A08-T07) —
// AC8 invariant: mode=full is byte-equivalent to the pre-Phase-B legacy
// behavior (PreloadedMessages = messagesWithoutLastUser). This guards
// against accidental drift in the dispatch when refactoring applyMode.
//
// We construct the expected PreloadedMessages + UserMessage from the
// legacy helpers directly and assert that the LLM saw exactly those
// messages (in the same order, with the same content).
func TestSubTurnRunner_FullMode_EquivalentToLegacy(t *testing.T) {
	llm, runner := buildSubTurnFixture(t)
	parent := []types.Message{
		{Role: types.MessageRoleSystem, Content: "you are devrix"},
		{Role: types.MessageRoleUser, Content: "first user"},
		{Role: types.MessageRoleAssistant, Content: "first asst", Metadata: map[string]string{"k": "v"}},
		{Role: types.MessageRoleUser, Content: "second user", Metadata: map[string]string{"tool_results": `[{"call_id":"t1","content":"tr"}]`}},
		{Role: types.MessageRoleAssistant, Content: "second asst"},
		{Role: types.MessageRoleUser, Content: "directive"},
	}
	_, err := runner.RunSubTurn(context.Background(), contracts.SubTurnRequest{
		SessionID: "s",
		Messages:  parent,
		Mode:      contracts.SubAgentModeFull,
	})
	if err != nil {
		t.Fatalf("RunSubTurn: %v", err)
	}
	got := llm.lastMessages()

	// Build expected (legacy) using the same helper the legacy code path used.
	expected := append([]types.Message(nil), messagesWithoutLastUser(parent)...)
	lastUser := lastUserMessage(parent)
	expected = append(expected, lastUser)

	if len(got) != len(expected) {
		t.Fatalf("full ≡ legacy: length mismatch got=%d expected=%d", len(got), len(expected))
	}
	for i := range got {
		if got[i].Role != expected[i].Role {
			t.Errorf("full ≡ legacy: msg[%d] role got=%s want=%s", i, got[i].Role, expected[i].Role)
		}
		if got[i].Content != expected[i].Content {
			t.Errorf("full ≡ legacy: msg[%d] content got=%q want=%q", i, got[i].Content, expected[i].Content)
		}
	}
}

// TestSubTurnRunner_FullMode_EmptyParent (D2-S15-A08-T07 boundary) —
// when parent has no pre-last-user content, mode=full should send only
// the last user message (no error, no nil panic).
func TestSubTurnRunner_FullMode_EmptyParent(t *testing.T) {
	llm, runner := buildSubTurnFixture(t)
	_, err := runner.RunSubTurn(context.Background(), contracts.SubTurnRequest{
		SessionID: "s",
		Messages: []types.Message{
			{Role: types.MessageRoleUser, Content: "only directive"},
		},
		Mode: contracts.SubAgentModeFull,
	})
	if err != nil {
		t.Fatalf("RunSubTurn: %v", err)
	}
	got := llm.lastMessages()
	if len(got) != 1 || got[0].Content != "only directive" {
		t.Fatalf("full empty parent: expected single directive, got %+v", got)
	}
}

// --- T17: default mode from config ---

// TestSubTurnRunner_DefaultModeFromConfig (D7-S2-A06-T17) — when req.Mode
// is empty, SubTurnRunner resolves via Cfg.DefaultMode (brief by default).
func TestSubTurnRunner_DefaultModeFromConfig(t *testing.T) {
	llm := &captureStubLLM{chunks: []llmgateway.Chunk{textChunk("ok"), doneChunk()}}
	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: &stubContext{}, Tools: &stubTools{}, Persist: &stubPersist{}, MaxTurns: 4,
	})
	// Legacy override: legacy_mode=full → resolver should pick "full"
	// even though default_mode is "brief".
	runner := NewSubTurnRunner(orch, SubTurnConfig{DefaultMode: "brief", LegacyMode: "full", MaxDepth: 3})

	_, err := runner.RunSubTurn(context.Background(), contracts.SubTurnRequest{
		SessionID: "s",
		Messages: []types.Message{
			{Role: types.MessageRoleAssistant, Content: "old asst"},
			{Role: types.MessageRoleUser, Content: "directive"},
		},
		// Mode is empty → use resolved default
	})
	if err != nil {
		t.Fatalf("RunSubTurn: %v", err)
	}
	got := llm.lastMessages()
	if len(got) != 2 {
		t.Fatalf("default-from-config (legacy=full): expected 2 messages, got %d: %+v", len(got), got)
	}
	if got[0].Role != types.MessageRoleAssistant {
		t.Fatalf("default-from-config: expected first message to be assistant (full mode), got %s", got[0].Role)
	}
}

// TestSubTurnRunner_DefaultModeBrief (D7-S2-A06-T17 — empty config
// branch) — when neither req.Mode nor LegacyMode is set, default is "brief".
func TestSubTurnRunner_DefaultModeBrief(t *testing.T) {
	llm, runner := buildSubTurnFixture(t) // config: DefaultMode=brief, no legacy
	_, err := runner.RunSubTurn(context.Background(), contracts.SubTurnRequest{
		SessionID: "s",
		Messages: []types.Message{
			{Role: types.MessageRoleAssistant, Content: "old"},
			{Role: types.MessageRoleUser, Content: "directive"},
		},
	})
	if err != nil {
		t.Fatalf("RunSubTurn: %v", err)
	}
	got := llm.lastMessages()
	if len(got) != 1 || got[0].Content != "directive" {
		t.Fatalf("default=brief: expected single directive message, got %+v", got)
	}
}

// TestSubTurnRunner_InvalidModeRejected (D7-S2-A06-T14-T17 boundary) —
// unknown mode value returns ErrSubagentInvalidMode before any LLM call.
func TestSubTurnRunner_InvalidModeRejected(t *testing.T) {
	llm := &captureStubLLM{chunks: []llmgateway.Chunk{textChunk("ok"), doneChunk()}}
	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: &stubContext{}, Tools: &stubTools{}, Persist: &stubPersist{}, MaxTurns: 4,
	})
	runner := NewSubTurnRunner(orch, SubTurnConfig{DefaultMode: "brief", MaxDepth: 3})

	_, err := runner.RunSubTurn(context.Background(), contracts.SubTurnRequest{
		SessionID: "s",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "x"}},
		Mode:      contracts.SubAgentMode("unknown"),
	})
	if err == nil {
		t.Fatalf("expected error for invalid mode, got nil")
	}
	if !errors.Is(err, derrors.ErrSubagentInvalidMode) {
		t.Fatalf("expected ErrSubagentInvalidMode, got %v", err)
	}
	if llm.mu.Load() != 0 {
		t.Fatalf("LLM should not have been called for invalid mode, calls=%d", llm.mu.Load())
	}
}

// --- T16: depth limit ---

// TestSubTurnRunner_DepthLimit_Equals (D7-S2-A06-T16) — depth == MaxDepth
// is rejected with ErrSubagentDepthExceeded (>= boundary).
func TestSubTurnRunner_DepthLimit_Equals(t *testing.T) {
	llm, runner := buildSubTurnFixture(t) // MaxDepth=3
	_, err := runner.RunSubTurn(context.Background(), contracts.SubTurnRequest{
		SessionID: "s",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "x"}},
		Depth:     3,
	})
	if err == nil {
		t.Fatalf("expected depth-exceeded error, got nil")
	}
	if !errors.Is(err, derrors.ErrSubagentDepthExceeded) {
		t.Fatalf("expected ErrSubagentDepthExceeded, got %v", err)
	}
	if llm.mu.Load() != 0 {
		t.Fatalf("LLM should not have been called for depth-exceeded, calls=%d", llm.mu.Load())
	}
}

// TestSubTurnRunner_DepthLimit_Exceeds (D7-S2-A06-T16) — depth > MaxDepth
// is rejected.
func TestSubTurnRunner_DepthLimit_Exceeds(t *testing.T) {
	llm, runner := buildSubTurnFixture(t) // MaxDepth=3
	_, err := runner.RunSubTurn(context.Background(), contracts.SubTurnRequest{
		SessionID: "s",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "x"}},
		Depth:     10,
	})
	if err == nil {
		t.Fatalf("expected depth-exceeded error, got nil")
	}
	if !errors.Is(err, derrors.ErrSubagentDepthExceeded) {
		t.Fatalf("expected ErrSubagentDepthExceeded, got %v", err)
	}
	if llm.mu.Load() != 0 {
		t.Fatalf("LLM should not have been called for depth-exceeded, calls=%d", llm.mu.Load())
	}
}

// TestSubTurnRunner_DepthLimit_BoundaryAtMaxMinus1 (D7-S2-A06-T16) —
// depth == MaxDepth-1 is allowed (boundary case).
func TestSubTurnRunner_DepthLimit_BoundaryAtMaxMinus1(t *testing.T) {
	llm, runner := buildSubTurnFixture(t) // MaxDepth=3
	_, err := runner.RunSubTurn(context.Background(), contracts.SubTurnRequest{
		SessionID: "s",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "x"}},
		Depth:     2, // 3-1 = 2 → allowed
	})
	if err != nil {
		t.Fatalf("depth=MaxDepth-1 should be allowed, got: %v", err)
	}
	if llm.mu.Load() == 0 {
		t.Fatalf("LLM should have been called for depth=MaxDepth-1, calls=%d", llm.mu.Load())
	}
}

// captureTurnReqStub records the TurnRequest that the orchestrator received,
// used by Phase C AC1 to verify SubTurnRunner propagates MaxContextTokens.
type captureTurnReqStub struct {
	lastReq atomic.Pointer[TurnRequest]
	// delegate to a real orchestrator so RunSubTurn actually completes
	inner *DefaultOrchestrator
}

func (c *captureTurnReqStub) RunTurn(ctx context.Context, req TurnRequest) (<-chan *contracts.EngineEvent, error) {
	c.lastReq.Store(&req)
	return c.inner.RunTurn(ctx, req)
}

// TestSubTurnRunner_MaxContextTokens_Propagated (D7-S2-A06-T21+T22) — Phase C
// AC1. SubTurnRequest.MaxContextTokens flows into TurnRequest.MaxContextTokens.
// When the inbound request omits it, the SubTurnRunner falls back to
// Cfg.MaxContextTokens (wired by bootstrap from wire_coordinator.go).
func TestSubTurnRunner_MaxContextTokens_Propagated_DM_20260620_002(t *testing.T) {
	llm := &stubLLM{chunks: []llmgateway.Chunk{textChunk("ok"), doneChunk()}}
	inner := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: &stubContext{}, Tools: &stubTools{}, Persist: &stubPersist{}, MaxTurns: 2,
	})
	capture := &captureTurnReqStub{inner: inner}

	// Case 1: explicit SubTurnRequest.MaxContextTokens wins.
	runner := NewSubTurnRunner(capture, SubTurnConfig{
		DefaultMode:      "brief",
		MaxContextTokens: 10000, // fallback value, must NOT win when request sets one
	})
	if _, err := runner.RunSubTurn(context.Background(), contracts.SubTurnRequest{
		SessionID:         "s",
		SystemPrompt:      "p",
		Messages:          []types.Message{{Role: types.MessageRoleUser, Content: "x"}},
		Mode:              contracts.SubAgentModeBrief,
		MaxContextTokens:  64000,
	}); err != nil {
		t.Fatalf("RunSubTurn: %v", err)
	}
	if got := capture.lastReq.Load(); got == nil || got.MaxContextTokens != 64000 {
		t.Errorf("expected TurnRequest.MaxContextTokens=64000, got %+v", got)
	}

	// Case 2: SubTurnRequest.MaxContextTokens=0 → fallback to Cfg.
	capture.lastReq.Store(nil)
	runner2 := NewSubTurnRunner(capture, SubTurnConfig{
		DefaultMode:      "brief",
		MaxContextTokens: 42000,
	})
	if _, err := runner2.RunSubTurn(context.Background(), contracts.SubTurnRequest{
		SessionID: "s",
		SystemPrompt: "p",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "x"}},
		Mode:      contracts.SubAgentModeBrief,
		// MaxContextTokens omitted (=0)
	}); err != nil {
		t.Fatalf("RunSubTurn (fallback): %v", err)
	}
	if got := capture.lastReq.Load(); got == nil || got.MaxContextTokens != 42000 {
		t.Errorf("expected fallback TurnRequest.MaxContextTokens=42000, got %+v", got)
	}

	// Case 3: both zero → orchestrator-level fallback path (no assertion
	// on TurnRequest value, just ensure no panic).
	capture.lastReq.Store(nil)
	runner3 := NewSubTurnRunner(capture, SubTurnConfig{DefaultMode: "brief"})
	if _, err := runner3.RunSubTurn(context.Background(), contracts.SubTurnRequest{
		SessionID: "s",
		SystemPrompt: "p",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "x"}},
		Mode:      contracts.SubAgentModeBrief,
	}); err != nil {
		t.Fatalf("RunSubTurn (zero/zero): %v", err)
	}
	case3Req := capture.lastReq.Load()
	if case3Req == nil {
		t.Fatal("expected TurnRequest captured")
	}
	if case3Req.MaxContextTokens != 0 {
		t.Errorf("expected TurnRequest.MaxContextTokens=0 when both unset, got %d", case3Req.MaxContextTokens)
	}
}
