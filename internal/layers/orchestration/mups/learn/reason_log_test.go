// T: D7-S2-A50-T08 — ReasonLog record / query / drift tests.
package learn

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// D7-S2-A50-T08: Record stores a ReasonEntry.
func TestReasonLog_Record(t *testing.T) {
	m := NewReasonLog(100)
	err := m.Record(ReasonEntry{
		SessionID:        "sess-1",
		ToolName:         "read_file",
		EmissionClass:    contracts.EC_Probe,
		VerifyExitReason: "deliverable_missing",
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if m.Len() != 1 {
		t.Errorf("expected 1 entry, got %d", m.Len())
	}
}

// D7-S2-A50-T08: Record rejects empty SessionID.
func TestReasonLog_RejectsEmptySessionID(t *testing.T) {
	m := NewReasonLog(100)
	err := m.Record(ReasonEntry{ToolName: "read_file", VerifyExitReason: "ok"})
	if err == nil {
		t.Errorf("empty SessionID should be rejected")
	}
}

// D7-S2-A50-T08: Record rejects empty VerifyExitReason.
func TestReasonLog_RejectsEmptyReason(t *testing.T) {
	m := NewReasonLog(100)
	err := m.Record(ReasonEntry{SessionID: "sess-1", ToolName: "read_file"})
	if err == nil {
		t.Errorf("empty VerifyExitReason should be rejected")
	}
}

// D7-S2-A50-T08: FIFO eviction when at capacity.
func TestReasonLog_FIFOEviction(t *testing.T) {
	m := NewReasonLog(3)
	for i := 0; i < 5; i++ {
		_ = m.Record(ReasonEntry{
			SessionID:        "sess-" + string(rune('A'+i)),
			ToolName:         "read_file",
			VerifyExitReason: "ok",
		})
	}
	if m.Len() != 3 {
		t.Errorf("expected 3 entries after eviction, got %d", m.Len())
	}
}

// D7-S2-A50-T08: RecentByTool returns matching entries newest-first.
func TestReasonLog_RecentByTool(t *testing.T) {
	m := NewReasonLog(100)
	_ = m.Record(ReasonEntry{SessionID: "s1", ToolName: "read_file", VerifyExitReason: "ok"})
	_ = m.Record(ReasonEntry{SessionID: "s2", ToolName: "grep", VerifyExitReason: "ok"})
	_ = m.Record(ReasonEntry{SessionID: "s3", ToolName: "read_file", VerifyExitReason: "deliverable_missing"})

	recent := m.RecentByTool("read_file", 5)
	if len(recent) != 2 {
		t.Fatalf("expected 2 read_file entries, got %d", len(recent))
	}
	if recent[0].SessionID != "s3" {
		t.Errorf("newest first: expected s3, got %s", recent[0].SessionID)
	}
	if recent[1].SessionID != "s1" {
		t.Errorf("older: expected s1, got %s", recent[1].SessionID)
	}
}

// D7-S2-A50-T08: DriftRate — fraction of non-OK entries.
func TestReasonLog_DriftRate(t *testing.T) {
	m := NewReasonLog(100)
	_ = m.Record(ReasonEntry{SessionID: "s1", ToolName: "read_file", VerifyExitReason: "ok"})
	_ = m.Record(ReasonEntry{SessionID: "s2", ToolName: "read_file", VerifyExitReason: "deliverable_missing"})
	_ = m.Record(ReasonEntry{SessionID: "s3", ToolName: "read_file", VerifyExitReason: "ok"})
	_ = m.Record(ReasonEntry{SessionID: "s4", ToolName: "read_file", VerifyExitReason: "source_uncertainty_high"})

	drift := m.DriftRate("read_file", 10)
	if drift != 0.5 {
		t.Errorf("expected drift=0.5, got %f", drift)
	}
}

// D7-S2-A50-T08: DriftRate for unknown tool returns 0.
func TestReasonLog_DriftRate_Unknown(t *testing.T) {
	m := NewReasonLog(100)
	if drift := m.DriftRate("nonexistent", 10); drift != 0 {
		t.Errorf("unknown tool should have drift=0, got %f", drift)
	}
}

// D7-S2-A50-T08: RecordFromVerdict convenience wrapper.
func TestReasonLog_RecordFromVerdict(t *testing.T) {
	m := NewReasonLog(100)
	err := m.RecordFromVerdict("sess-1", "read_file", contracts.EC_Probe,
		"deliverable_missing", "review", 16)
	if err != nil {
		t.Fatalf("RecordFromVerdict: %v", err)
	}
	recent := m.RecentByTool("read_file", 1)
	if len(recent) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(recent))
	}
	if recent[0].TaskKind != "review" {
		t.Errorf("expected task_kind=review, got %q", recent[0].TaskKind)
	}
	if recent[0].IterationsUsed != 16 {
		t.Errorf("expected iter=16 (ProbeToolChannel Bounded(15) hit), got %d", recent[0].IterationsUsed)
	}
}
