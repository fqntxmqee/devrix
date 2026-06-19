package decisionplanning

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/turn"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
)

// fakeLLMInvoker is a minimal turn.LLMInvoker that streams canned
// content back through the chunk channel. It supports multiple chunks
// to verify collectChunkContent concatenates correctly.
type fakeLLMInvoker struct {
	chunks []string
	calls  int
}

func (f *fakeLLMInvoker) InvokeStream(ctx context.Context, req turn.LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
	f.calls++
	out := make(chan llmgateway.Chunk, len(f.chunks)+1)
	go func() {
		defer close(out)
		for _, c := range f.chunks {
			select {
			case <-ctx.Done():
				return
			case out <- llmgateway.Chunk{Content: c}:
			}
		}
		out <- llmgateway.Chunk{Done: true}
	}()
	return out, nil
}

// T: D7-S5-A03-T01 — LLMDecomposer parses a well-formed JSON array
// from the streaming response and produces valid wavescheduler.TaskNodes.
func TestLLMDecomposer_Decompose_HappyPath(t *testing.T) {
	llm := &fakeLLMInvoker{chunks: []string{
		"Sure! Here's the plan:\n",
		"```json\n",
		`[{"id":"design_api","title":"Design REST API","directive":"Define endpoints and contracts","worker_type":"cursor","context_policy":"fresh","depends_on":[]},`,
		`{"id":"impl_backend","title":"Implement Go backend","directive":"Build the HTTP handlers","worker_type":"cursor","context_policy":"upstream","depends_on":["design_api"]},`,
		`{"id":"write_tests","title":"Write integration tests","directive":"Cover happy + error paths","worker_type":"claude_code","context_policy":"upstream","depends_on":["impl_backend"]}]`,
		"\n```",
	}}
	d := NewLLMDecomposer(LLMDecomposerDeps{LLM: llm, DefaultTier: "sonnet"})

	nodes, err := d.Decompose(context.Background(), "sess_test", "Build a user profile API")
	if err != nil {
		t.Fatalf("Decompose: unexpected error: %v", err)
	}
	if got, want := len(nodes), 3; got != want {
		t.Fatalf("got %d nodes, want %d", got, want)
	}

	// First task: no deps, fresh context, cursor worker.
	if nodes[0].ID != "design_api" || nodes[0].WorkerType != wavescheduler.WorkerCursor ||
		nodes[0].ContextPolicy != wavescheduler.ContextFresh || len(nodes[0].DependsOn) != 0 {
		t.Errorf("node[0] mismatch: %+v", nodes[0])
	}

	// Second task: depends_on design_api, upstream context.
	if got := nodes[1].DependsOn; len(got) != 1 || got[0] != "design_api" {
		t.Errorf("node[1].DependsOn = %v, want [design_api]", got)
	}
	if nodes[1].ContextPolicy != wavescheduler.ContextUpstream {
		t.Errorf("node[1].ContextPolicy = %v, want upstream", nodes[1].ContextPolicy)
	}

	// Third task: claude_code worker.
	if nodes[2].WorkerType != wavescheduler.WorkerClaudeCode {
		t.Errorf("node[2].WorkerType = %v, want claude_code", nodes[2].WorkerType)
	}

	if llm.calls != 1 {
		t.Errorf("LLMInvoker.InvokeStream calls = %d, want 1", llm.calls)
	}
}

// T: D7-S5-A03-T02 — when the LLM returns invalid JSON, Decompose
// returns an error so TaskDecomposer.SynthesizeTaskGraph can fall back
// to the rule-based path.
func TestLLMDecomposer_Decompose_BadJSON(t *testing.T) {
	llm := &fakeLLMInvoker{chunks: []string{"sorry, I can't help with that"}}
	d := NewLLMDecomposer(LLMDecomposerDeps{LLM: llm})

	_, err := d.Decompose(context.Background(), "sess_test", "anything")
	if err == nil {
		t.Fatal("expected error on non-JSON response, got nil")
	}
	if !strings.Contains(err.Error(), "no JSON payload") {
		t.Errorf("expected 'no JSON payload' error, got: %v", err)
	}
}

// T: D7-S5-A03-T03 — unknown worker_type / context_policy values are
// coerced to safe defaults instead of failing the parse.
func TestLLMDecomposer_Decompose_InvalidEnumsCoerced(t *testing.T) {
	llm := &fakeLLMInvoker{chunks: []string{
		`[{"id":"t1","directive":"do the thing","worker_type":"hologram","context_policy":"banana"}]`,
	}}
	d := NewLLMDecomposer(LLMDecomposerDeps{LLM: llm})

	nodes, err := d.Decompose(context.Background(), "sess_test", "goal")
	if err != nil {
		t.Fatalf("Decompose: unexpected error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(nodes))
	}
	if nodes[0].WorkerType != wavescheduler.WorkerCursor {
		t.Errorf("WorkerType = %v, want cursor (default)", nodes[0].WorkerType)
	}
	if nodes[0].ContextPolicy != wavescheduler.ContextFresh {
		t.Errorf("ContextPolicy = %v, want fresh (default)", nodes[0].ContextPolicy)
	}
}

// T: D7-S5-A03-T04 — DependsOn edges that reference unknown ids are
// dropped, so the downstream DAG validator never rejects the graph.
func TestLLMDecomposer_Decompose_DropsUnknownDeps(t *testing.T) {
	llm := &fakeLLMInvoker{chunks: []string{
		`[{"id":"a","directive":"a","depends_on":["ghost"]},{"id":"b","directive":"b","depends_on":["a"]}]`,
	}}
	d := NewLLMDecomposer(LLMDecomposerDeps{LLM: llm})

	nodes, err := d.Decompose(context.Background(), "sess_test", "goal")
	if err != nil {
		t.Fatalf("Decompose: unexpected error: %v", err)
	}
	if len(nodes[0].DependsOn) != 0 {
		t.Errorf("node[0].DependsOn = %v, want [] (ghost dropped)", nodes[0].DependsOn)
	}
	if len(nodes[1].DependsOn) != 1 || nodes[1].DependsOn[0] != "a" {
		t.Errorf("node[1].DependsOn = %v, want [a]", nodes[1].DependsOn)
	}
}

// T: D7-S5-A03-T05 — extractJSON helper handles code fences and prose
// around the JSON payload.
func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain array", `[{"id":"a"}]`, `[{"id":"a"}]`},
		{"with prose", `Here's the plan: [{"id":"a"}] — done!`, `[{"id":"a"}]`},
		{"with json fence", "```json\n[{\"id\":\"a\"}]\n```", `[{"id":"a"}]`},
		{"with bare fence", "```\n[{\"id\":\"a\"}]\n```", `[{"id":"a"}]`},
		{"no array", "no json here", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSON(tt.in)
			if got != tt.want {
				t.Errorf("extractJSON(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// T: D7-S5-A03-T06 — NewLLMDecomposer returns nil when deps.LLM is
// nil, so callers can defensively skip wiring without crashing.
func TestNewLLMDecomposer_NilLLM(t *testing.T) {
	if d := NewLLMDecomposer(LLMDecomposerDeps{}); d != nil {
		t.Errorf("NewLLMDecomposer({}) = %v, want nil", d)
	}
}

// T: D7-S5-A03-T07 — SynthesizeTaskGraph routes through LLMDecomposer
// when one is wired, and the LLM output becomes the task graph.
func TestTaskDecomposer_UsesLLMDecomposer(t *testing.T) {
	llm := &fakeLLMInvoker{chunks: []string{
		`[{"id":"x","directive":"step x","worker_type":"cursor","context_policy":"fresh","depends_on":[]}]`,
	}}
	decomp := NewTaskDecomposer()
	decomp.SetLLMDecomposer(NewLLMDecomposer(LLMDecomposerDeps{LLM: llm}))

	result, err := decomp.SynthesizeTaskGraph(context.Background(), "sess_x", "anything")
	if err != nil {
		t.Fatalf("SynthesizeTaskGraph: %v", err)
	}
	if len(result.Nodes) != 1 || result.Nodes[0].ID != "x" {
		t.Errorf("expected LLM-derived node 'x', got %+v", result.Nodes)
	}
	if got := result.Nodes[0].Metadata["source"]; got != "llm_decomposer" {
		t.Errorf("Metadata.source = %v, want llm_decomposer", got)
	}
}
