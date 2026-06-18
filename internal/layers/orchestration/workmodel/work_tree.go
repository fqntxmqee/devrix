package workmodel

import (
	"fmt"
	"os"
	"sort"
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
}

// NewWorkTree creates an in-memory work tree.
func NewWorkTree() *WorkTree {
	return &WorkTree{
		items:             make(map[string]map[string]*WorkItem),
		maxDecomposeDepth: DefaultMaxDecomposeDepth,
	}
}

// SetStore wires optional disk persistence.
func (t *WorkTree) SetStore(store WorkItemStore) {
	t.store = store
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

// EnsureGoal returns the session root goal, creating one if absent.
func (t *WorkTree) EnsureGoal(sessionID, directive string) (*WorkItem, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureSessionLocked(sessionID)

	for _, item := range t.items[sessionID] {
		if item.Kind == WorkKindGoal && item.ParentID == "" {
			return item, nil
		}
	}

	title := directive
	if len(title) > 80 {
		title = title[:80] + "..."
	}
	item := NewWorkItem(WorkKindGoal, title, directive)
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
	}

	item := NewWorkItem(in.Kind, in.Title, in.Directive)
	item.ParentID = in.ParentID
	item.Uncertainty = in.Uncertainty
	item.Policy = in.Policy
	item.Ephemeral = in.Ephemeral
	item.SourceSession = in.SourceSession
	if item.Policy == "" {
		item.Policy = ExecPolicySync
	}

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
	ancestors := t.Ancestors(sessionID, itemID)
	if len(ancestors) == 0 {
		return 0
	}
	return len(ancestors) - 1
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

// ClearSession removes all items for a session.
func (t *WorkTree) ClearSession(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.items, sessionID)
	if store, ok := t.store.(*DiskWorkItemStore); ok && store != nil {
		_ = os.Remove(store.path(sessionID))
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
