package workmodel

import (
	"github.com/devrix/devrix/internal/shared/types"
)

const ExitReasonChildPromoted = "child_promoted"

// TryPromoteSingleChildDeliverable completes a decompose parent from its sole
// child's verified structured deliverable, skipping an extra rollup LLM round
// when the child already produced a presentable findings payload (CC-3 shortcut).
func TryPromoteSingleChildDeliverable(sessionID string, tm *TaskManager, parent *WorkItem) bool {
	if tm == nil || parent == nil || parent.ID == "" {
		return false
	}
	got, ok := tm.GetWorkItem(sessionID, parent.ID)
	if !ok || got == nil {
		return false
	}
	parent = got
	if !parentHadDecomposeSpawn(parent) {
		return false
	}
	child := soleNonEphemeralChild(tm, sessionID, parent.ID)
	if child == nil || child.Status != TaskStatusCompleted || child.LastRound == nil {
		return false
	}
	payload := presentableChildDeliverable(child.LastRound)
	if payload == nil {
		return false
	}
	roundNo := 1
	if parent.LastRound != nil {
		roundNo = parent.LastRound.RoundNo + 1
	}
	lr := child.LastRound
	promoted := &WorkItemPipelineRound{
		RoundNo:               roundNo,
		WorkItemID:            parent.ID,
		SessionID:             sessionID,
		VerdictKind:           types.VerdictPass,
		SpawnPolicy:           SpawnNone,
		DeliverableStatus:     DeliverableStatusComplete,
		DeliverableContract:   lr.DeliverableContract,
		DeliverableSchema:     lr.DeliverableSchema,
		StructuredDeliverable: cloneDeliverablePayload(payload),
		ArtifactSummary:       FormatDeliverablePayloadForIM(payload),
		SpawnRationale:          "CC-3: single child deliverable promoted (rollup skipped)",
		ExitReason:              ExitReasonChildPromoted,
	}
	if err := tm.Tree().ApplyPipelineRound(sessionID, parent.ID, promoted, RoundPhaseIdle); err != nil {
		return false
	}
	_ = tm.Tree().SetNeedsRollup(sessionID, parent.ID, false)
	if parent.Status == TaskStatusPending {
		_ = tm.Tree().UpdateStatus(sessionID, parent.ID, TaskStatusInProgress)
	}
	_ = tm.Tree().UpdateStatus(sessionID, parent.ID, TaskStatusCompleted)
	return true
}

// MaybePromotePendingSingleChildRollups finalizes parents that still owe rollup
// but have a single completed child with a presentable deliverable.
func MaybePromotePendingSingleChildRollups(sessionID string, tm *TaskManager) bool {
	if tm == nil {
		return false
	}
	promoted := false
	for _, item := range tm.Tree().List(sessionID) {
		if item == nil || item.Ephemeral || !item.NeedsRollup || IsTerminalStatus(item.Status) {
			continue
		}
		if TryPromoteSingleChildDeliverable(sessionID, tm, item) {
			promoted = true
		}
	}
	return promoted
}

func soleNonEphemeralChild(tm *TaskManager, sessionID, parentID string) *WorkItem {
	if tm == nil {
		return nil
	}
	var sole *WorkItem
	for _, c := range tm.Tree().ListChildren(sessionID, parentID) {
		if c == nil || c.Ephemeral || c.Kind == WorkKindChecklist {
			continue
		}
		if sole != nil {
			return nil
		}
		sole = c
	}
	return sole
}

func presentableChildDeliverable(round *WorkItemPipelineRound) *DeliverablePayload {
	if round == nil {
		return nil
	}
	if round.DeliverableStatus != DeliverableStatusComplete {
		return nil
	}
	if round.StructuredDeliverable != nil && FindingsPayloadPresentable(round.StructuredDeliverable) {
		return round.StructuredDeliverable
	}
	contract := round.DeliverableContract
	if !contract.ContractApplicable() {
		contract = ExpandLegacySchemaToContract(round.DeliverableSchema)
	}
	payload := SalvageDeliverablePayload(round.ArtifactSummary, contract)
	if FindingsPayloadPresentable(payload) {
		return payload
	}
	return nil
}

func cloneDeliverablePayload(p *DeliverablePayload) *DeliverablePayload {
	if p == nil {
		return nil
	}
	out := *p
	out.Findings = append([]DeliverableFinding(nil), p.Findings...)
	return &out
}
