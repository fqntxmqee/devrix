package workmodel

import (
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/google/uuid"
)

// ContextScope is the ContextGraph partition bound 1:1 to a non-ephemeral WorkItem (CG1).
type ContextScope struct {
	ID           string            `json:"id"`
	SessionID    string            `json:"session_id"`
	WorkItemID   string            `json:"work_item_id"`
	SidechainKey string            `json:"sidechain_key"`
	PersistScope plan.PersistScope `json:"persist_scope,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}

// ContextScopeSidechainKey returns the D2 sidechain partition key for a WorkItem.
func ContextScopeSidechainKey(workItemID string) string {
	return "wi_" + workItemID
}

// NewContextScope creates a scope for a WorkItem. Ephemeral items must not call this (I-CG3).
func NewContextScope(sessionID, workItemID string, persist plan.PersistScope) *ContextScope {
	now := time.Now().UTC()
	if persist == "" {
		persist = plan.PersistSession
	}
	return &ContextScope{
		ID:           uuid.NewString(),
		SessionID:    sessionID,
		WorkItemID:   workItemID,
		SidechainKey: ContextScopeSidechainKey(workItemID),
		PersistScope: persist,
		CreatedAt:    now,
	}
}

// DefaultPersistScopeForKind suggests PersistScope per design §4.
func DefaultPersistScopeForKind(kind WorkKind) plan.PersistScope {
	switch kind {
	case WorkKindVerify:
		return plan.PersistTransient
	case WorkKindChecklist:
		return plan.PersistTransient
	default:
		return plan.PersistSession
	}
}

// ValidateContextScope checks invariant I-CG1 fields.
func ValidateContextScope(scope *ContextScope) error {
	if scope == nil {
		return errWorkItem("context scope is nil")
	}
	if scope.ID == "" || scope.SessionID == "" || scope.WorkItemID == "" {
		return errWorkItem(fmt.Sprintf("context scope %q missing id/session/work_item", scope.ID))
	}
	if scope.SidechainKey != ContextScopeSidechainKey(scope.WorkItemID) {
		return errWorkItem(fmt.Sprintf("context scope %q sidechain key mismatch", scope.ID))
	}
	if scope.PersistScope != "" && !scope.PersistScope.Valid() {
		return errWorkItem(fmt.Sprintf("context scope %q invalid persist_scope %q", scope.ID, scope.PersistScope))
	}
	return nil
}
