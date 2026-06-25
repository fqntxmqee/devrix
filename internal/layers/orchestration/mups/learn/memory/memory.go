// Package memory: Learn node's 3-channel memory interface and implementations (PR-E4).
//
// Learn's LP-2 principle requires that asset.LearningAsset classes be partitioned
// into 3 independent memory channels with no cross-channel leakage:
//
//   SkillMemory      ← LearningSOP + LearningProtocol    (deterministic, ★4-5)
//   FeedbackMemory   ← LearningKnowledge + LearningConclusion (soft, ★2-3)
//   ScheduledMemory  ← LearningPending                   (deferred, ★1, ⭐new)
//
// Each channel rejects assets whose Class does not belong to it via
// asset.ErrAssetClassMismatch (fail-fast at the boundary).
//
// All stores are goroutine-safe via sync.RWMutex so the next Observe tick
// can read them concurrently with Learner.Inject writes.
package memory

import (
	"context"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn/asset"
	"github.com/devrix/devrix/internal/shared/types"
)

// Memory is the LP-2 channel contract. Every implementation MUST enforce
// class-to-channel membership (LP-2 隔离) in Store() and reject mismatches
// with ErrAssetClassMismatch.
type Memory interface {
	// Store inserts an asset. Class MUST belong to this channel; otherwise
	// returns ErrAssetClassMismatch. Re-storing the same AssetKey replaces
	// the prior value.
	Store(ctx context.Context, asset *asset.LearningAsset) error

	// Retrieve fetches an asset by AssetKey. Returns (nil, nil) if absent;
	// returns (asset, nil) on hit. Expired assets are NOT auto-evicted here;
	// callers should filter via MemoryFilter.Expired or call IsExpired().
	Retrieve(ctx context.Context, key string) (*asset.LearningAsset, error)

	// Delete removes an asset by AssetKey. Returns nil if the key was absent.
	Delete(ctx context.Context, key string) error

	// List returns assets matching the filter. Empty filter returns all
	// (channel-local) assets. Expired handling is governed by filter.Expired:
	//   filter.Expired == false (default) → skip expired assets
	//   filter.Expired == true            → return only expired assets
	List(ctx context.Context, filter MemoryFilter) ([]*asset.LearningAsset, error)
}

// MemoryChannel is the LP-2 channel identifier.
type MemoryChannel int

const (
	// MemorySkill — LearningSOP + LearningProtocol (★4-5).
	MemorySkill MemoryChannel = iota

	// MemoryFeedback — LearningKnowledge + LearningConclusion (★2-3).
	MemoryFeedback

	// MemoryScheduled — LearningPending (★1, ⭐new).
	MemoryScheduled
)

// String returns the wire format name.
func (c MemoryChannel) String() string {
	switch c {
	case MemorySkill:
		return "skill"
	case MemoryFeedback:
		return "feedback"
	case MemoryScheduled:
		return "scheduled"
	default:
		return "unknown"
	}
}

// allowedClasses returns the asset.LearningClass set accepted by each channel.
// Compile-time guarantee that LP-2 partitioning is exhaustive across the
// 5 asset.LearningClass values.
func (c MemoryChannel) allowedClasses() map[asset.LearningClass]bool {
	switch c {
	case MemorySkill:
		return map[asset.LearningClass]bool{
			asset.LearningClass(types.LearningSOP):      true,
			asset.LearningClass(types.LearningProtocol): true,
		}
	case MemoryFeedback:
		return map[asset.LearningClass]bool{
			asset.LearningClass(types.LearningKnowledge):  true,
			asset.LearningClass(types.LearningConclusion): true,
		}
	case MemoryScheduled:
		return map[asset.LearningClass]bool{
			asset.LearningClass(types.LearningPending): true,
		}
	default:
		return nil
	}
}

// MemoryFilter selects assets returned by Memory.List. The zero value
// matches every asset in the channel.
type MemoryFilter struct {
	// Class — when set, only assets with this Class match.
	Class asset.LearningClass

	// SessionID — when non-empty, only assets whose SessionID OR
	// SourceSessionIDs contain this value match (LP-5 跨会话可追溯).
	SessionID string

	// MinStrength — when > 0, only assets with Strength ≥ MinStrength match.
	MinStrength asset.CertaintyStrength

	// Expired — when true, return only expired assets; when false (default),
	// skip expired assets.
	Expired bool
}

// ─────────────────────────────────────────────────────────────────────────
// SkillMemory — LearningSOP + LearningProtocol (LP-2 隔离, concurrent-safe)
// ─────────────────────────────────────────────────────────────────────────

// SkillMemory stores deterministic procedural assets (SOP + Protocol). Used
// by D2 to retrieve matching procedures when an Observation.Kind routes to
// one of the action-shaped Plan kinds (Commitment / Protocol).
type SkillMemory struct {
	store map[string]*asset.LearningAsset
	mu    sync.RWMutex
}

// NewSkillMemory constructs an empty SkillMemory.
func NewSkillMemory() *SkillMemory {
	return &SkillMemory{
		store: make(map[string]*asset.LearningAsset),
	}
}

// Store — LP-2 enforcement: Class MUST be LearningSOP or LearningProtocol.
// v6.0.0 S6-A49 P0: emit memory.persist Span around the in-memory write.
func (m *SkillMemory) Store(ctx context.Context, a *asset.LearningAsset) error {
	if a == nil {
		return asset.ErrAssetIncomplete
	}
	if !MemorySkill.allowedClasses()[a.Class] {
		return asset.ErrAssetClassMismatch
	}
	ttlMs := ttlRemainingMs(a)
	end := hardening.EmitMemoryPersist(ctx, a.SessionID, MemorySkill.String(), a.Class.String(), ttlMs, assetPayloadSize(a))
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[a.AssetKey] = a
	end(nil)
	return nil
}

// Retrieve returns the asset by AssetKey, or (nil, nil) if absent.
func (m *SkillMemory) Retrieve(ctx context.Context, key string) (*asset.LearningAsset, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.store[key], nil
}

// Delete removes the asset by AssetKey.
func (m *SkillMemory) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.store, key)
	return nil
}

// List returns assets matching the filter.
func (m *SkillMemory) List(ctx context.Context, filter MemoryFilter) ([]*asset.LearningAsset, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return filterAssets(m.store, filter), nil
}

// ─────────────────────────────────────────────────────────────────────────
// FeedbackMemory — LearningKnowledge + LearningConclusion (LP-2 隔离,
// concurrent-safe)
// ─────────────────────────────────────────────────────────────────────────

// FeedbackMemory stores soft-knowledge assets (Knowledge + Conclusion). Used
// by D2 to surface prior hypotheses / statistical conclusions when an
// Observation.Kind routes to a hypothesis-shaped Plan kind (Scenario).
type FeedbackMemory struct {
	store map[string]*asset.LearningAsset
	mu    sync.RWMutex
}

// NewFeedbackMemory constructs an empty FeedbackMemory.
func NewFeedbackMemory() *FeedbackMemory {
	return &FeedbackMemory{
		store: make(map[string]*asset.LearningAsset),
	}
}

// Store — LP-2 enforcement: Class MUST be LearningKnowledge or
// LearningConclusion.
func (m *FeedbackMemory) Store(ctx context.Context, a *asset.LearningAsset) error {
	if a == nil {
		return asset.ErrAssetIncomplete
	}
	if !MemoryFeedback.allowedClasses()[a.Class] {
		return asset.ErrAssetClassMismatch
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[a.AssetKey] = a
	return nil
}

// Retrieve returns the asset by AssetKey, or (nil, nil) if absent.
func (m *FeedbackMemory) Retrieve(ctx context.Context, key string) (*asset.LearningAsset, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.store[key], nil
}

// Delete removes the asset by AssetKey.
func (m *FeedbackMemory) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.store, key)
	return nil
}

// List returns assets matching the filter.
func (m *FeedbackMemory) List(ctx context.Context, filter MemoryFilter) ([]*asset.LearningAsset, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return filterAssets(m.store, filter), nil
}

// ─────────────────────────────────────────────────────────────────────────
// ScheduledMemory — LearningPending (LP-2 隔离, concurrent-safe) +
// ScheduledRetry envelope
// ─────────────────────────────────────────────────────────────────────────

// ScheduledRetry wraps a LearningPending asset with retry-scheduling
// metadata. The default MaxRetries is 3 (matches PendingAssetContent
// validator upper bound).
type ScheduledRetry struct {
	// Asset — the LearningPending being retried.
	Asset *asset.LearningAsset

	// TriggerAt — when this retry should fire (defaults to Asset.ExpiryAt).
	TriggerAt time.Time

	// RetryCount — number of retries attempted so far.
	RetryCount int

	// MaxRetries — upper bound on retries (default 3, matches
	// PendingAssetContent validator).
	MaxRetries int

	// LastRetryAt — zero until first retry.
	LastRetryAt time.Time
}

// ScheduledMemory stores LearningPending assets wrapped in ScheduledRetry
// envelopes. The Learn node's ScheduledTick drains TriggerAt-due retries
// back into Verify (PR-E5 E5.3).
type ScheduledMemory struct {
	store map[string]*ScheduledRetry
	mu    sync.RWMutex
}

// NewScheduledMemory constructs an empty ScheduledMemory.
func NewScheduledMemory() *ScheduledMemory {
	return &ScheduledMemory{
		store: make(map[string]*ScheduledRetry),
	}
}

// Store — LP-2 enforcement: Class MUST be LearningPending. TriggerAt
// defaults to asset.ExpiryAt; MaxRetries defaults to
// asset.DefaultPendingMaxRetries when ≤ 0.
func (m *ScheduledMemory) Store(ctx context.Context, a *asset.LearningAsset) error {
	if a == nil {
		return asset.ErrAssetIncomplete
	}
	if !MemoryScheduled.allowedClasses()[a.Class] {
		return asset.ErrAssetClassMismatch
	}
	maxRetries := asset.DefaultPendingMaxRetries
	if pending, ok := a.Content.(*asset.PendingAssetContent); ok && pending.MaxRetries > 0 {
		maxRetries = pending.MaxRetries
	}
	triggerAt := a.ExpiryAt
	if pending, ok := a.Content.(*asset.PendingAssetContent); ok && !pending.NextRetryAt.IsZero() {
		triggerAt = pending.NextRetryAt
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[a.AssetKey] = &ScheduledRetry{
		Asset:      a,
		TriggerAt:  triggerAt,
		RetryCount: 0,
		MaxRetries: maxRetries,
	}
	return nil
}

// Retrieve returns the ScheduledRetry envelope by AssetKey, or (nil, nil)
// if absent.
func (m *ScheduledMemory) Retrieve(ctx context.Context, key string) (*ScheduledRetry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.store[key], nil
}

// Delete removes the retry envelope by AssetKey.
func (m *ScheduledMemory) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.store, key)
	return nil
}

// List returns the assets (envelope.Asset) matching the filter.
func (m *ScheduledMemory) List(ctx context.Context, filter MemoryFilter) ([]*asset.LearningAsset, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	asMap := make(map[string]*asset.LearningAsset, len(m.store))
	for k, v := range m.store {
		asMap[k] = v.Asset
	}
	return filterAssets(asMap, filter), nil
}

// ListDue returns deep copies of retry envelopes whose TriggerAt ≤ now.
// Used by ScheduledTick (PR-E5 E5.3).
//
// Copies are returned (not references) so callers can mutate the
// envelopes (RetryCount++, TriggerAt update) without holding the
// ScheduledMemory lock for the entire Learn/Escalate write path. The
// caller is responsible for re-applying mutations via Delete (escalate)
// or by computing the new TriggerAt and re-Storing the asset with
// updated PendingAssetContent.NextRetryAt (re-queue). The wrapper
// (DefaultLearner.ScheduledTick) is the only production caller and
// handles this contract.
func (m *ScheduledMemory) ListDue(now time.Time) []*ScheduledRetry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*ScheduledRetry, 0)
	for _, v := range m.store {
		if !v.TriggerAt.After(now) {
			out = append(out, &ScheduledRetry{
				Asset:       v.Asset,
				TriggerAt:   v.TriggerAt,
				RetryCount:  v.RetryCount,
				MaxRetries:  v.MaxRetries,
				LastRetryAt: v.LastRetryAt,
			})
		}
	}
	return out
}

// IsExhausted returns true when retry.RetryCount >= retry.MaxRetries.
// Used by ScheduledTick to decide whether to delete or re-queue.
func (r *ScheduledRetry) IsExhausted() bool {
	return r.RetryCount >= r.MaxRetries
}

// ForceExhaustRetry is a test-only helper that marks a ScheduledRetry as
// exhausted and due, allowing tests to exercise the escalation path in
// DefaultLearner.ScheduledTick without having to wait for the production
// TriggerAt / MaxRetries gates. Returns the (mutated) envelope so the test
// can read RetryCount / MaxRetries for assertions.
//
// This is exported (rather than left as a lower-case test helper) because
// the test lives in the parent learn package — keeping the symbol package-
// private would require moving the test into memory (which would also be
// fine, but the parent-package test is the canonical place for LP-1
// closed-loop coverage).
func (m *ScheduledMemory) ForceExhaustRetry(key string) (*ScheduledRetry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	env, ok := m.store[key]
	if !ok {
		return nil, asset.ErrAssetIncomplete
	}
	env.RetryCount = env.MaxRetries
	env.TriggerAt = time.Now().Add(-1 * time.Minute)
	return env, nil
}

// filterAssets applies the filter to a map. Shared by SkillMemory,
// FeedbackMemory, and ScheduledMemory.List to keep semantics identical.
func filterAssets(store map[string]*asset.LearningAsset, filter MemoryFilter) []*asset.LearningAsset {
	out := make([]*asset.LearningAsset, 0, len(store))
	now := time.Now()
	for _, a := range store {
		if filter.Class != asset.LearningClass(types.LearningUnknown) && a.Class != filter.Class {
			continue
		}
		if filter.SessionID != "" && a.SessionID != filter.SessionID {
			matched := false
			for _, sid := range a.SourceSessionIDs {
				if sid == filter.SessionID {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if filter.MinStrength > 0 && a.Strength < filter.MinStrength {
			continue
		}
		expired := a.IsExpired() || now.After(a.ExpiryAt)
		if expired != filter.Expired {
			continue
		}
		out = append(out, a)
	}
	return out
}

// ttlRemainingMs returns the asset's remaining TTL in milliseconds (clamped
// at 0 if already expired). Used as a Span attribute for memory.persist.
func ttlRemainingMs(a *asset.LearningAsset) int {
	if a == nil {
		return 0
	}
	remaining := time.Until(a.ExpiryAt)
	if remaining < 0 {
		return 0
	}
	return int(remaining / time.Millisecond)
}

// assetPayloadSize returns a coarse size estimate for the asset's content
// suitable for the memory.persist Span attribute. We avoid reflecting into
// the AssetContent interface (5 polymorphic classes) and instead use the
// AssetKey + ContentHash as a proxy length so the attribute is always cheap.
func assetPayloadSize(a *asset.LearningAsset) int {
	if a == nil {
		return 0
	}
	return len(a.AssetKey) + len(a.ContentHash) + len(a.SessionID)
}