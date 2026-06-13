package wave

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestContextResolver_Fresh(t *testing.T) {
	// ORCH-S2-T12: fresh policy => Messages only contain directive.
	store := NewArtifactStore()
	rec := &stubSidechain{}
	r := NewContextResolver(ContextResolverDeps{
		Artifacts:        store,
		Sidechain:        rec,
		BaseSystemPrompt: "base",
	})

	got, err := r.Resolve(TaskNode{
		ID:                "t1",
		WorkerType:        WorkerSubAgent,
		ContextPolicy:     ContextFresh,
		Directive:         "do X",
		SystemPromptExtra: "be terse",
		FileScope:         []string{"src/api/**"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Policy != ContextFresh {
		t.Fatalf("expected fresh policy, got %q", got.Policy)
	}
	// Messages: only the user directive.
	if len(got.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got.Messages))
	}
	if got.Messages[0].Role != "user" || got.Messages[0].Content != "do X" {
		t.Fatalf("expected user directive only, got %+v", got.Messages[0])
	}
	// SystemPrompt: base + extra + file scope.
	if !contains(got.SystemPrompt, "base") || !contains(got.SystemPrompt, "be terse") {
		t.Fatalf("expected system prompt to include base and extra, got %q", got.SystemPrompt)
	}
	if !contains(got.SystemPrompt, "src/api/**") {
		t.Fatalf("expected system prompt to include file scope hint")
	}
}

func TestContextResolver_Upstream(t *testing.T) {
	// ORCH-S2-T11: upstream policy => consume artifact, no Leader history.
	store := NewArtifactStore()
	store.Put(Artifact{
		TaskID:       "upstream-1",
		Summary:      "did stuff",
		FilesChanged: []string{"src/api/users.go"},
	})
	r := NewContextResolver(ContextResolverDeps{
		Artifacts:        store,
		BaseSystemPrompt: "base",
	})

	got, err := r.Resolve(TaskNode{
		ID:             "downstream",
		WorkerType:     WorkerSubAgent,
		ContextPolicy:  ContextUpstream,
		UpstreamTaskID: "upstream-1",
		Directive:      "extend X",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UpstreamSummary != "did stuff" {
		t.Fatalf("expected upstream summary, got %q", got.UpstreamSummary)
	}
	// Only directive in messages (no Leader history).
	if len(got.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got.Messages))
	}
	if got.Messages[0].Content != "extend X" {
		t.Fatalf("expected directive only, got %q", got.Messages[0].Content)
	}
	if !contains(got.SystemPrompt, "did stuff") {
		t.Fatalf("expected system prompt to include upstream summary")
	}
}

func TestContextResolver_UpstreamMissingArtifact(t *testing.T) {
	store := NewArtifactStore()
	r := NewContextResolver(ContextResolverDeps{Artifacts: store})

	_, err := r.Resolve(TaskNode{
		ID:             "down",
		WorkerType:     WorkerSubAgent,
		ContextPolicy:  ContextUpstream,
		UpstreamTaskID: "missing",
		Directive:      "x",
	})
	if err == nil {
		t.Fatal("expected error for missing upstream artifact")
	}
}

func TestContextResolver_Resume(t *testing.T) {
	rec := &stubSidechain{
		msgs: []types.Message{
			{Role: "user", Content: "earlier turn"},
			{Role: "assistant", Content: "earlier reply"},
		},
	}
	r := NewContextResolver(ContextResolverDeps{
		Sidechain:        rec,
		BaseSystemPrompt: "base",
	})

	got, err := r.Resolve(TaskNode{
		ID:               "t-resume",
		WorkerType:       WorkerSubAgent,
		ContextPolicy:    ContextResume,
		SidechainAgentID: "agent-A",
		Directive:        "next",
		ParentSessionID:  "sess-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ResumeAgentID != "agent-A" {
		t.Fatalf("expected resume agent id 'agent-A', got %q", got.ResumeAgentID)
	}
	if len(got.Messages) != 3 {
		t.Fatalf("expected 3 messages (2 history + 1 directive), got %d", len(got.Messages))
	}
	if got.Messages[2].Content != "next" {
		t.Fatalf("expected directive at end, got %q", got.Messages[2].Content)
	}
}

func TestContextResolver_ResumeSidechainMissing(t *testing.T) {
	rec := &stubSidechain{errLoad: errStub("not found")}
	r := NewContextResolver(ContextResolverDeps{Sidechain: rec, BaseSystemPrompt: "base"})

	_, err := r.Resolve(TaskNode{
		ID:               "t",
		WorkerType:       WorkerSubAgent,
		ContextPolicy:    ContextResume,
		SidechainAgentID: "agent-A",
		Directive:        "x",
		ParentSessionID:  "sess-1",
	})
	if err == nil {
		t.Fatal("expected error when sidechain load fails for resume")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

type stubSidechain struct {
	msgs    []types.Message
	errLoad error
}

func (s *stubSidechain) Append(sessionID, agentID string, msg types.Message) error {
	return nil
}
func (s *stubSidechain) Load(sessionID, agentID string) ([]types.Message, error) {
	if s.errLoad != nil {
		return nil, s.errLoad
	}
	return s.msgs, nil
}

type stubErr string

func (e stubErr) Error() string { return string(e) }

func errStub(s string) error { return stubErr(s) }
