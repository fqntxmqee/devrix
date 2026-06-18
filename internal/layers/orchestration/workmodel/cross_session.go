package workmodel

import (
	"errors"
)

// FindWorkItemAcrossSessions scans disk store for an item ID (Phase 7 cross-session query).
func (m *TaskManager) FindWorkItemAcrossSessions(itemID string) (*WorkItem, string, bool) {
	if m == nil || itemID == "" {
		return nil, "", false
	}
	if store := m.tree.DiskStore(); store != nil {
		if item, sid, ok := store.FindByItemID(itemID); ok {
			return item, sid, true
		}
	}
	return m.tree.FindInMemory(itemID)
}

// QueryHistoricalWorkItem returns read-only status for cross-session queries (AC30).
func (m *TaskManager) QueryHistoricalWorkItem(itemID string) (*WorkItem, string, error) {
	item, sessionID, ok := m.FindWorkItemAcrossSessions(itemID)
	if !ok {
		return nil, "", ErrWorkItemNotFound
	}
	return item, sessionID, nil
}

// ErrWorkItemNotFound indicates no work item exists for the id.
var ErrWorkItemNotFound = errors.New("work item not found")

// InheritFromSession copies a work item into a new session (AC32 baseline).
func (m *TaskManager) InheritFromSession(sourceSession, targetSession, rootItemID string) (*WorkItem, error) {
	src, ok := m.GetWorkItem(sourceSession, rootItemID)
	if !ok {
		item, sid, found := m.FindWorkItemAcrossSessions(rootItemID)
		if !found {
			return nil, ErrWorkItemNotFound
		}
		src = item
		sourceSession = sid
	}
	goal, err := m.EnsureGoal(targetSession, src.Directive)
	if err != nil {
		return nil, err
	}
	return m.CreateWorkItem(targetSession, CreateWorkItemInput{
		ParentID:      goal.ID,
		Kind:          src.Kind,
		Title:         src.Title,
		Directive:     src.Directive,
		SourceSession: sourceSession,
	})
}
