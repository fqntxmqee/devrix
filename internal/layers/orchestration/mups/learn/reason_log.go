// Package learn: ReasonLog is the D7-S2-A50-T08 minimal bridge that closes
// the H6 reason透传 loop:
//
//   verify_exit_reason
//     → ReasonLog.Record (intra-session, sync)
//     → ReputationStore (cross-session, async, H6→H7 reputation equilibrium)
//     → AdaptivePrior (next session, Observe reads)
//
// ReasonLog is distinct from memory.FeedbackMemory (which is the
// LearningAsset in-memory store). ReasonLog is the lightweight audit
// log specifically for the H6 "verdict.Reason 透传" requirement.
//
// DSAFT: D7-S2-A50-T08 (Phase C — Learn ReasonLog minimal接入).
// Change: devrix-mups-tool-classification-and-channel-autonomy (DM-20260701-007).
package learn

import (
	"fmt"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// ReasonEntry is one record of a verify_exit_reason + the tool
// that produced it. Immutable.
type ReasonEntry struct {
	// SessionID — the session that produced the verdict.
	SessionID string
	// ToolName — the tool the LLM was using when the verdict was issued.
	ToolName string
	// EmissionClass — the tool's emission class.
	EmissionClass contracts.EmissionClass
	// VerifyExitReason — the canonical reason code.
	VerifyExitReason string
	// IterationsUsed — the iter count from the channel's Finalize.
	IterationsUsed int
	// Timestamp — when the entry was recorded.
	Timestamp time.Time
	// TaskKind — the inferred user task kind.
	TaskKind string
}

// ReasonLog is an in-process store of recent ReasonEntry records.
// It is the minimal Phase C surface; the full DriftAudit (Phase E)
// will add cross-session reputation queries.
type ReasonLog struct {
	mu      sync.RWMutex
	entries []ReasonEntry
	maxEntries int
}

// NewReasonLog constructs a ReasonLog with the given capacity.
// 0 means default (1000).
func NewReasonLog(maxEntries int) *ReasonLog {
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	return &ReasonLog{
		entries:    make([]ReasonEntry, 0, maxEntries),
		maxEntries: maxEntries,
	}
}

// Record appends a ReasonEntry. FIFO eviction at capacity.
func (m *ReasonLog) Record(entry ReasonEntry) error {
	if entry.SessionID == "" {
		return fmt.Errorf("learn: ReasonLog.Record: SessionID is empty")
	}
	if entry.VerifyExitReason == "" {
		return fmt.Errorf("learn: ReasonLog.Record: VerifyExitReason is empty")
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.entries) >= m.maxEntries {
		m.entries = m.entries[1:]
	}
	m.entries = append(m.entries, entry)
	return nil
}

// RecordFromVerdict is a convenience wrapper that constructs a
// ReasonEntry from a sessionID + tool name + emission class +
// verify exit reason + iter count + task kind. Used by the D7
// session_complete meta透传 path.
func (m *ReasonLog) RecordFromVerdict(sessionID, toolName string, ec contracts.EmissionClass,
	exitReason, taskKind string, iterUsed int) error {
	return m.Record(ReasonEntry{
		SessionID:        sessionID,
		ToolName:         toolName,
		EmissionClass:    ec,
		VerifyExitReason: exitReason,
		IterationsUsed:   iterUsed,
		TaskKind:         taskKind,
		Timestamp:        time.Now(),
	})
}

// RecentByTool returns the most recent n entries for a given tool name.
func (m *ReasonLog) RecentByTool(toolName string, n int) []ReasonEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if n <= 0 || len(m.entries) == 0 {
		return nil
	}
	out := make([]ReasonEntry, 0, n)
	for i := len(m.entries) - 1; i >= 0 && len(out) < n; i-- {
		if m.entries[i].ToolName == toolName {
			out = append(out, m.entries[i])
		}
	}
	return out
}

// DriftRate returns the fraction of recent entries for a given tool
// that have a non-OK exit reason. A high drift rate signals that the
// tool's declared EmissionClass may be mis-tagged (cheap talk risk;
// H10 / Codex §3.1).
func (m *ReasonLog) DriftRate(toolName string, n int) float64 {
	recent := m.RecentByTool(toolName, n)
	if len(recent) == 0 {
		return 0
	}
	var bad int
	for _, e := range recent {
		if e.VerifyExitReason != "ok" {
			bad++
		}
	}
	return float64(bad) / float64(len(recent))
}

// Len returns the current number of entries (for tests + observability).
func (m *ReasonLog) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries)
}

// Reset clears all entries (for tests).
func (m *ReasonLog) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = m.entries[:0]
}
