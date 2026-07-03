package workmodel

import "fmt"

// IncrementInlineRetriesAtMaxDepth bumps the leaf inline counter (CC-1.2).
func (t *WorkTree) IncrementInlineRetriesAtMaxDepth(sessionID, itemID string) error {
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
	item.InlineRetriesAtMaxDepth++
	t.touch(item)
	t.persistLocked(sessionID)
	return nil
}

// ResetInlineRetriesAtMaxDepth clears the leaf inline counter.
func (t *WorkTree) ResetInlineRetriesAtMaxDepth(sessionID, itemID string) error {
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
	if item.InlineRetriesAtMaxDepth == 0 {
		return nil
	}
	item.InlineRetriesAtMaxDepth = 0
	t.touch(item)
	t.persistLocked(sessionID)
	return nil
}

// IsInlineRetryExhaustedAtMaxDepth reports whether inline deliverable retries
// are exhausted (CC-1.2 budget applies at max depth and gates stagnation).
func IsInlineRetryExhaustedAtMaxDepth(item *WorkItem, _ int) bool {
	return IsDeliverableInlineBudgetExhausted(item)
}
