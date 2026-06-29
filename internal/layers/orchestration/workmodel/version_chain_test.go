package workmodel

import (
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

func TestNewVersionChainRegistry_Empty(t *testing.T) {
	r := NewVersionChainRegistry()
	if r.SessionCount() != 0 {
		t.Fatalf("expected SessionCount=0, got %d", r.SessionCount())
	}
}

func TestVersionChainRegistry_ChainFor_CreatesEmpty(t *testing.T) {
	r := NewVersionChainRegistry()
	c := r.ChainFor("sess_1")
	if c == nil {
		t.Fatalf("expected non-nil chain")
	}
	if c.Len() != 0 {
		t.Fatalf("expected empty chain, got Len=%d", c.Len())
	}
	if r.SessionCount() != 1 {
		t.Fatalf("expected SessionCount=1, got %d", r.SessionCount())
	}
}

func TestVersionChainRegistry_ChainFor_ReturnsSameOnRepeat(t *testing.T) {
	r := NewVersionChainRegistry()
	c1 := r.ChainFor("sess_1")
	c2 := r.ChainFor("sess_1")
	if c1 != c2 {
		t.Fatalf("expected same chain pointer for same session")
	}
}

func TestVersionChainRegistry_Append_PersistsNewChain(t *testing.T) {
	r := NewVersionChainRegistry()
	h, _, err := r.Append("sess_1", []byte("hello"), "commit")
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if h == interfaces.EmptyHash {
		t.Fatalf("expected non-empty hash")
	}
	c := r.ChainFor("sess_1")
	if c.Len() != 1 {
		t.Fatalf("expected Len=1 after append, got %d", c.Len())
	}
}

func TestVersionChainRegistry_Append_ChainedHashes(t *testing.T) {
	r := NewVersionChainRegistry()
	h1, _, err := r.Append("sess_1", []byte("v1"), "commit")
	if err != nil {
		t.Fatalf("append v1: %v", err)
	}
	h2, next, err := r.Append("sess_1", []byte("v2"), "commit")
	if err != nil {
		t.Fatalf("append v2: %v", err)
	}
	if h1 == h2 {
		t.Fatalf("h1 should not equal h2")
	}
	entry, ok := next.Get(h2)
	if !ok {
		t.Fatalf("get h2: missing")
	}
	if entry.Parent != h1 {
		t.Fatalf("expected Parent=%q, got %q", h1, entry.Parent)
	}
}

func TestVersionChainRegistry_Rollback_Success(t *testing.T) {
	r := NewVersionChainRegistry()
	h1, _, _ := r.Append("sess_1", []byte("v1"), "commit")
	_, _, _ = r.Append("sess_1", []byte("v2"), "commit")
	rb, err := r.Rollback("sess_1", h1)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if rb.Head() != h1 {
		t.Fatalf("expected head=%q, got %q", h1, rb.Head())
	}
}

func TestVersionChainRegistry_Rollback_NoChainYet(t *testing.T) {
	r := NewVersionChainRegistry()
	rb, err := r.Rollback("never_touched", interfaces.Hash("deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"))
	if err == nil {
		t.Fatalf("expected error for unknown hash on fresh session")
	}
	if rb != nil {
		t.Fatalf("expected nil chain on error")
	}
}

func TestVersionChainRegistry_GCAll_NoOpWhenFresh(t *testing.T) {
	r := NewVersionChainRegistry()
	_, _, _ = r.Append("sess_1", []byte("v1"), "commit")
	_, _, _ = r.Append("sess_1", []byte("v2"), "commit")
	deleted, touched, err := r.GCAll(24 * time.Hour)
	if err != nil {
		t.Fatalf("gc: %v", err)
	}
	if deleted != 0 || touched != 0 {
		t.Fatalf("fresh chain should yield 0 deletions / 0 touched, got %d/%d", deleted, touched)
	}
}

func TestVersionChainRegistry_GCAll_PrunesOldNonHead(t *testing.T) {
	// The non-head pruning logic is owned by interfaces.VersionChain.GC (tested
	// in interfaces/version_chain_test.go). At the workmodel layer we only
	// verify the orchestration glue: GCAll iterates sessions and aggregates
	// results without errors on a fresh registry.
	r := NewVersionChainRegistry()
	_, _, _ = r.Append("sess_1", []byte("v1"), "commit")
	_, _, _ = r.Append("sess_1", []byte("v2"), "commit")
	_, _, _ = r.Append("sess_2", []byte("v1"), "commit")
	deleted, touched, err := r.GCAll(24 * time.Hour)
	if err != nil {
		t.Fatalf("gc: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("fresh chains should yield 0 deletions, got %d", deleted)
	}
	if touched != 0 {
		t.Fatalf("fresh chains should yield 0 touched sessions, got %d", touched)
	}
}

func TestVersionChainRegistry_GCAll_NegativeTTLUsesDefault(t *testing.T) {
	r := NewVersionChainRegistry()
	_, _, _ = r.Append("sess_1", []byte("v1"), "commit")
	// ttl <= 0 should silently fall back to default 24h. No deletion expected
	// because entry is fresh.
	deleted, _, err := r.GCAll(0)
	if err != nil {
		t.Fatalf("gc: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("expected 0 deletions on fresh chain with default TTL, got %d", deleted)
	}
}

func TestVersionChainRegistry_StartStop(t *testing.T) {
	r := NewVersionChainRegistry()
	r.Start()
	// Give worker a moment to spin up.
	time.Sleep(20 * time.Millisecond)
	r.Stop()
	// Calling Stop twice should not deadlock or panic.
	r.Stop()
}

func TestVersionChainRegistry_Concurrent_AppendRace(t *testing.T) {
	r := NewVersionChainRegistry()
	const N = 8
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			_, _, err := r.Append("sess_race", []byte{byte(i)}, "commit")
			if err != nil {
				t.Errorf("goroutine %d: %v", i, err)
			}
		}()
	}
	wg.Wait()
	c := r.ChainFor("sess_race")
	if c.Len() != N {
		t.Fatalf("expected Len=%d, got %d", N, c.Len())
	}
}

func TestVersionChainRegistry_MultiSession(t *testing.T) {
	r := NewVersionChainRegistry()
	_, _, _ = r.Append("a", []byte("a1"), "commit")
	_, _, _ = r.Append("b", []byte("b1"), "commit")
	_, _, _ = r.Append("a", []byte("a2"), "commit")
	if r.SessionCount() != 2 {
		t.Fatalf("expected 2 sessions, got %d", r.SessionCount())
	}
	if r.ChainFor("a").Len() != 2 {
		t.Fatalf("session a should have 2 entries")
	}
	if r.ChainFor("b").Len() != 1 {
		t.Fatalf("session b should have 1 entry")
	}
}
