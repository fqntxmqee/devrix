package isolate_test

import (
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/multiagent/isolate"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D4-S3-A01-T04 (ForkSessionView API injection)
func TestFork_should_share_readonly_fields(t *testing.T) {
	parent := types.NewSession("sess_fork_view", "cli", "/tmp")
	parent.Model = "claude-opus-4-5"
	parent.ContextSnapshot = []byte("ctx-bytes")

	view := isolate.Fork(parent)
	if view == nil {
		t.Fatal("Fork returned nil")
	}
	if view.ID() != parent.SessionID {
		t.Errorf("ID() = %q, want %q", view.ID(), parent.SessionID)
	}
	if view.Model() != parent.Model {
		t.Errorf("Model() = %q, want %q", view.Model(), parent.Model)
	}
	if view.CreatedAt().IsZero() {
		t.Error("CreatedAt() is zero")
	}
	if view.Snapshot() == nil {
		t.Error("Snapshot() is nil")
	}
}

func TestFork_should_isolate_metadata_writes(t *testing.T) {
	parent := types.NewSession("sess_meta_iso", "cli", "/tmp")
	view := isolate.Fork(parent)

	view.SetMetadata("trace_id", "child-trace-1")
	view.SetMetadata("step", 1)

	if v, ok := view.GetMetadata("trace_id"); !ok || v != "child-trace-1" {
		t.Errorf("GetMetadata(trace_id) = %v, %v; want child-trace-1, true", v, ok)
	}
	if v, ok := view.GetMetadata("step"); !ok || v != 1 {
		t.Errorf("GetMetadata(step) = %v, %v; want 1, true", v, ok)
	}
	// Parent's metadata must not contain the child's writes.
	if _, ok := parent.Metadata["trace_id"]; ok {
		t.Error("parent metadata was polluted by child view SetMetadata")
	}
	if _, ok := parent.Metadata["step"]; ok {
		t.Error("parent metadata was polluted by child view SetMetadata")
	}
}

func TestView_GetMetadata_missing_key(t *testing.T) {
	parent := types.NewSession("sess_getmiss", "cli", "/tmp")
	view := isolate.Fork(parent)
	if _, ok := view.GetMetadata("absent"); ok {
		t.Fatal("expected ok=false for missing key")
	}
}

func TestMergeToParent_should_copy_metadata_and_snapshot(t *testing.T) {
	parent := types.NewSession("sess_merge", "cli", "/tmp")
	parent.ContextSnapshot = []byte("parent-ctx")
	view := isolate.Fork(parent)
	view.SetMetadata("artifact", "result-A")
	view.SetSnapshot([]byte("child-ctx"))

	if err := view.MergeToParent(parent); err != nil {
		t.Fatalf("MergeToParent: %v", err)
	}
	if v, ok := parent.Metadata["artifact"]; !ok || v != "result-A" {
		t.Errorf("parent.Metadata[artifact] = %v, %v; want result-A, true", v, ok)
	}
	if string(parent.ContextSnapshot) != "child-ctx" {
		t.Errorf("parent.ContextSnapshot = %q, want child-ctx", string(parent.ContextSnapshot))
	}
}

func TestMergeToParent_should_reject_nil_parent(t *testing.T) {
	parent := types.NewSession("sess_nil", "cli", "/tmp")
	view := isolate.Fork(parent)
	if err := view.MergeToParent(nil); err == nil {
		t.Fatal("expected error when parent is nil")
	}
}

// T: D4-S3-A01-T02 (metadata write isolation under concurrency)
func TestFork_concurrent_writes_should_not_race(t *testing.T) {
	parent := types.NewSession("sess_concurrent", "cli", "/tmp")
	view := isolate.Fork(parent)

	var wg sync.WaitGroup
	stop := time.Now().Add(50 * time.Millisecond)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			j := 0
			for time.Now().Before(stop) {
				view.SetMetadata("k", idx*1000+j)
				_, _ = view.GetMetadata("k")
				j++
			}
		}(i)
	}
	wg.Wait()
	// After concurrent writes, parent must not contain the child's k.
	if _, ok := parent.Metadata["k"]; ok {
		t.Error("parent metadata was polluted by concurrent child writes")
	}
}
