package workmodel

import "time"

// PeerStatusSignal is a terminal sibling status line for cohort sharing (D7-S16-A64).
type PeerStatusSignal struct {
	WorkItemID string    `json:"work_item_id"`
	Verdict    string    `json:"verdict,omitempty"`
	Summary    string    `json:"summary,omitempty"`
	RecordedAt time.Time `json:"recorded_at,omitempty"`
}

// MinCohortSizeForPeerStatus is the opt-in threshold (OQ-LC-5).
const MinCohortSizeForPeerStatus = 3

// RecordPeerStatusOnTerminal appends a peer status signal when a sibling WI completes.
func (m *TaskManager) RecordPeerStatusOnTerminal(sessionID string, item *WorkItem) {
	if m == nil || item == nil || item.ParentID == "" || !IsTerminalStatus(item.Status) {
		return
	}
	siblings := m.Tree().ListChildren(sessionID, item.ParentID)
	nonTrivial := 0
	for _, s := range siblings {
		if s == nil || (s.Kind == WorkKindChecklist && s.Ephemeral) {
			continue
		}
		nonTrivial++
	}
	if nonTrivial < MinCohortSizeForPeerStatus {
		return
	}
	summary := ""
	verdict := ""
	if item.LastRound != nil {
		summary = TruncateArtifactSummary(item.LastRound.ArtifactSummary, 240)
		verdict = string(item.LastRound.VerdictKind)
	}
	m.appendPeerStatus(sessionID, item.ParentID, PeerStatusSignal{
		WorkItemID: item.ID,
		Verdict:    verdict,
		Summary:    summary,
		RecordedAt: time.Now(),
	})
}

func (m *TaskManager) appendPeerStatus(sessionID, parentID string, sig PeerStatusSignal) {
	sd := m.contextData(sessionID)
	if sd == nil || parentID == "" || sig.WorkItemID == "" {
		return
	}
	key := cohortKey(sessionID, parentID)
	sd.mu.Lock()
	defer sd.mu.Unlock()
	if sd.peerStatus == nil {
		sd.peerStatus = make(map[string][]PeerStatusSignal)
	}
	sd.peerStatus[key] = append(sd.peerStatus[key], sig)
}

// PeerStatusSignalsForCohort returns recorded peer status lines for siblings under parent.
func (m *TaskManager) PeerStatusSignalsForCohort(sessionID, parentID string) []PeerStatusSignal {
	sd := m.contextData(sessionID)
	if sd == nil || parentID == "" {
		return nil
	}
	key := cohortKey(sessionID, parentID)
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	if len(sd.peerStatus[key]) == 0 {
		return nil
	}
	out := make([]PeerStatusSignal, len(sd.peerStatus[key]))
	copy(out, sd.peerStatus[key])
	return out
}

// PeerStatusLines formats cohort peer signals for Materialize inject.
func PeerStatusLines(signals []PeerStatusSignal) []string {
	if len(signals) == 0 {
		return nil
	}
	lines := make([]string, 0, len(signals))
	for _, s := range signals {
		line := "peer_status: wi=" + s.WorkItemID
		if s.Verdict != "" {
			line += " verdict=" + s.Verdict
		}
		if s.Summary != "" {
			line += " summary=" + s.Summary
		}
		lines = append(lines, line)
	}
	return lines
}
