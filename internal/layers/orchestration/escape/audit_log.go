// EscapeAuditLog (DM-20260625-003, PR-V5.3)
//
// 关键设计 (doc 38 §21.5):
//   - Record(ctx, decisions, final) 写 audit log
//   - AuditLevel 0/1/2 区分:
//     * 0 = 无审计 (default, Continue 路径)
//     * 1 = 记录 (ForceExit / Escalate 路径)
//     * 2 = 完整审计 (AbortWithAudit / 终态)
//   - dev 默认 in-memory; 生产 V5.4+ 接入 D5 observability
package escape

import (
	"sync"
	"time"
)

// AuditEntry is a single audit log record.
type AuditEntry struct {
	LoopContext       LoopContext
	UpstreamDecisions []EscapeDecision
	Final             EscapeDecision
	RecordedAt        time.Time
}

// EscapeAuditLog is the audit-trail interface.
//
// In-memory implementation provided for dev/test; production should
// wire a structured logger (D5 spans / structured file) via the same
// Record method.
type EscapeAuditLog struct {
	mu      sync.RWMutex
	entries []AuditEntry
}

// NewEscapeAuditLog constructs an empty in-memory audit log.
func NewEscapeAuditLog() *EscapeAuditLog {
	return &EscapeAuditLog{
		entries: make([]AuditEntry, 0),
	}
}

// Record writes a new audit entry. No-op for AuditLevel 0 (silent).
func (l *EscapeAuditLog) Record(loopCtx LoopContext, decisions []EscapeDecision, final EscapeDecision) {
	if final.AuditLevel == 0 {
		return
	}

	l.mu.Lock()
	l.entries = append(l.entries, AuditEntry{
		LoopContext:       loopCtx,
		UpstreamDecisions: append([]EscapeDecision{}, decisions...),
		Final:             final,
		RecordedAt:        nowFunc(),
	})
	l.mu.Unlock()
}

// Entries returns a snapshot of all audit entries (for tests).
func (l *EscapeAuditLog) Entries() []AuditEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]AuditEntry, len(l.entries))
	copy(out, l.entries)
	return out
}

// Len returns the number of recorded entries.
func (l *EscapeAuditLog) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
}

// Clear empties the audit log (test aid).
func (l *EscapeAuditLog) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = l.entries[:0]
}