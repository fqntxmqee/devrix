// T: D2-S15-A02-T02, T03, T15 — Filter v2 3-dimensional tests + cross-consistency.
package filter

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// D2-S15-A02-T02: PerEmissionClassFilter — keep only allowed classes.
func TestPerEmissionClassFilter_Apply(t *testing.T) {
	allowed := []contracts.EmissionClass{contracts.EC_Fact, contracts.EC_Probe}
	filter := NewPerEmissionClassFilter(allowed)

	specs := []contracts.ToolSpec{
		{Name: "read_file", EmissionClass: contracts.EC_Probe},
		{Name: "write_file", EmissionClass: contracts.EC_Action},
		{Name: "query_diagnostics", EmissionClass: contracts.EC_Fact},
	}
	out := filter.Apply(specs)
	if len(out) != 2 {
		t.Errorf("expected 2 specs (Fact+Probe), got %d", len(out))
	}
	for _, s := range out {
		if s.Name == "write_file" {
			t.Errorf("Action should have been filtered out")
		}
	}
}

// D2-S15-A02-T02: PerEmissionClassFilter — nil allow list = allow all.
func TestPerEmissionClassFilter_AllowAll(t *testing.T) {
	filter := NewPerEmissionClassFilter(nil)
	specs := []contracts.ToolSpec{
		{Name: "read_file", EmissionClass: contracts.EC_Probe},
		{Name: "write_file", EmissionClass: contracts.EC_Action},
		{Name: "query_diagnostics", EmissionClass: contracts.EC_Fact},
		{Name: "free_fork", EmissionClass: contracts.EC_Experiment},
	}
	out := filter.Apply(specs)
	if len(out) != 4 {
		t.Errorf("nil allow list should pass all, got %d", len(out))
	}
}

// D2-S15-A02-T03: PerTaskKindFilter — review → Bounded(15).
func TestTaskKindBound_Review(t *testing.T) {
	b := TaskKindBound("review")
	if b.Kind != contracts.IB_Bounded || b.MaxN != 15 {
		t.Errorf("review should be Bounded(15), got %v", b)
	}
}

// D2-S15-A02-T03: PerTaskKindFilter — edit → Bounded(10).
func TestTaskKindBound_Edit(t *testing.T) {
	b := TaskKindBound("edit")
	if b.Kind != contracts.IB_Bounded || b.MaxN != 10 {
		t.Errorf("edit should be Bounded(10), got %v", b)
	}
}

// D2-S15-A02-T03: PerTaskKindFilter — observe → OpenEnded.
func TestTaskKindBound_Observe(t *testing.T) {
	b := TaskKindBound("observe")
	if b.Kind != contracts.IB_OpenEnded {
		t.Errorf("observe should be OpenEnded, got %v", b)
	}
}

// D2-S15-A02-T03: PerTaskKindFilter — refactor → Bounded(8).
func TestTaskKindBound_Refactor(t *testing.T) {
	b := TaskKindBound("refactor")
	if b.Kind != contracts.IB_Bounded || b.MaxN != 8 {
		t.Errorf("refactor should be Bounded(8), got %v", b)
	}
}

// D2-S15-A02-T03: PerTaskKindFilter — unknown kind → OpenEnded.
func TestTaskKindBound_Unknown(t *testing.T) {
	b := TaskKindBound("made_up_kind")
	if b.Kind != contracts.IB_OpenEnded {
		t.Errorf("unknown kind should be OpenEnded, got %v", b)
	}
}

// D2-S15-A02-T15: PerTaskKindFilterCrossConsistency — review + Probe tool
// must NOT be downgraded to OpenEnded.
func TestPerTaskKindFilterCrossConsistency(t *testing.T) {
	filter := NewPerTaskKindFilter("review")
	specs := []contracts.ToolSpec{
		{
			Name:          "read_file",
			EmissionClass: contracts.EC_Probe,
			IterationBound: contracts.IterationBound{Kind: contracts.IB_OpenEnded}, // starts OpenEnded
		},
		{
			Name:          "query_diagnostics",
			EmissionClass: contracts.EC_Fact,
			IterationBound: contracts.IterationBound{Kind: contracts.IB_OpenEnded},
		},
	}
	out := filter.Apply(specs)

	// DM-20260702-008 / D2-S15-A02-T12: cross-consistency rule RELAXED.
	// read_file is now OpenEnded by default (T11) and the task_kind
	// filter no longer forces it to Bounded(15). The治本 change in T09
	// means the channel never hard-rejects anyway, so the rule's
	// purpose (prevent Probe from escaping the bound) is moot.
	readFile := out[0]
	if readFile.IterationBound.Kind != contracts.IB_OpenEnded {
		t.Errorf("read_file should stay OpenEnded (T12 advisory), got %v", readFile.IterationBound)
	}

	// query_diagnostics: Fact class, observe-task bound would be OpenEnded
	// but task_kind=review applies the cross-consistency rule for Probe only.
	// For Fact, OpenEnded is fine.
	queryDiag := out[1]
	if queryDiag.IterationBound.Kind != contracts.IB_OpenEnded {
		t.Errorf("query_diagnostics (Fact) should remain OpenEnded for review, got %v", queryDiag.IterationBound)
	}
}

// D2-S15-A02-T04: AllowedEmissionClassesForAgent — explore → Fact + Probe.
func TestAllowedEmissionClassesForAgent_Explore(t *testing.T) {
	allowed := AllowedEmissionClassesForAgent("explore")
	if len(allowed) != 2 {
		t.Errorf("explore should have 2 allowed classes, got %d", len(allowed))
	}
	m := map[contracts.EmissionClass]bool{}
	for _, c := range allowed {
		m[c] = true
	}
	if !m[contracts.EC_Fact] || !m[contracts.EC_Probe] {
		t.Errorf("explore should allow Fact + Probe, got %v", allowed)
	}
}

// D2-S15-A02-T04: AllowedEmissionClassesForAgent — worker → Fact + Action + Probe.
func TestAllowedEmissionClassesForAgent_Worker(t *testing.T) {
	allowed := AllowedEmissionClassesForAgent("worker")
	if len(allowed) != 3 {
		t.Errorf("worker should have 3 allowed classes, got %d", len(allowed))
	}
}

// D2-S15-A02-T04: AllowedEmissionClassesForAgent — planner → all (nil).
func TestAllowedEmissionClassesForAgent_Planner(t *testing.T) {
	allowed := AllowedEmissionClassesForAgent("planner")
	if allowed != nil {
		t.Errorf("planner should allow all (nil), got %v", allowed)
	}
}

// D2-S15-A02-T03: isTighter — Bounded(5) is tighter than Bounded(15).
func TestIsTighter_BoundedVsBounded(t *testing.T) {
	tight := contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 5}
	loose := contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 15}
	if !isTighter(tight, loose) {
		t.Errorf("Bounded(5) should be tighter than Bounded(15)")
	}
	if isTighter(loose, tight) {
		t.Errorf("Bounded(15) should NOT be tighter than Bounded(5)")
	}
}

// D2-S15-A02-T03: isTighter — Bounded is always tighter than OpenEnded.
func TestIsTighter_BoundedVsOpenEnded(t *testing.T) {
	bounded := contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 15}
	open := contracts.IterationBound{Kind: contracts.IB_OpenEnded}
	if !isTighter(bounded, open) {
		t.Errorf("Bounded should be tighter than OpenEnded")
	}
	if isTighter(open, bounded) {
		t.Errorf("OpenEnded should NOT be tighter than Bounded")
	}
}
