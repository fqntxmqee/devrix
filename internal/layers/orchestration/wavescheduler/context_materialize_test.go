package wavescheduler

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/materialize"
)

func TestMaterializingContextResolver_FreshParity(t *testing.T) {
	store := NewArtifactStore()
	r := NewMaterializingContextResolver(ContextResolverDeps{
		Artifacts:        store,
		Materializer:     materialize.NewDefaultMaterializer(nil),
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
		t.Fatal(err)
	}
	if len(got.Messages) != 1 || got.Messages[0].Content != "do X" {
		t.Fatalf("messages = %+v", got.Messages)
	}
}

func TestMaterializingContextResolver_Upstream(t *testing.T) {
	artifacts := NewArtifactStore()
	artifacts.Put(Artifact{
		TaskID:       "upstream-1",
		Summary:      "did stuff",
		FilesChanged: []string{"src/api/users.go"},
	})
	r := NewMaterializingContextResolver(ContextResolverDeps{
		Artifacts:        artifacts,
		Materializer:     materialize.NewDefaultMaterializer(nil),
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
		t.Fatal(err)
	}
	if got.UpstreamSummary != "did stuff" {
		t.Fatalf("summary = %q", got.UpstreamSummary)
	}
	if len(got.Messages) != 1 || got.Messages[0].Content != "extend X" {
		t.Fatalf("messages = %+v", got.Messages)
	}
}

func TestNewMaterializingContextResolver_FallsBackWithoutMaterializer(t *testing.T) {
	r := NewMaterializingContextResolver(ContextResolverDeps{
		Artifacts:        NewArtifactStore(),
		BaseSystemPrompt: "base",
	})
	if _, ok := r.(*ContextResolver); !ok {
		t.Fatalf("expected legacy ContextResolver fallback, got %T", r)
	}
}
