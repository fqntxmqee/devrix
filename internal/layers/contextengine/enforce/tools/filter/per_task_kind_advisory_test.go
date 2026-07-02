// T: D2-S15-A02-T12 — task_kind filter advisory mode + TaskKindHint.
package filter

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// D2-S15-A02-T12: TaskKindHint exposes the advisory bound for read_file
// (which is now OpenEnded in T11).
func TestTaskKindHint_ReadFile(t *testing.T) {
	hint, ok := TaskKindHint("read_file")
	if !ok {
		t.Fatal("TaskKindHint(read_file) should return a hint, got false")
	}
	// review-task bound is Bounded(15); the hint for read_file reflects
	// what the channel would use IF the spec had a Bounded bound. The
	// spec is OpenEnded in T11, so this hint is metadata only.
	if hint.Kind != contracts.IB_Bounded || hint.MaxN != 15 {
		t.Errorf("TaskKindHint(read_file) = %+v, want Bounded(15)", hint)
	}
}

// D2-S15-A02-T12: TaskKindHint returns false for unknown tools.
func TestTaskKindHint_Unknown(t *testing.T) {
	_, ok := TaskKindHint("not_a_real_tool")
	if ok {
		t.Errorf("TaskKindHint(unknown) should return false")
	}
}

// D2-S15-A02-T12: PerTaskKindFilter.Apply no longer forces Probe tools
// to Bounded(15) under review (the cross-consistency rule from P1-AC-7
// is relaxed in T12).
func TestPerTaskKindFilter_Advisory_ProbeStaysOpenEnded(t *testing.T) {
	filter := NewPerTaskKindFilter("review")
	specs := []contracts.ToolSpec{
		{
			Name:          "read_file",
			EmissionClass: contracts.EC_Probe,
			// read_file is OpenEnded in T11
			IterationBound: contracts.IterationBound{Kind: contracts.IB_OpenEnded},
		},
	}
	out := filter.Apply(specs)
	if out[0].IterationBound.Kind != contracts.IB_OpenEnded {
		t.Errorf("read_file (OpenEnded Probe) should stay OpenEnded under review (T12), got %v", out[0].IterationBound)
	}
}

// D2-S15-A02-T12: Bounded(15) tools under review still get the hint
// (e.g. write_file, bash). Only OpenEnded tools are unaffected.
func TestPerTaskKindFilter_Advisory_BoundedStillTightened(t *testing.T) {
	filter := NewPerTaskKindFilter("review")
	specs := []contracts.ToolSpec{
		{
			Name:          "bash",
			EmissionClass: contracts.EC_Action,
			IterationBound: contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 10}, // bash default
		},
	}
	out := filter.Apply(specs)
	// review hint is Bounded(15) — looser than 10, so no tightening.
	if out[0].IterationBound.MaxN != 10 {
		t.Errorf("bash under review should keep Bounded(10), got %v", out[0].IterationBound)
	}
}
