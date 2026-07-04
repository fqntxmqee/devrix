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
	m := NewDefaultMaterializer(store, dir)
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
	if !strings.Contains(res.SystemPrompt, "Do not label observations as Obs") &&
		!strings.Contains(res.SystemPrompt, "不要自行标注 ObsFact") {
		t.Fatal("system prompt must forbid Execute-side Obs* self-labeling")
	}
	if len(res.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(res.Messages))
	}
}

func TestDefaultMaterializer_Materialize_repairs_orphan_tool_results_after_compress(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewPartitionStore(dir)
	calls := `[{"id":"call_a","type":"function","function":{"name":"read_file","arguments":"{}"}},{"id":"call_b","type":"function","function":{"name":"grep","arguments":"{}"}}]`
	priv := []types.Message{
		{SessionID: "s1", Role: types.MessageRoleAssistant, Metadata: map[string]string{"tool_calls": calls}},
		{SessionID: "s1", Role: types.MessageRoleTool, Content: "a", Metadata: map[string]string{"tool_call_id": "call_a"}},
		{SessionID: "s1", Role: types.MessageRoleTool, Content: "orphan", Metadata: map[string]string{"tool_call_id": "call_stale"}},
	}
	if err := store.Append("s1", "wi1", priv); err != nil {
		t.Fatalf("Append: %v", err)
	}
	m := NewDefaultMaterializer(store, dir)
	res, err := m.Materialize(context.Background(), Request{
		Partition: Partition{SessionID: "s1", Kind: PartitionWorkItem, WorkItemID: "wi1"},
		Policy:    Policy{Mode: ModeFresh, TokenBudget: 100000},
		Signals:   InboundSignals{Directive: "work"},
	})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	toolCount := 0
	for _, msg := range res.Messages {
		if msg.Role == types.MessageRoleTool {
			toolCount++
			if msg.Metadata["tool_call_id"] == "call_stale" {
				t.Fatal("orphan tool result should be repaired out")
			}
		}
	}
	if toolCount != 0 {
		t.Fatalf("tool messages = %d, want 0 (incomplete multi-call round stripped)", toolCount)
	}
}

func TestMergeInitialWithPrivateChain_skips_duplicate_opening_user(t *testing.T) {
	directive := "do work"
	initial := []types.Message{{Role: types.MessageRoleUser, Content: directive}}
	priv := []types.Message{
		{Role: types.MessageRoleUser, Content: directive},
		{Role: types.MessageRoleAssistant, Content: "ok"},
	}
	got := mergeInitialWithPrivateChain(initial, priv)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Role != types.MessageRoleUser || got[1].Role != types.MessageRoleAssistant {
		t.Fatalf("got %+v", got)
	}
}
