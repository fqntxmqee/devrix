package prompttags

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// T: D2-S15-A96-T01 (DM-20260704-005) MUPSIOCatalog documents all I/O profiles.
func TestMUPSIOCatalog_CoversAllProfiles(t *testing.T) {
	seen := make(map[EncodingProfile]bool)
	for _, entry := range MUPSIOCatalog {
		if entry.Profile == "" {
			t.Fatalf("empty profile for %q", entry.Name)
		}
		seen[entry.Profile] = true
	}
	for _, want := range []EncodingProfile{EncodingEnvelope, EncodingLineField, EncodingLineFrame, EncodingWholeBody} {
		if !seen[want] {
			t.Fatalf("catalog missing profile %q", want)
		}
	}
	if len(MUPSRegistry) == 0 || len(LineFrameRegistry) != 2 || len(WholeBodyRegistry) != 2 {
		t.Fatalf("registry sizes: envelope=%d lineframe=%d wholebody=%d",
			len(MUPSRegistry), len(LineFrameRegistry), len(WholeBodyRegistry))
	}
}

// T: D2-S15-A96-T02 (DM-20260704-005) LookupLineFrame returns registered frames.
func TestLookupLineFrame_ObserveAndPlan(t *testing.T) {
	obs, ok := LookupLineFrame(FrameObserveUser)
	if !ok || len(obs.Fields) == 0 {
		t.Fatal("observe frame not registered")
	}
	plan, ok := LookupLineFrame(FramePlanUser)
	if !ok || len(plan.Fields) == 0 {
		t.Fatal("plan frame not registered")
	}
	if _, ok := LookupLineFrame(FrameName("unknown")); ok {
		t.Fatal("unknown frame should not resolve")
	}
}

func TestWholeBodyRegistry_Phases(t *testing.T) {
	if entry, ok := WholeBodyRegistry[contracts.MUPSPhaseObserve]; !ok || entry.Profile != EncodingWholeBody {
		t.Fatalf("observe wholebody: %+v ok=%v", entry, ok)
	}
	if entry, ok := WholeBodyRegistry[contracts.MUPSPhasePlan]; !ok || entry.Profile != EncodingWholeBody {
		t.Fatalf("plan wholebody: %+v ok=%v", entry, ok)
	}
}
