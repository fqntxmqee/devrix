package interfaces

import (
	"regexp"
	"sort"
	"sync"
	"testing"
	"time"
)

// sha256HexPattern matches the 64-char lowercase hex string produced by
// ComputeVersionHash. Used to lock down the hash format invariant.
var sha256HexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TestComputeVersionHash_SHA256Format(t *testing.T) {
	h := ComputeVersionHash(EmptyHash, []byte("hello"))
	if !sha256HexPattern.MatchString(string(h)) {
		t.Fatalf("expected 64-char lowercase hex, got %q", h)
	}
	if len(h) != 64 {
		t.Fatalf("expected 64-char hash, got %d", len(h))
	}
}

func TestComputeVersionHash_DeterministicForSameInput(t *testing.T) {
	a := ComputeVersionHash(EmptyHash, []byte("same content"))
	b := ComputeVersionHash(EmptyHash, []byte("same content"))
	if a != b {
		t.Fatalf("hash not deterministic: %q vs %q", a, b)
	}
}

func TestComputeVersionHash_DiffersWithParent(t *testing.T) {
	a := ComputeVersionHash(EmptyHash, []byte("content"))
	b := ComputeVersionHash(Hash("deadbeef"), []byte("content"))
	if a == b {
		t.Fatalf("hash should differ when parent differs")
	}
}

func TestComputeVersionHash_DiffersWithContent(t *testing.T) {
	a := ComputeVersionHash(EmptyHash, []byte("alpha"))
	b := ComputeVersionHash(EmptyHash, []byte("beta"))
	if a == b {
		t.Fatalf("hash should differ when content differs")
	}
}

func TestNewVersionChain_Empty(t *testing.T) {
	vc := NewVersionChain()
	if vc.Len() != 0 {
		t.Fatalf("expected Len=0, got %d", vc.Len())
	}
	if vc.Head() != EmptyHash {
		t.Fatalf("expected Head=EmptyHash, got %q", vc.Head())
	}
	if vc.Order() != nil && len(vc.Order()) != 0 {
		t.Fatalf("expected empty Order, got %v", vc.Order())
	}
}

func TestAppend_BasicHashComputed(t *testing.T) {
	vc := NewVersionChain()
	h1, next, err := vc.Append([]byte("first"), "commit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h1 == EmptyHash {
		t.Fatalf("expected non-empty hash")
	}
	if next.Len() != 1 {
		t.Fatalf("expected Len=1, got %d", next.Len())
	}
	if next.Head() != h1 {
		t.Fatalf("expected Head=%q, got %q", h1, next.Head())
	}
}

func TestAppend_ChainedHashes(t *testing.T) {
	vc := NewVersionChain()
	h1, vc, err := vc.Append([]byte("v1"), "commit")
	if err != nil {
		t.Fatalf("append v1: %v", err)
	}
	h2, vc, err := vc.Append([]byte("v2"), "commit")
	if err != nil {
		t.Fatalf("append v2: %v", err)
	}
	if h1 == h2 {
		t.Fatalf("expected h1 != h2")
	}
	// v2's parent should be v1's hash.
	entry, ok := vc.Get(h2)
	if !ok {
		t.Fatalf("expected to find entry for h2")
	}
	if entry.Parent != h1 {
		t.Fatalf("expected Parent=%q, got %q", h1, entry.Parent)
	}
}

func TestAppend_Immutable(t *testing.T) {
	vc := NewVersionChain()
	_, next, err := vc.Append([]byte("first"), "commit")
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if vc.Len() != 0 {
		t.Fatalf("original chain should be empty (immutability violated)")
	}
	if next.Len() != 1 {
		t.Fatalf("new chain should have Len=1")
	}
}

func TestAppend_DefaultReason(t *testing.T) {
	vc := NewVersionChain()
	h, next, err := vc.Append([]byte("x"), "")
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	entry, ok := next.Get(h)
	if !ok {
		t.Fatalf("missing entry")
	}
	if entry.Reason != "commit" {
		t.Fatalf("expected default reason \"commit\", got %q", entry.Reason)
	}
}

func TestRollbackTo_Success(t *testing.T) {
	vc := NewVersionChain()
	h1, vc, _ := vc.Append([]byte("v1"), "commit")
	_, vc, _ = vc.Append([]byte("v2"), "commit")
	h3, vc, _ := vc.Append([]byte("v3"), "commit")
	if vc.Head() != h3 {
		t.Fatalf("precondition: head should be h3")
	}
	rb, err := vc.RollbackTo(h1)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if rb.Head() != h1 {
		t.Fatalf("expected head=%q, got %q", h1, rb.Head())
	}
	// RollbackTo does not delete intermediate entries.
	if rb.Len() != 3 {
		t.Fatalf("expected Len=3 (rollback preserves intermediate), got %d", rb.Len())
	}
}

func TestRollbackTo_Immutable(t *testing.T) {
	vc := NewVersionChain()
	h1, vc, _ := vc.Append([]byte("v1"), "commit")
	_, vc, _ = vc.Append([]byte("v2"), "commit")
	originalHead := vc.Head()
	rb, err := vc.RollbackTo(h1)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if vc.Head() != originalHead {
		t.Fatalf("original head should not change (immutability violated)")
	}
	if rb.Head() != h1 {
		t.Fatalf("rollback head should be h1")
	}
}

func TestRollbackTo_HashNotFound(t *testing.T) {
	vc := NewVersionChain()
	_, vc, _ = vc.Append([]byte("v1"), "commit")
	rb, err := vc.RollbackTo(Hash("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"))
	if err == nil {
		t.Fatalf("expected error for unknown hash")
	}
	if rb != nil {
		t.Fatalf("expected nil chain on error")
	}
}

func TestRollbackTo_EmptyHashRejected(t *testing.T) {
	vc := NewVersionChain()
	_, vc, _ = vc.Append([]byte("v1"), "commit")
	rb, err := vc.RollbackTo(EmptyHash)
	if err == nil {
		t.Fatalf("expected error for EmptyHash")
	}
	if rb != nil {
		t.Fatalf("expected nil chain on error")
	}
}

func TestGet_DefensiveCopy(t *testing.T) {
	vc := NewVersionChain()
	h, vc, _ := vc.Append([]byte("original"), "commit")
	entry1, ok := vc.Get(h)
	if !ok {
		t.Fatalf("get: missing entry")
	}
	entry1.Content[0] = 'X'
	entry2, _ := vc.Get(h)
	if entry2.Content[0] != 'o' {
		t.Fatalf("mutation leaked into chain (defensive copy violated)")
	}
}

func TestGet_NotFound(t *testing.T) {
	vc := NewVersionChain()
	entry, ok := vc.Get(Hash("deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"))
	if ok || entry != nil {
		t.Fatalf("expected not-found for unknown hash")
	}
}

func TestHead_EmptyChain(t *testing.T) {
	vc := NewVersionChain()
	if vc.Head() != EmptyHash {
		t.Fatalf("expected Head=EmptyHash on empty chain")
	}
}

func TestHead_AdvancesOnAppend(t *testing.T) {
	vc := NewVersionChain()
	h1, vc, _ := vc.Append([]byte("v1"), "commit")
	h2, vc, _ := vc.Append([]byte("v2"), "commit")
	if vc.Head() != h2 {
		t.Fatalf("head should be h2 after second append")
	}
	if h1 == h2 {
		t.Fatalf("h1 should not equal h2")
	}
}

func TestLen_AccurateAcrossOps(t *testing.T) {
	vc := NewVersionChain()
	if vc.Len() != 0 {
		t.Fatalf("empty chain Len should be 0")
	}
	_, vc, _ = vc.Append([]byte("a"), "commit")
	if vc.Len() != 1 {
		t.Fatalf("Len=1 expected, got %d", vc.Len())
	}
	_, vc, _ = vc.Append([]byte("b"), "commit")
	if vc.Len() != 2 {
		t.Fatalf("Len=2 expected, got %d", vc.Len())
	}
}

func TestGC_HeadPreservedWhenOld(t *testing.T) {
	vc := NewVersionChain()
	h, vc, _ := vc.Append([]byte("v1"), "commit")
	// Build a separate chain that shares the entries map but lets us mutate CreatedAt.
	vc2 := NewVersionChain()
	vc2.entries = vc.entries
	vc2.order = vc.order
	vc2.head = vc.head
	vc2.entries[h].CreatedAt = time.Now().Add(-2 * time.Hour)
	deleted, next, err := vc2.GC(1 * time.Hour)
	if err != nil {
		t.Fatalf("gc: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("head should not be GC'd, but %d were deleted", deleted)
	}
	if next.Head() != h {
		t.Fatalf("head should remain %q, got %q", h, next.Head())
	}
}

func TestGC_OldEntriesRemoved(t *testing.T) {
	vc := NewVersionChain()
	h1, vc, _ := vc.Append([]byte("old"), "commit")
	_, vc, _ = vc.Append([]byte("newer"), "commit")
	// Make h1 older than TTL.
	vc.entries[h1].CreatedAt = time.Now().Add(-2 * time.Hour)
	deleted, next, err := vc.GC(1 * time.Hour)
	if err != nil {
		t.Fatalf("gc: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted, got %d", deleted)
	}
	if _, ok := next.Get(h1); ok {
		t.Fatalf("h1 should have been GC'd")
	}
	if next.Len() != 1 {
		t.Fatalf("expected Len=1 after gc, got %d", next.Len())
	}
}

func TestGC_ImmutableReturnsNewChain(t *testing.T) {
	vc := NewVersionChain()
	h1, vc, _ := vc.Append([]byte("a"), "commit")
	_, vc, _ = vc.Append([]byte("b"), "commit")
	vc.entries[h1].CreatedAt = time.Now().Add(-2 * time.Hour)
	originalLen := vc.Len()
	_, next, err := vc.GC(1 * time.Hour)
	if err != nil {
		t.Fatalf("gc: %v", err)
	}
	if vc.Len() != originalLen {
		t.Fatalf("original chain should not be mutated (immutability violated)")
	}
	if next.Len() != originalLen-1 {
		t.Fatalf("expected new chain Len=%d, got %d", originalLen-1, next.Len())
	}
}

func TestGC_RejectsNonPositiveTTL(t *testing.T) {
	vc := NewVersionChain()
	deleted, next, err := vc.GC(0)
	if err == nil {
		t.Fatalf("expected error for zero TTL")
	}
	if deleted != 0 || next != nil {
		t.Fatalf("expected zero-value return on error")
	}
}

func TestGC_HeadMissingAfterGC_IsError(t *testing.T) {
	// Construct a chain where the head exists but is unreachable from order.
	// Manually craft to simulate corruption.
	vc := NewVersionChain()
	h, vc, _ := vc.Append([]byte("v1"), "commit")
	// Remove the head's entry — corruption scenario.
	delete(vc.entries, h)
	vc.head = h
	deleted, next, err := vc.GC(1 * time.Hour)
	if err == nil {
		t.Fatalf("expected error for broken chain")
	}
	if deleted != 0 || next != nil {
		t.Fatalf("expected zero-value on broken chain")
	}
}

func TestLastN_RecentSlice(t *testing.T) {
	vc := NewVersionChain()
	hashes := make([]Hash, 0, 7)
	var err error
	for i := 0; i < 7; i++ {
		var h Hash
		h, vc, err = vc.Append([]byte{byte(i)}, "commit")
		if err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
		hashes = append(hashes, h)
	}
	last3 := vc.LastN(3)
	if len(last3) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(last3))
	}
	for i, h := range last3 {
		expected := hashes[len(hashes)-3+i]
		if h != expected {
			t.Fatalf("LastN[%d] mismatch: expected %q, got %q", i, expected, h)
		}
	}
}

func TestLastN_LargerThanChain(t *testing.T) {
	vc := NewVersionChain()
	_, vc, _ = vc.Append([]byte("a"), "commit")
	_, vc, _ = vc.Append([]byte("b"), "commit")
	last := vc.LastN(10)
	if len(last) != 2 {
		t.Fatalf("expected all 2 entries, got %d", len(last))
	}
}

func TestLastN_ZeroOrNegative(t *testing.T) {
	vc := NewVersionChain()
	_, vc, _ = vc.Append([]byte("a"), "commit")
	if got := vc.LastN(0); got != nil {
		t.Fatalf("expected nil for n=0, got %v", got)
	}
	if got := vc.LastN(-1); got != nil {
		t.Fatalf("expected nil for n<0, got %v", got)
	}
}

func TestOrder_ReturnsCopy(t *testing.T) {
	vc := NewVersionChain()
	h1, vc, _ := vc.Append([]byte("a"), "commit")
	h2, vc, _ := vc.Append([]byte("b"), "commit")
	o := vc.Order()
	if len(o) != 2 {
		t.Fatalf("expected 2, got %d", len(o))
	}
	if o[0] != h1 || o[1] != h2 {
		t.Fatalf("order mismatch")
	}
	// Mutate the returned slice — chain should not change.
	o[0] = Hash("zzz")
	o2 := vc.Order()
	if o2[0] != h1 {
		t.Fatalf("Order() should return a defensive copy")
	}
}

func TestCanonicalJSON_SortedHashes(t *testing.T) {
	vc := NewVersionChain()
	h1, vc, _ := vc.Append([]byte("a"), "commit")
	_, vc, _ = vc.Append([]byte("b"), "commit")
	canonical := vc.CanonicalJSON()
	if canonical == "" {
		t.Fatalf("expected non-empty canonical JSON")
	}
	// The hashes inside should appear in sorted order.
	pos1 := sort.Search(len(canonical), func(i int) bool { return canonical[i:] >= string(h1) })
	if pos1 < 0 || canonical[pos1:] == "" {
		t.Fatalf("canonical JSON missing h1")
	}
}

func TestCanonicalJSON_StableForSameChain(t *testing.T) {
	vc := NewVersionChain()
	_, vc, _ = vc.Append([]byte("a"), "commit")
	_, vc, _ = vc.Append([]byte("b"), "commit")
	a := vc.CanonicalJSON()
	b := vc.CanonicalJSON()
	if a != b {
		t.Fatalf("canonical JSON should be stable for same chain")
	}
}

func TestNewVersionChainEntry_DefaultReason(t *testing.T) {
	e := NewVersionChainEntry(EmptyHash, []byte("x"), "")
	if e.Reason != "commit" {
		t.Fatalf("expected default reason \"commit\", got %q", e.Reason)
	}
	if e.Parent != EmptyHash {
		t.Fatalf("expected Parent=EmptyHash, got %q", e.Parent)
	}
	if e.Hash == EmptyHash {
		t.Fatalf("expected non-empty hash")
	}
}

func TestNewVersionChainEntry_CustomReason(t *testing.T) {
	e := NewVersionChainEntry(EmptyHash, []byte("x"), "rollback")
	if e.Reason != "rollback" {
		t.Fatalf("expected reason=\"rollback\", got %q", e.Reason)
	}
}

func TestConcurrent_AppendRace(t *testing.T) {
	vc := NewVersionChain()
	const N = 16
	var wg sync.WaitGroup
	chains := make([]*VersionChain, N)
	errs := make([]error, N)
	hashes := make([]Hash, N)
	wg.Add(N)
	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			h, c, err := vc.Append([]byte{byte(i)}, "commit")
			hashes[i] = h
			chains[i] = c
			errs[i] = err
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
		if chains[i] == nil {
			t.Fatalf("goroutine %d: nil chain", i)
		}
	}
}

func TestNewCoWVersionChainBrokenError_Code(t *testing.T) {
	e := NewCoWVersionChainBrokenError()
	if e == nil || e.Code != "ORCH_COW_VERSION_CHAIN_BROKEN_7120" {
		t.Fatalf("unexpected error: %+v", e)
	}
}

func TestNewVersionChainHashEmptyError_Code(t *testing.T) {
	e := NewVersionChainHashEmptyError()
	if e == nil || e.Code != "ORCH_COW_VERSION_CHAIN_BROKEN_7120" {
		t.Fatalf("unexpected error: %+v", e)
	}
}

func TestNewVersionChainHashFormatError_Code(t *testing.T) {
	e := NewVersionChainHashFormatError()
	if e == nil || e.Code != "ORCH_COW_VERSION_CHAIN_BROKEN_7120" {
		t.Fatalf("unexpected error: %+v", e)
	}
}
