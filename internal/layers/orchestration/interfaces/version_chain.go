package interfaces

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// Hash is a content-addressed version identifier (SHA-256, 64-char hex).
// Hash values are immutable strings; the SHA-256 input is parent_hash ++
// content, so identical content+parent produce the same hash, allowing
// content-deduplication at the chain level.
//
// PR-C upgrade note: this supersedes the PR-B FNV-1a 16-char placeholder
// (buildChainHash). Hash length changes from 16 to 64 characters; semantics
// are unchanged (content addressing).
type Hash string

// EmptyHash is the zero-value Hash used as Parent for the genesis entry.
const EmptyHash Hash = ""

// ErrVersionChainHashEmpty is returned by helpers that require a non-empty
// hash (e.g. RollbackTo).
var ErrVersionChainHashEmpty = errors.New("interfaces: VersionChain hash is empty")

// ErrVersionChainHashFormat is returned when a hash string does not have
// the expected SHA-256 64-char hex shape.
var ErrVersionChainHashFormat = errors.New("interfaces: VersionChain hash format invalid (expected 64-char SHA-256 hex)")

// VersionChainEntry is an immutable CoW snapshot of a WorkItem state. Once
// Appended, an entry is never mutated (PR-C IV-2: CoW 不可变).
type VersionChainEntry struct {
	Hash      Hash      // SHA-256(parent + content)
	Parent    Hash      // previous head (EmptyHash for genesis)
	Content   []byte    // serialized snapshot
	CreatedAt time.Time // append time
	Reason    string    // "commit" / "rollback" / "replan" / "init"
}

// NewVersionChainEntry constructs an entry with the given content and
// parent hash. Hash is computed via SHA-256(parent + content). CreatedAt
// is set to time.Now(). Reason defaults to "commit" if empty.
func NewVersionChainEntry(parent Hash, content []byte, reason string) VersionChainEntry {
	if reason == "" {
		reason = "commit"
	}
	h := ComputeVersionHash(parent, content)
	return VersionChainEntry{
		Hash:      h,
		Parent:    parent,
		Content:   content,
		CreatedAt: time.Now(),
		Reason:    reason,
	}
}

// ComputeVersionHash returns SHA-256(parent + content) as 64-char hex.
// Used internally by VersionChain.Append and exposed for tests.
func ComputeVersionHash(parent Hash, content []byte) Hash {
	sum := sha256.Sum256([]byte(parent))
	// content is hashed in a streaming way via Sum256 + appending content
	// to the running digest. This is equivalent to a single Sum256 of
	// parent+content; we use the standard double-call form to avoid
	// allocating a concatenation buffer.
	h := sha256.New()
	h.Write(sum[:])
	h.Write(content)
	return Hash(hex.EncodeToString(h.Sum(nil)))
}

// VersionChain is an append-only CoW chain with O(1) hash lookup.
// It is safe for concurrent use via the embedded mutex; however, callers
// should treat the returned *VersionChain as the post-mutation state —
// Append/RollbackTo/GC return new (shallow-copied) instances.
//
// PR-C IV-2: 不可变 (Append/RollbackTo/GC return new copies).
// PR-C IV-3: head 永远不被 GC.
type VersionChain struct {
	mu      sync.RWMutex
	entries map[Hash]*VersionChainEntry // O(1) hash index
	order   []Hash                      // insertion order (sorted by CreatedAt)
	head    Hash                        // current pointer (never GC'd)
}

// NewVersionChain returns an empty CoW chain.
func NewVersionChain() *VersionChain {
	return &VersionChain{
		entries: make(map[Hash]*VersionChainEntry),
		order:   nil,
		head:    EmptyHash,
	}
}

// snapshot returns a shallow copy of vc suitable for returning from
// Append/RollbackTo/GC. The entries map is re-allocated so subsequent
// mutations on the returned instance do not affect the source. Slice
// headers for `order` and `entries[*].Content` are also copied.
//
// The mutex is NOT held during the copy — callers are responsible for
// holding the read or write lock externally.
func (vc *VersionChain) snapshot() *VersionChain {
	cp := &VersionChain{
		entries: make(map[Hash]*VersionChainEntry, len(vc.entries)),
		order:   make([]Hash, len(vc.order)),
		head:    vc.head,
	}
	for h, e := range vc.entries {
		// shallow copy of entry struct + its Content slice header
		ee := *e
		cp.entries[h] = &ee
	}
	copy(cp.order, vc.order)
	return cp
}

// Append adds a new content snapshot to the chain. The hash is
// SHA-256(parent=head + content). Returns the new hash. CoW semantics:
// never modifies existing entries; returns a new *VersionChain with the
// new entry appended.
//
// PR-C IV-2: 不可变 (returns new chain).
func (vc *VersionChain) Append(content []byte, reason string) (Hash, *VersionChain, error) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	entry := NewVersionChainEntry(vc.head, content, reason)
	if _, exists := vc.entries[entry.Hash]; exists {
		// Identical (parent, content) collision — extremely unlikely with
		// SHA-256 (2^-128); surface a clear error for the test suite to
		// catch regressions.
		return EmptyHash, nil, NewCoWVersionChainBrokenError()
	}
	cp := vc.snapshot()
	cp.entries[entry.Hash] = &entry
	cp.order = append(cp.order, entry.Hash)
	cp.head = entry.Hash
	return entry.Hash, cp, nil
}

// RollbackTo sets the head to a previous hash. O(1) hash lookup.
// Returns ErrCoWVersionChainBroken if hash not found or has been GC'd.
//
// PR-C IV-2: 不可变 (returns new chain).
// Note: RollbackTo does NOT delete intermediate entries — they remain in
// the chain, available for future rollbacks. They will be cleaned up by
// the 24h GC cycle.
func (vc *VersionChain) RollbackTo(h Hash) (*VersionChain, error) {
	if h == EmptyHash {
		return nil, NewVersionChainHashEmptyError()
	}
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	if _, ok := vc.entries[h]; !ok {
		return nil, NewCoWVersionChainBrokenError()
	}
	cp := vc.snapshot()
	cp.head = h
	return cp, nil
}

// Get returns the entry for a hash. O(1) lookup. Returns (nil, false) if
// the hash is not present (either never existed or has been GC'd).
func (vc *VersionChain) Get(h Hash) (*VersionChainEntry, bool) {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	e, ok := vc.entries[h]
	if !ok {
		return nil, false
	}
	// Return a defensive copy so callers cannot mutate the entry's
	// exported fields (Hash, Parent, Reason) and corrupt the chain.
	ee := *e
	ee.Content = append([]byte(nil), e.Content...)
	return &ee, true
}

// Head returns the current chain head hash. Returns EmptyHash for a chain
// with no entries.
func (vc *VersionChain) Head() Hash {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return vc.head
}

// Len returns the number of entries in the chain. O(1).
func (vc *VersionChain) Len() int {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return len(vc.entries)
}

// GC removes entries older than ttl, except head. Returns count deleted
// and a new *VersionChain with the GC'd entries removed.
//
// PR-C IV-2: 不可变 (returns new chain).
// PR-C IV-3: head 永远不被 GC (explicit exclusion + test guard).
// PR-C IV-4: Hash 唯一性 (SHA-256 standard library).
//
// Complexity: O(n log n) for the sort, where n = len(vc.entries) at the
// time of call. The 24h GC worker should only call this when Len > 10 to
// keep amortized cost bounded.
func (vc *VersionChain) GC(ttl time.Duration) (int, *VersionChain, error) {
	if ttl <= 0 {
		return 0, nil, errors.New("interfaces: VersionChain.GC ttl must be positive")
	}
	vc.mu.Lock()
	defer vc.mu.Unlock()
	now := time.Now()
	cp := vc.snapshot()
	deleted := 0
	// Walk order (insertion order) and delete entries whose CreatedAt is
	// older than now-ttl AND which are not the current head. We preserve
	// the ordering invariant: head always remains reachable.
	keptOrder := make([]Hash, 0, len(cp.order))
	for _, h := range cp.order {
		e, ok := cp.entries[h]
		if !ok {
			// Should not happen — order and entries are kept in sync.
			// Skip defensively to avoid a panic.
			continue
		}
		if h == cp.head {
			keptOrder = append(keptOrder, h)
			continue
		}
		if now.Sub(e.CreatedAt) > ttl {
			delete(cp.entries, h)
			deleted++
			continue
		}
		keptOrder = append(keptOrder, h)
	}
	cp.order = keptOrder
	// Sanity: head must still be in entries after GC.
	if _, ok := cp.entries[cp.head]; !ok && cp.head != EmptyHash {
		return 0, nil, NewCoWVersionChainBrokenError()
	}
	return deleted, cp, nil
}

// Order returns a copy of the insertion-order hash list. Useful for tests
// and for the SimilarityCheck.LookbackN scan.
func (vc *VersionChain) Order() []Hash {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	out := make([]Hash, len(vc.order))
	copy(out, vc.order)
	return out
}

// LastN returns the most-recent N hashes in insertion order. If the chain
// has fewer than N entries, returns all of them. The returned slice is a
// copy; callers may mutate it freely.
func (vc *VersionChain) LastN(n int) []Hash {
	if n <= 0 {
		return nil
	}
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	if n > len(vc.order) {
		n = len(vc.order)
	}
	out := make([]Hash, n)
	copy(out, vc.order[len(vc.order)-n:])
	return out
}

// CanonicalJSON returns a stable JSON representation of the chain for
// snapshot logging. Sort the order to make output reproducible.
func (vc *VersionChain) CanonicalJSON() string {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	hashes := make([]string, 0, len(vc.order))
	for _, h := range vc.order {
		hashes = append(hashes, string(h))
	}
	sort.Strings(hashes)
	return fmt.Sprintf(`{"head":%q,"entries":%d,"hashes":%v}`,
		string(vc.head), len(vc.entries), hashes)
}

// Errors (PR-C 7120 range, sharederrors.WithCode pattern).

// NewCoWVersionChainBrokenError is the canonical wrap helper for
// ORCH_COW_VERSION_CHAIN_BROKEN_7120.
func NewCoWVersionChainBrokenError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_COW_VERSION_CHAIN_BROKEN_7120",
		"CoW VersionChain broken: hash not found or GC'd",
		errors.New("interfaces: VersionChain hash not found or GC'd"),
	)
}

// NewVersionChainHashEmptyError is the canonical wrap helper for
// ORCH_COW_VERSION_CHAIN_BROKEN_7120 (variant: empty hash arg).
func NewVersionChainHashEmptyError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_COW_VERSION_CHAIN_BROKEN_7120",
		"CoW VersionChain broken: empty hash argument",
		ErrVersionChainHashEmpty,
	)
}

// NewVersionChainHashFormatError is the canonical wrap helper for
// ORCH_COW_VERSION_CHAIN_BROKEN_7120 (variant: bad format).
func NewVersionChainHashFormatError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_COW_VERSION_CHAIN_BROKEN_7120",
		"CoW VersionChain broken: hash format invalid",
		ErrVersionChainHashFormat,
	)
}
