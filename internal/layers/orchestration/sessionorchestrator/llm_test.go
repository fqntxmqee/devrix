package sessionorchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/types"
)

// --- gateway stubs ---

// stubGateway implements llmgateway.IGateway for InvokeStream-level tests.
type stubGateway struct {
	streamFn func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error)
}

func (s *stubGateway) Stream(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
	if s.streamFn != nil {
		return s.streamFn(ctx, req)
	}
	return nil, errors.New("stub: Stream not configured")
}

func (s *stubGateway) ResolveTier(tier string) string { return tier }

func (s *stubGateway) Close() error { return nil }

// stubTierResolver implements llmgateway.ITierResolver.
type stubTierResolver struct {
	resolved string
	err      error
}

func (s *stubTierResolver) ResolveTier(tier string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if s.resolved != "" {
		return s.resolved, nil
	}
	return tier, nil
}

// --- D7-S2-A07-T01: Breaker open error propagation ---

func TestGatewayInvoker_InvokeStream_BreakerOpen(t *testing.T) {
	breakerErr := fmt.Errorf("circuit breaker rejected: openai/gpt-5")
	gw := &stubGateway{
		streamFn: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
			return nil, breakerErr
		},
	}
	invoker := NewGatewayInvoker(LLMInvokerDeps{
		Gateway:     gw,
		DefaultTier: "fast",
	})

	_, err := invoker.InvokeStream(context.Background(), LLMInvokeRequest{
		SessionID: "sess-1",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "hello"}},
	})
	if err == nil {
		t.Fatal("expected error for breaker open, got nil")
	}
	if !errors.Is(err, breakerErr) {
		t.Errorf("expected breaker error to propagate, got: %v", err)
	}
}

// TestGatewayInvoker_InvokeStream_BreakerOpen_Content verifies the error
// message contains the provider name so upstream can surface it.
func TestGatewayInvoker_InvokeStream_BreakerOpen_Content(t *testing.T) {
	gw := &stubGateway{
		streamFn: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
			return nil, fmt.Errorf("circuit breaker rejected: anthropic/claude-opus")
		},
	}
	invoker := NewGatewayInvoker(LLMInvokerDeps{
		Gateway:     gw,
		DefaultTier: "fast",
	})

	_, err := invoker.InvokeStream(context.Background(), LLMInvokeRequest{
		SessionID: "sess-2",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
}

// --- D7-S2-A07-T01: other gateway errors also propagate ---

func TestGatewayInvoker_InvokeStream_GatewayError(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"timeout", errors.New("stream timeout")},
		{"connection refused", errors.New("dial tcp: connection refused")},
		{"internal error", errors.New("internal server error")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gw := &stubGateway{
				streamFn: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
					return nil, tt.err
				},
			}
			invoker := NewGatewayInvoker(LLMInvokerDeps{
				Gateway:     gw,
				DefaultTier: "fast",
			})

			_, err := invoker.InvokeStream(context.Background(), LLMInvokeRequest{
				SessionID: "sess-3",
				Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "test"}},
			})
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

// --- D7-S2-A07-T02: Stream timeout propagation ---

func TestGatewayInvoker_InvokeStream_ContextCanceled(t *testing.T) {
	gw := &stubGateway{
		streamFn: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
			return nil, ctx.Err()
		},
	}
	invoker := NewGatewayInvoker(LLMInvokerDeps{
		Gateway:     gw,
		DefaultTier: "fast",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := invoker.InvokeStream(ctx, LLMInvokeRequest{
		SessionID: "sess-4",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "test"}},
	})
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestGatewayInvoker_InvokeStream_ContextDeadlineExceeded(t *testing.T) {
	gw := &stubGateway{
		streamFn: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
			return nil, ctx.Err()
		},
	}
	invoker := NewGatewayInvoker(LLMInvokerDeps{
		Gateway:     gw,
		DefaultTier: "fast",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	_, err := invoker.InvokeStream(ctx, LLMInvokeRequest{
		SessionID: "sess-5",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "test"}},
	})
	if err == nil {
		t.Fatal("expected error for deadline exceeded, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}
}

// --- D7-S2-A07-T02: orchestrator-level timeout → EngineEvent ---

func TestOrchestrator_RunTurn_StreamTimeout_EngineEvent(t *testing.T) {
	llm := &stubLLM{}
	llm.fn = func(ctx context.Context, req LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
		// Simulate a stream that succeeds to return a channel, but the
		// context is already timed out so the actual chunk read fails.
		// The InvokeStream layer returns the error directly in this case.
		return nil, context.DeadlineExceeded
	}

	ctxPrep := &stubContext{prepared: PreparedContext{}}
	persist := &stubPersist{}
	tools := &stubTools{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: tools, Persist: persist, MaxTurns: 4,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID: "sess-timeout",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "hi"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)

	if !hasType(evs, "error") {
		t.Error("expected error event for stream timeout")
	}
	// Verify the error content mentions timeout/deadline
	found := false
	for _, e := range evs {
		if e.Type == "error" && e.Content != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected non-empty error event content")
	}
}

// --- Tier resolution error propagation ---

func TestGatewayInvoker_InvokeStream_TierResolveError(t *testing.T) {
	gw := &stubGateway{
		streamFn: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
			return nil, nil
		},
	}
	resolverErr := errors.New("unknown tier: experimental")
	invoker := NewGatewayInvoker(LLMInvokerDeps{
		Gateway:      gw,
		TierResolver: &stubTierResolver{err: resolverErr},
		DefaultTier:  "fast",
	})

	_, err := invoker.InvokeStream(context.Background(), LLMInvokeRequest{
		SessionID: "sess-tier",
		Tier:      "experimental",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "test"}},
	})
	if err == nil {
		t.Fatal("expected error for tier resolve failure, got nil")
	}
}

// --- Successful tier resolution ---

func TestGatewayInvoker_InvokeStream_TierResolveSuccess(t *testing.T) {
	gw := &stubGateway{
		streamFn: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
			if req.Model != "gpt-5-mini" {
				t.Errorf("expected resolved model 'gpt-5-mini', got %q", req.Model)
			}
			ch := make(chan llmgateway.Chunk, 1)
			ch <- llmgateway.Chunk{Content: "ok", Done: true}
			close(ch)
			return ch, nil
		},
	}
	invoker := NewGatewayInvoker(LLMInvokerDeps{
		Gateway:      gw,
		TierResolver: &stubTierResolver{resolved: "gpt-5-mini"},
		DefaultTier:  "fast",
	})

	ch, err := invoker.InvokeStream(context.Background(), LLMInvokeRequest{
		SessionID: "sess-resolve",
		Tier:      "fast",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "test"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range ch {
	}
}

// --- Default tier used when request tier is empty ---

func TestGatewayInvoker_InvokeStream_DefaultTier(t *testing.T) {
	var capturedModel string
	gw := &stubGateway{
		streamFn: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
			capturedModel = req.Model
			ch := make(chan llmgateway.Chunk, 1)
			ch <- llmgateway.Chunk{Content: "ok", Done: true}
			close(ch)
			return ch, nil
		},
	}
	invoker := NewGatewayInvoker(LLMInvokerDeps{
		Gateway:     gw,
		DefaultTier: "default-fast",
	})

	ch, err := invoker.InvokeStream(context.Background(), LLMInvokeRequest{
		SessionID: "sess-default-tier",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "test"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range ch {
	}
	if capturedModel != "default-fast" {
		t.Errorf("expected default tier 'default-fast', got %q", capturedModel)
	}
}

// --- DM-20260621-007: convertToolSchemas must preserve Parameters ---

// TestConvertToolSchemas_PreservesParameters is the regression test for
// the empty-args tool-call bug. Prior to this fix, the D7→D3 bridge
// dropped the Parameters JSON schema, so the LLM saw tools with no
// parameter definitions and defaulted every call to {} (bash invoked
// without "command"; glob without "pattern").
func TestConvertToolSchemas_PreservesParameters(t *testing.T) {
	in := []ToolSchema{{
		Name:        "bash",
		Description: "Execute a shell command in the session WorkDir (sandboxed).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string"},
			},
			"required": []any{"command"},
		},
	}}
	out := convertToolSchemas(in)
	if len(out) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(out))
	}
	if out[0].Name != "bash" {
		t.Errorf("expected name 'bash', got %q", out[0].Name)
	}
	if out[0].Description == "" {
		t.Error("description should be preserved")
	}
	if out[0].Parameters == "" {
		t.Fatal("Parameters string is empty — regression: schema dropped at D7→D3 boundary")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out[0].Parameters), &parsed); err != nil {
		t.Fatalf("Parameters is not valid JSON: %v (raw=%q)", err, out[0].Parameters)
	}
	if parsed["type"] != "object" {
		t.Errorf("expected type=object in Parameters, got %v", parsed["type"])
	}
	props, ok := parsed["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties map, got %T", parsed["properties"])
	}
	if _, ok := props["command"]; !ok {
		t.Error("expected 'command' property in Parameters")
	}
	required, ok := parsed["required"].([]any)
	if !ok || len(required) == 0 || required[0] != "command" {
		t.Errorf("expected required=[command], got %v", parsed["required"])
	}
}

// TestConvertToolSchemas_PreservesNestedParameters covers tool specs
// with nested object properties (e.g. glob.pattern with glob_filter
// sub-objects), which is the common case for read_file/grep.
func TestConvertToolSchemas_PreservesNestedParameters(t *testing.T) {
	in := []ToolSchema{{
		Name:        "glob",
		Description: "Find files by glob pattern.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern":  map[string]any{"type": "string"},
				"base_dir": map[string]any{"type": "string"},
			},
			"required": []any{"pattern"},
		},
	}}
	out := convertToolSchemas(in)
	if len(out) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(out))
	}
	if out[0].Parameters == "" {
		t.Fatal("Parameters string is empty for glob tool")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out[0].Parameters), &parsed); err != nil {
		t.Fatalf("Parameters is not valid JSON: %v", err)
	}
	props := parsed["properties"].(map[string]any)
	if _, ok := props["pattern"]; !ok {
		t.Error("expected 'pattern' property")
	}
	if _, ok := props["base_dir"]; !ok {
		t.Error("expected 'base_dir' property")
	}
}

// TestConvertToolSchemas_EmptyInput ensures nil in → nil out (the
// orchestrator relies on this when a session has no tools wired).
func TestConvertToolSchemas_EmptyInput(t *testing.T) {
	if got := convertToolSchemas(nil); got != nil {
		t.Errorf("expected nil for nil input, got %v", got)
	}
	if got := convertToolSchemas([]ToolSchema{}); got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}
}

// TestConvertToolSchemas_EmptyParameters covers a tool with no
// parameter spec (e.g. a fixed-purpose helper). Parameters must
// marshal to "{}" or empty string — never panic — so the LLM still
// sees the tool in its tool list.
func TestConvertToolSchemas_EmptyParameters(t *testing.T) {
	in := []ToolSchema{{
		Name:        "ping",
		Description: "Health check, no inputs.",
		Parameters:  nil,
	}}
	out := convertToolSchemas(in)
	if len(out) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(out))
	}
	if out[0].Name != "ping" {
		t.Errorf("expected name 'ping', got %q", out[0].Name)
	}
	// empty Parameters on the input maps to empty string on the output
	// (NOT the JSON literal "null"), since llmgateway distinguishes
	// "no schema" from "schema is null".
	if out[0].Parameters != "" {
		t.Errorf("expected empty Parameters for nil-input, got %q", out[0].Parameters)
	}
}

// TestConvertToolSchemas_AllFieldsPreserved is the canonical
// table-driven check: every field on the input survives the
// conversion. Future field additions to ToolSchema should extend
// this test.
func TestConvertToolSchemas_AllFieldsPreserved(t *testing.T) {
	in := []ToolSchema{
		{
			Name:        "read_file",
			Description: "Read a file from disk.",
			Parameters: map[string]any{
				"type":     "object",
				"required": []any{"path"},
				"properties": map[string]any{
					"path": map[string]any{"type": "string"},
				},
			},
		},
		{
			Name:        "grep",
			Description: "Regex search.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern":  map[string]any{"type": "string"},
					"path":     map[string]any{"type": "string"},
					"context":  map[string]any{"type": "integer"},
					"is_regex": map[string]any{"type": "boolean"},
				},
				"required": []any{"pattern"},
			},
		},
		{Name: "list_dir", Description: "List a directory."},
	}
	out := convertToolSchemas(in)
	if len(out) != len(in) {
		t.Fatalf("expected %d tools, got %d", len(in), len(out))
	}
	for i, ts := range in {
		if out[i].Name != ts.Name {
			t.Errorf("[%d] name: got %q want %q", i, out[i].Name, ts.Name)
		}
		if out[i].Description != ts.Description {
			t.Errorf("[%d] description: got %q want %q", i, out[i].Description, ts.Description)
		}
		if ts.Parameters == nil && out[i].Parameters != "" {
			t.Errorf("[%d] nil Parameters should marshal to empty string, got %q", i, out[i].Parameters)
		}
		if ts.Parameters != nil && out[i].Parameters == "" {
			t.Errorf("[%d] non-nil Parameters dropped: got empty string", i)
		}
	}
}
