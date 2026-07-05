package workmodel

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	DefaultMaxDecomposeDepth = 3
	DefaultMaxChildren       = 7
)

// WorkTree holds the canonical work-item tree per session.
type WorkTree struct {
	mu                sync.RWMutex
	items             map[string]map[string]*WorkItem // sessionID -> itemID -> WorkItem
	store             WorkItemStore
	maxDecomposeDepth int

	// versionChainReg is the optional CoW VersionChain registry (PR-C IV-2/3).
	// Lazy-initialized by EnsureVersionChainRegistry. Nil when never wired;
	// callers must guard with EnsureVersionChainRegistry.
	versionChainReg *VersionChainRegistry
}

// NewWorkTree creates an in-memory work tree.
func NewWorkTree() *WorkTree {
	return &WorkTree{
		items:             make(map[string]map[string]*WorkItem),
		maxDecomposeDepth: DefaultMaxDecomposeDepth,
	}
}

// EnsureVersionChainRegistry returns the worktree's VersionChainRegistry,
// creating one on first access (PR-C AC13: CoW VersionChain embedded).
//
// This is a lazily-initialised additive helper. Pre-PR-C callers that never
// touched the field are unaffected.
func (t *WorkTree) EnsureVersionChainRegistry() *VersionChainRegistry {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.versionChainReg == nil {
		t.versionChainReg = NewVersionChainRegistry()
	}
	return t.versionChainReg
}

// SetStore wires optional disk persistence.
//
// RH-D2-CC-04 (DM-20260630-013 T-P2-11.3): SetStore previously wrote
// t.store without holding t.mu, which is a data race if called
// concurrently with any other method (the rest of the file locks
// before reading/writing). The contract is still "bootstrap-only":
// SetStore is expected to fire once at startup before any session
// exists, and concurrent callers will block briefly on the write
// lock rather than tearing the store pointer.
//
// Returns the previously-installed store (nil if none) for symmetry
// with the original assignment; the caller can use it to flush
// pending writes before swap.
func (t *WorkTree) SetStore(store WorkItemStore) WorkItemStore {
	t.mu.Lock()
	defer t.mu.Unlock()
	prev := t.store
	t.store = store
	return prev
}

func (t *WorkTree) ensureSessionLocked(sessionID string) {
	if t.items[sessionID] != nil {
		return
	}
	t.items[sessionID] = make(map[string]*WorkItem)
	if t.store == nil {
		return
	}
	loaded, err := t.store.Load(sessionID)
	if err != nil || len(loaded) == 0 {
		return
	}
	for _, item := range loaded {
		if item != nil {
			t.items[sessionID][item.ID] = item
		}
	}
}

func (t *WorkTree) persistLocked(sessionID string) {
	if t.store == nil {
		return
	}
	items := make([]*WorkItem, 0, len(t.items[sessionID]))
	for _, item := range t.items[sessionID] {
		if item.Ephemeral {
			continue
		}
		items = append(items, item)
	}
	_ = t.store.Save(sessionID, items)
}

func (t *WorkTree) touch(item *WorkItem) {
	item.UpdatedAt = time.Now()
}

// DiskStore returns the disk backing store when configured.
func (t *WorkTree) DiskStore() *DiskWorkItemStore {
	if t == nil {
		return nil
	}
	s, _ := t.store.(*DiskWorkItemStore)
	return s
}

// FindInMemory locates an item across loaded in-memory sessions.
func (t *WorkTree) FindInMemory(itemID string) (*WorkItem, string, bool) {
	if t == nil || itemID == "" {
		return nil, "", false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for sid, items := range t.items {
		if item, ok := items[itemID]; ok {
			return item, sid, true
		}
	}
	return nil, "", false
}

func (t *WorkTree) checkMutable(item *WorkItem) error {
	if item != nil && item.Locked {
		return ErrWorkItemLocked
	}
	return nil
}

// EnsureSession loads or initializes the session item map.
func (t *WorkTree) EnsureSession(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)
}

// MaxDecomposeDepth returns configured max tree depth for decomposition.
func (t *WorkTree) MaxDecomposeDepth() int {
	if t == nil || t.maxDecomposeDepth <= 0 {
		return DefaultMaxDecomposeDepth
	}
	return t.maxDecomposeDepth
}

// EnsureGoal returns the session root goal, creating one if absent.
//
// When an existing goal is found and is not locked, its Title and
// Directive are refreshed to reflect the latest user directive (so the
// focus hint and downstream prompts track the current intent instead of
// the very first message). Locked goals (terminal status set by
// UpdateStatus) are preserved as-is and a fresh root goal is created
// above the historical work — the original goal's children stay attached
// to the original parent.
//
// Context-bleed fix (2026-06-20): the previous implementation always
// returned the first goal regardless of new directives, so "Work focus:
// [goal] 你好" stayed stuck across turns even when the user pivoted to a
// different intent, and the LLM treated later instructions as
// continuations of the first message.
func (t *WorkTree) EnsureGoal(sessionID, directive string) (*WorkItem, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)

	for _, item := range t.items[sessionID] {
		if item.Kind == WorkKindGoal && item.ParentID == "" {
			if item.Locked {
				continue
			}
			if item.Directive == directive {
				return item, nil
			}
			title := directive
			if len(title) > 80 {
				title = title[:80] + "..."
			}
			item.Title = title
			item.Directive = directive
			t.touch(item)
			t.persistLocked(sessionID)
			return item, nil
		}
	}

	title := directive
	if len(title) > 80 {
		title = title[:80] + "..."
	}
	item := NewWorkItem(WorkKindGoal, title, directive)
	item.Uncertainty = DefaultUncertaintyDecomposeThreshold
	t.assignSemanticIDLocked(sessionID, item)
	t.items[sessionID][item.ID] = item
	t.persistLocked(sessionID)
	return item, nil
}

// Create adds a new work item to the session tree.
func (t *WorkTree) Create(sessionID string, in CreateWorkItemInput) (*WorkItem, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)

	if in.ParentID != "" {
		parent, ok := t.items[sessionID][in.ParentID]
		if !ok {
			return nil, fmt.Errorf("parent not found: %s", in.ParentID)
		}
		if err := t.checkMutable(parent); err != nil {
			return nil, err
		}
		if t.depthLocked(sessionID, in.ParentID)+1 > t.MaxDecomposeDepth() {
			return nil, ErrDecomposeDepthExceeded
		}
	}

	item := NewWorkItem(in.Kind, in.Title, in.Directive)
	item.ParentID = in.ParentID
	item.Uncertainty = in.Uncertainty
	item.Policy = in.Policy
	item.Ephemeral = in.Ephemeral
	item.SourceSession = in.SourceSession
	if in.ToolFilter != nil {
		item.ToolFilter = append([]string(nil), in.ToolFilter...)
	}
	if item.Policy == "" {
		item.Policy = ExecPolicySync
	}

	t.assignSemanticIDLocked(sessionID, item)
	t.items[sessionID][item.ID] = item
	t.persistLocked(sessionID)
	return item, nil
}

// Get retrieves a work item by ID.
func (t *WorkTree) Get(sessionID, itemID string) (*WorkItem, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)
	item, ok := t.items[sessionID][itemID]
	return item, ok
}

// GetByRunRef finds a work item by run reference.
func (t *WorkTree) GetByRunRef(sessionID, runRef string) (*WorkItem, bool) {
	if runRef == "" {
		return nil, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)
	for _, item := range t.items[sessionID] {
		if item.RunRef == runRef {
			return item, true
		}
	}
	return nil, false
}

// List returns all work items for a session (includes ephemeral in memory).
func (t *WorkTree) List(sessionID string) []*WorkItem {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)

	out := make([]*WorkItem, 0, len(t.items[sessionID]))
	for _, item := range t.items[sessionID] {
		out = append(out, item)
	}
	return out
}

// ListRoots returns top-level items.
func (t *WorkTree) ListRoots(sessionID string) []*WorkItem {
	all := t.List(sessionID)
	var roots []*WorkItem
	for _, item := range all {
		if item.ParentID == "" {
			roots = append(roots, item)
		}
	}
	return roots
}

// ListChildren returns direct children of parentID.
func (t *WorkTree) ListChildren(sessionID, parentID string) []*WorkItem {
	all := t.List(sessionID)
	var children []*WorkItem
	for _, item := range all {
		if item.ParentID == parentID {
			children = append(children, item)
		}
	}
	return children
}

// ListSubtree returns root and all descendants in depth-first order.
func (t *WorkTree) ListSubtree(sessionID, rootID string) []*WorkItem {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)

	root, ok := t.items[sessionID][rootID]
	if !ok {
		return nil
	}
	return t.collectSubtreeLocked(sessionID, root)
}

func (t *WorkTree) collectSubtreeLocked(sessionID string, node *WorkItem) []*WorkItem {
	out := []*WorkItem{node}
	for _, item := range t.items[sessionID] {
		if item.ParentID == node.ID {
			out = append(out, t.collectSubtreeLocked(sessionID, item)...)
		}
	}
	return out
}

// Ancestors returns the chain from root to item (inclusive).
func (t *WorkTree) Ancestors(sessionID, itemID string) []*WorkItem {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)
	return t.ancestorsLocked(sessionID, itemID)
}

func (t *WorkTree) ancestorsLocked(sessionID, itemID string) []*WorkItem {
	item, ok := t.items[sessionID][itemID]
	if !ok {
		return nil
	}
	var chain []*WorkItem
	for cur := item; cur != nil; {
		chain = append([]*WorkItem{cur}, chain...)
		if cur.ParentID == "" {
			break
		}
		cur, ok = t.items[sessionID][cur.ParentID]
		if !ok {
			break
		}
	}
	return chain
}

// Depth returns the depth of item in the tree (root=0).
func (t *WorkTree) Depth(sessionID, itemID string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)
	return t.depthLocked(sessionID, itemID)
}

func (t *WorkTree) depthLocked(sessionID, itemID string) int {
	ancestors := t.ancestorsLocked(sessionID, itemID)
	if len(ancestors) == 0 {
		return 0
	}
	return len(ancestors) - 1
}

// EnsureSemanticID backfills semantic_id for legacy persisted items.
func (t *WorkTree) EnsureSemanticID(sessionID string, item *WorkItem) {
	if t == nil || item == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)
	t.ensureSemanticIDLocked(sessionID, item)
	if stored, ok := t.items[sessionID][item.ID]; ok && stored != nil {
		item.SemanticID = stored.SemanticID
	}
}

func (t *WorkTree) assignSemanticIDLocked(sessionID string, item *WorkItem) {
	if item == nil {
		return
	}
	if strings.TrimSpace(item.SemanticID) != "" {
		return
	}
	depth := 0
	if item.ParentID != "" {
		depth = t.depthLocked(sessionID, item.ParentID) + 1
	}
	sibling := t.nextSiblingIndexLocked(sessionID, item.ParentID)
	item.SemanticID = BuildSemanticID(depth, sibling, item.Kind)
}

func (t *WorkTree) ensureSemanticIDLocked(sessionID string, item *WorkItem) {
	if item == nil || strings.TrimSpace(item.SemanticID) != "" {
		return
	}
	depth := 0
	if item.ParentID != "" {
		depth = t.depthLocked(sessionID, item.ParentID) + 1
	}
	// Item is already in the map — count peer index among siblings, not
	// nextSiblingIndexLocked (which would include self and skew the index).
	sibling := t.siblingIndexAmongPeersLocked(sessionID, item)
	item.SemanticID = BuildSemanticID(depth, sibling, item.Kind)
	t.touch(item)
}

func (t *WorkTree) nextSiblingIndexLocked(sessionID, parentID string) int {
	n := 0
	for _, it := range t.items[sessionID] {
		if it == nil || it.Ephemeral {
			continue
		}
		if it.Kind == WorkKindChecklist && it.Ephemeral {
			continue
		}
		if it.ParentID == parentID {
			n++
		}
	}
	return n
}

// SiblingIndex returns the 0-based peer index among non-ephemeral siblings.
func (t *WorkTree) SiblingIndex(sessionID, itemID string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)
	item, ok := t.items[sessionID][itemID]
	if !ok || item == nil {
		return 0
	}
	return t.siblingIndexAmongPeersLocked(sessionID, item)
}

func (t *WorkTree) siblingIndexAmongPeersLocked(sessionID string, item *WorkItem) int {
	if item == nil {
		return 0
	}
	peers := t.peersLocked(sessionID, item.ParentID)
	for i, p := range peers {
		if p != nil && p.ID == item.ID {
			return i
		}
	}
	return 0
}

func (t *WorkTree) peersLocked(sessionID, parentID string) []*WorkItem {
	var peers []*WorkItem
	for _, it := range t.items[sessionID] {
		if it == nil || it.Ephemeral {
			continue
		}
		if it.Kind == WorkKindChecklist && it.Ephemeral {
			continue
		}
		if it.ParentID == parentID {
			peers = append(peers, it)
		}
	}
	sort.Slice(peers, func(i, j int) bool {
		if peers[i].CreatedAt.Equal(peers[j].CreatedAt) {
			return peers[i].ID < peers[j].ID
		}
		return peers[i].CreatedAt.Before(peers[j].CreatedAt)
	})
	return peers
}

// UpsertChecklist replaces ephemeral checklist children under parent.
func (t *WorkTree) UpsertChecklist(sessionID, parentID string, entries []ChecklistEntry) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)

	parent, ok := t.items[sessionID][parentID]
	if !ok {
		return fmt.Errorf("parent not found: %s", parentID)
	}
	if err := t.checkMutable(parent); err != nil {
		return err
	}

	for id, item := range t.items[sessionID] {
		if item.ParentID == parentID && item.Kind == WorkKindChecklist && item.Ephemeral {
			delete(t.items[sessionID], id)
		}
	}

	for _, e := range entries {
		status := e.Status
		if status == "" {
			status = TaskStatusPending
		}
		title := e.Content
		if title == "" {
			title = e.ActiveForm
		}
		item := NewWorkItem(WorkKindChecklist, title, e.Content)
		item.ParentID = parentID
		item.Ephemeral = true
		item.Status = status
		t.items[sessionID][item.ID] = item
	}
	t.persistLocked(sessionID)
	return nil
}

// PromoteChecklist copies ephemeral checklist items to persistent implement children.
func (t *WorkTree) PromoteChecklist(sessionID, parentID string) ([]*WorkItem, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)

	var promoted []*WorkItem
	for _, item := range t.items[sessionID] {
		if item.ParentID != parentID || item.Kind != WorkKindChecklist || !item.Ephemeral {
			continue
		}
		child := NewWorkItem(WorkKindImplement, item.Title, item.Directive)
		child.ParentID = parentID
		child.BlockedBy = append([]string(nil), item.BlockedBy...)
		t.items[sessionID][child.ID] = child
		promoted = append(promoted, child)
	}
	t.persistLocked(sessionID)
	return promoted, nil
}

// SetRunRef attaches a run registry reference.
func (t *WorkTree) SetRunRef(sessionID, itemID, runRef string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)

	item, ok := t.items[sessionID][itemID]
	if !ok {
		return fmt.Errorf("work item not found: %s", itemID)
	}
	if err := t.checkMutable(item); err != nil {
		return err
	}
	item.RunRef = runRef
	t.touch(item)
	t.persistLocked(sessionID)
	return nil
}

// UpdateStatus updates item status with state-machine validation.
func (t *WorkTree) UpdateStatus(sessionID, itemID string, status TaskStatus) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)

	item, ok := t.items[sessionID][itemID]
	if !ok {
		return fmt.Errorf("work item not found: %s", itemID)
	}
	if !IsLegalTransition(item.Status, status) {
		return fmt.Errorf("%w: from %s to %s", ErrIllegalTransition, item.Status, status)
	}
	if item.Locked && status != item.Status {
		return ErrWorkItemLocked
	}
	item.Status = status
	if isTerminalStatus(status) {
		item.Locked = true
	}
	t.touch(item)
	t.persistLocked(sessionID)
	return nil
}

func isTerminalStatus(s TaskStatus) bool {
	return s == TaskStatusCompleted || s == TaskStatusFailed || s == TaskStatusCancelled
}

// SetOwner assigns an owner to a work item.
func (t *WorkTree) SetOwner(sessionID, itemID, owner string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)

	item, ok := t.items[sessionID][itemID]
	if !ok {
		return fmt.Errorf("work item not found: %s", itemID)
	}
	if err := t.checkMutable(item); err != nil {
		return err
	}
	item.Owner = owner
	t.touch(item)
	t.persistLocked(sessionID)
	return nil
}

// SetUncertainty updates the uncertainty score.
func (t *WorkTree) SetUncertainty(sessionID, itemID string, u float64) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)

	item, ok := t.items[sessionID][itemID]
	if !ok {
		return fmt.Errorf("work item not found: %s", itemID)
	}
	if err := t.checkMutable(item); err != nil {
		return err
	}
	item.Uncertainty = u
	t.touch(item)
	t.persistLocked(sessionID)
	return nil
}

// SetContextScopeID binds a WorkItem to its ContextScope (CG1).
func (t *WorkTree) SetContextScopeID(sessionID, itemID, scopeID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)
	item, ok := t.items[sessionID][itemID]
	if !ok {
		return fmt.Errorf("work item not found: %s", itemID)
	}
	if err := t.checkMutable(item); err != nil {
		return err
	}
	item.ContextScopeID = scopeID
	t.touch(item)
	t.persistLocked(sessionID)
	return nil
}

// SetContextPolicy writes materialized link policy on a WorkItem (OQ-CG-4).
func (t *WorkTree) SetContextPolicy(sessionID, itemID string, policy ContextLinkKind) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)
	item, ok := t.items[sessionID][itemID]
	if !ok {
		return fmt.Errorf("work item not found: %s", itemID)
	}
	if err := t.checkMutable(item); err != nil {
		return err
	}
	item.ContextPolicy = policy
	t.touch(item)
	t.persistLocked(sessionID)
	return nil
}

// AddDependency adds a blocked-by edge; rejects cycles.
func (t *WorkTree) AddDependency(sessionID, itemID, blockedByID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)

	item, ok := t.items[sessionID][itemID]
	if !ok {
		return fmt.Errorf("work item not found: %s", itemID)
	}
	if err := t.checkMutable(item); err != nil {
		return err
	}
	if itemID == blockedByID {
		return ErrDependencyCycle
	}
	if t.wouldCycleLocked(sessionID, itemID, blockedByID) {
		return ErrDependencyCycle
	}

	item.BlockedBy = append(item.BlockedBy, blockedByID)
	if blocker, ok := t.items[sessionID][blockedByID]; ok {
		blocker.Blocks = append(blocker.Blocks, itemID)
	}

	t.touch(item)
	t.persistLocked(sessionID)
	return nil
}

func (t *WorkTree) wouldCycleLocked(sessionID, itemID, blockedByID string) bool {
	visited := map[string]bool{itemID: true}
	queue := []string{blockedByID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == itemID {
			return true
		}
		if visited[cur] {
			continue
		}
		visited[cur] = true
		node, ok := t.items[sessionID][cur]
		if !ok {
			continue
		}
		for _, dep := range node.BlockedBy {
			queue = append(queue, dep)
		}
	}
	return false
}

// Remove deletes an item, its descendants, and dependency references.
func (t *WorkTree) Remove(sessionID, itemID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)

	item, ok := t.items[sessionID][itemID]
	if !ok {
		return fmt.Errorf("work item not found: %s", itemID)
	}
	if err := t.checkMutable(item); err != nil {
		return err
	}

	toDelete := map[string]bool{itemID: true}
	for _, sub := range t.collectSubtreeLocked(sessionID, item) {
		toDelete[sub.ID] = true
	}

	for id := range toDelete {
		for _, other := range t.items[sessionID] {
			filtered := other.BlockedBy[:0]
			for _, blocked := range other.BlockedBy {
				if blocked != id {
					filtered = append(filtered, blocked)
				}
			}
			other.BlockedBy = filtered
		}
		delete(t.items[sessionID], id)
	}

	t.persistLocked(sessionID)
	return nil
}

// GetReadyItems returns pending items whose blockers are all completed.
func (t *WorkTree) GetReadyItems(sessionID string) []*WorkItem {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)

	var ready []*WorkItem
	for _, item := range t.items[sessionID] {
		if item.Status != TaskStatusPending {
			if item.Status != TaskStatusInProgress || !item.NeedsRollup {
				continue
			}
		}
		if item.Ephemeral && item.Kind == WorkKindChecklist {
			continue
		}
		if !t.depsSatisfiedLocked(sessionID, item) {
			continue
		}
		ready = append(ready, item)
	}
	return ready
}

func (t *WorkTree) depsSatisfiedLocked(sessionID string, item *WorkItem) bool {
	for _, blockerID := range item.BlockedBy {
		blocker, ok := t.items[sessionID][blockerID]
		if !ok || blocker.Status != TaskStatusCompleted {
			return false
		}
	}
	return true
}

var kindFocusPriority = map[WorkKind]int{
	WorkKindVerify:    0,
	WorkKindImplement: 1,
	WorkKindExplore:   2,
	WorkKindChecklist: 3,
	WorkKindPlan:      4,
	WorkKindGoal:      5,
	WorkKindShell:     6,
	WorkKindAgent:     7,
}

// GetFocus selects the highest-priority ready work item.
func (t *WorkTree) GetFocus(sessionID string) (*WorkItem, error) {
	ready := t.GetReadyItems(sessionID)
	if len(ready) == 0 {
		return nil, nil
	}
	sort.SliceStable(ready, func(i, j int) bool {
		a, b := ready[i], ready[j]
		if a.NeedsRollup != b.NeedsRollup {
			return a.NeedsRollup
		}
		pa, pb := kindFocusPriority[a.Kind], kindFocusPriority[b.Kind]
		if pa != pb {
			return pa < pb
		}
		if a.Uncertainty != b.Uncertainty {
			return a.Uncertainty > b.Uncertainty
		}
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		return a.ID < b.ID
	})
	return ready[0], nil
}

// SetNeedsRollup sets the rollup gate flag (DM-20260627-001).
func (t *WorkTree) SetNeedsRollup(sessionID, itemID string, v bool) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)
	item, ok := t.items[sessionID][itemID]
	if !ok {
		return fmt.Errorf("work item not found: %s", itemID)
	}
	item.NeedsRollup = v
	t.touch(item)
	t.persistLocked(sessionID)
	return nil
}

// ReopenForRollup transitions a terminal locked item to pending for rollup R2+ (I3-Rollup).
func (t *WorkTree) ReopenForRollup(sessionID, itemID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)
	item, ok := t.items[sessionID][itemID]
	if !ok {
		return fmt.Errorf("work item not found: %s", itemID)
	}
	if !item.NeedsRollup {
		return errWorkItem("reopen for rollup: needs_rollup is false")
	}
	if item.Status != TaskStatusFailed && item.Status != TaskStatusCompleted {
		return errWorkItem("reopen for rollup: item not terminal")
	}
	item.Status = TaskStatusPending
	item.Locked = false
	item.RoundPhase = RoundPhaseIdle
	t.touch(item)
	t.persistLocked(sessionID)
	return nil
}

// ClearSession removes all items for a session.
func (t *WorkTree) ClearSession(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.items, sessionID)
	if store, ok := t.store.(*DiskWorkItemStore); ok && store != nil {
		p := store.path(sessionID)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			slog.Warn("worktree: clear session disk file failed; in-memory state cleared",
				"session_id", sessionID,
				"path", p,
				"error", err)
		}
	}
}

// TreeNode is a serializable tree view for task_list --format=tree.
type TreeNode struct {
	Item     *WorkItem  `json:"item"`
	Children []TreeNode `json:"children,omitempty"`
}

// BuildTree returns hierarchical nodes under optional root (all roots if empty).
func (t *WorkTree) BuildTree(sessionID, rootID string) []TreeNode {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)

	var build func(id string) TreeNode
	build = func(id string) TreeNode {
		item := t.items[sessionID][id]
		node := TreeNode{Item: item}
		for _, child := range t.items[sessionID] {
			if child.ParentID == id {
				node.Children = append(node.Children, build(child.ID))
			}
		}
		sort.Slice(node.Children, func(i, j int) bool {
			return node.Children[i].Item.CreatedAt.Before(node.Children[j].Item.CreatedAt)
		})
		return node
	}

	var roots []string
	if rootID != "" {
		roots = []string{rootID}
	} else {
		for id, item := range t.items[sessionID] {
			if item.ParentID == "" {
				roots = append(roots, id)
			}
		}
		sort.Strings(roots)
	}

	out := make([]TreeNode, 0, len(roots))
	for _, id := range roots {
		if _, ok := t.items[sessionID][id]; ok {
			out = append(out, build(id))
		}
	}
	return out
}
