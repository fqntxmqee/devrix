package materialize

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestPartitionStore_AppendLoad(t *testing.T) {
	dir := t.TempDir()
	store, err := NewPartitionStore(dir)
	if err != nil {
		t.Fatalf("NewPartitionStore: %v", err)
	}
	msgs := []types.Message{{
		SessionID: "s1",
		Role:      types.MessageRoleUser,
		Content:   "hello",
	}}
	if err := store.Append("s1", "wi1", msgs); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, err := store.Load("s1", "wi1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 || got[0].Content != "hello" {
		t.Fatalf("got %+v", got)
	}
	path := filepath.Join(dir, "s1", "wi", "wi1.jsonl")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("jsonl missing: %v", err)
	}
}

func TestDefaultMaterializer_Materialize(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewPartitionStore(dir)
	m := NewDefaultMaterializer(store)
	res, err := m.Materialize(context.Background(), Request{
		Partition: Partition{SessionID: "s1", Kind: PartitionWorkItem, WorkItemID: "wi1"},
		Policy:    Policy{Mode: ModeFresh, TokenBudget: 1000},
		Signals: InboundSignals{
			Directive:      "implement feature",
			ScopeIn:        []string{"internal/foo.go"},
			ExpectedReturn: "patch",
		},
	})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if !strings.Contains(res.SystemPrompt, "internal/foo.go") {
		t.Fatalf("system prompt missing scope: %q", res.SystemPrompt)
	}
	if !strings.Contains(res.SystemPrompt, "Do not label observations as Obs") {
		t.Fatal("system prompt must forbid Execute-side Obs* self-labeling")
	}
	if len(res.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(res.Messages))
	}
}
