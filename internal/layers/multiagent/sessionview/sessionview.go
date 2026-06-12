// Package sessionview provides the Fork-isolated COW view of a Session.
//
// DM-20260611-005 (devrix-multiagent-isolation).
//
// The view shares immutable fields with the parent session (ID, CreatedAt,
// Model) and isolates the mutable fields (metadata, snapshot) behind its
// own lock. Children write through SessionView methods; the parent is
// only touched on an explicit MergeToParent.
package sessionview

import (
	"fmt"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

// View is a forked child view of a parent Session. Created via Fork.
type View struct {
	id        string
	createdAt time.Time
	model     string
	budget    types.TokenBudget

	mu       sync.RWMutex
	metadata map[string]any
	snapshot []byte
}

// Fork creates a child view of the given parent session.
// The view starts with an empty metadata map but copies the parent's
// context snapshot so reads see the inherited bytes. The parent is
// not modified.
func Fork(parent *types.Session) *View {
	if parent == nil {
		return nil
	}
	var snapCopy []byte
	if len(parent.ContextSnapshot) > 0 {
		snapCopy = make([]byte, len(parent.ContextSnapshot))
		copy(snapCopy, parent.ContextSnapshot)
	}
	return &View{
		id:        parent.SessionID,
		createdAt: parent.CreatedAt,
		model:     parent.Model,
		budget:    types.DefaultTokenBudget(),
		metadata:  make(map[string]any, 8),
		snapshot:  snapCopy,
	}
}

// ID returns the parent session id (shared, read-only).
func (v *View) ID() string { return v.id }

// CreatedAt returns the parent session creation time (shared, read-only).
func (v *View) CreatedAt() time.Time { return v.createdAt }

// Model returns the model set at fork time.
func (v *View) Model() string { return v.model }

// TokenBudget returns the budget snapshot at fork time.
func (v *View) TokenBudget() types.TokenBudget { return v.budget }

// SetMetadata writes a key/value pair isolated to this view. The parent's
// metadata map is not touched.
func (v *View) SetMetadata(key string, value any) {
	if v == nil || key == "" {
		return
	}
	v.mu.Lock()
	if v.metadata == nil {
		v.metadata = make(map[string]any, 8)
	}
	v.metadata[key] = value
	v.mu.Unlock()
}

// GetMetadata reads a key previously set on this view.
// Returns (value, true) when present, (nil, false) otherwise.
func (v *View) GetMetadata(key string) (any, bool) {
	if v == nil || key == "" {
		return nil, false
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	val, ok := v.metadata[key]
	return val, ok
}

// MetadataSnapshot returns a shallow copy of the view's metadata.
func (v *View) MetadataSnapshot() map[string]any {
	if v == nil {
		return nil
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := make(map[string]any, len(v.metadata))
	for k, val := range v.metadata {
		out[k] = val
	}
	return out
}

// SetSnapshot replaces the view's local context snapshot. The parent's
// ContextSnapshot is not modified.
func (v *View) SetSnapshot(snap []byte) {
	if v == nil {
		return
	}
	v.mu.Lock()
	if len(snap) == 0 {
		v.snapshot = nil
	} else {
		buf := make([]byte, len(snap))
		copy(buf, snap)
		v.snapshot = buf
	}
	v.mu.Unlock()
}

// Snapshot returns a copy of the local context snapshot.
func (v *View) Snapshot() []byte {
	if v == nil {
		return nil
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	if len(v.snapshot) == 0 {
		return nil
	}
	out := make([]byte, len(v.snapshot))
	copy(out, v.snapshot)
	return out
}

// MergeToParent copies the view's metadata and snapshot onto the parent
// session, then drops the local buffer. Safe to call once per view.
// Returns an error when the parent is nil.
func (v *View) MergeToParent(parent *types.Session) error {
	if v == nil {
		return fmt.Errorf("sessionview: nil view")
	}
	if parent == nil {
		return fmt.Errorf("sessionview: nil parent session")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if parent.Metadata == nil {
		parent.Metadata = make(map[string]any, len(v.metadata))
	}
	for k, val := range v.metadata {
		parent.Metadata[k] = val
	}
	if len(v.snapshot) > 0 {
		buf := make([]byte, len(v.snapshot))
		copy(buf, v.snapshot)
		parent.ContextSnapshot = buf
	}
	return nil
}
