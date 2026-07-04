package i18n

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/prompttags"
)

// T: D2-S15-A97-T01 — L5-MUPS-TAG-01 Observe appendix includes obs_uncertainty when-use.
func TestObservationTaskAppendix_IncludesObserveKindSemantics(t *testing.T) {
	for _, loc := range []Locale{LocaleZH, LocaleEN} {
		got := ObservationTaskAppendix(loc)
		if !strings.Contains(got, "obs_uncertainty") {
			t.Fatalf("loc %q missing obs_uncertainty: %q", loc, got)
		}
		if loc == LocaleZH && !strings.Contains(got, "范围/目标不清") {
			t.Fatalf("zh missing when-use: %q", got)
		}
		if loc == LocaleEN && !strings.Contains(got, "scope/goal unclear") {
			t.Fatalf("en missing when-use: %q", got)
		}
		if !strings.Contains(got, `{"kind":"obs_fact`) {
			t.Fatalf("schema line missing after semantics: %q", got)
		}
	}
}

// T: D2-S15-A97-T02 — L5-MUPS-TAG-02 Plan appendix includes execution_mode decision tree.
func TestStrategicPlanAppendix_IncludesExecutionModeSemantics(t *testing.T) {
	dims := `{"citation":["file_line"]}`
	for _, loc := range []Locale{LocaleZH, LocaleEN} {
		got := StrategicPlanAppendix(loc, dims)
		if !strings.Contains(got, "uncertainty_mean") {
			t.Fatalf("loc %q missing uncertainty gate: %q", loc, got)
		}
		if !strings.Contains(got, "decompose") {
			t.Fatalf("loc %q missing decompose branch: %q", loc, got)
		}
	}
}

// T: D2-S15-A97-T03 — L5-MUPS-TAG-03 Execute hints include Required/Optional matrix.
func TestWorkItemExecuteOutputHints_IncludesRequiredOptionalMatrix(t *testing.T) {
	for _, loc := range []Locale{LocaleZH, LocaleEN} {
		got := WorkItemExecuteOutputHints(loc)
		if !strings.Contains(got, "deliverable_contract") {
			t.Fatalf("loc %q missing deliverable_contract semantics: %q", loc, got)
		}
		if !strings.Contains(got, "findings_json") {
			t.Fatalf("loc %q missing findings_json semantics: %q", loc, got)
		}
		if loc == LocaleZH {
			if !strings.Contains(got, "必填") || !strings.Contains(got, "可选") {
				t.Fatalf("zh missing 必填/可选 markers: %q", got)
			}
			if strings.Contains(got, "Required when contract applicable") {
				t.Fatalf("zh must not use English execute semantics: %q", got)
			}
		} else if !strings.Contains(got, "Required") && !strings.Contains(got, "Optional") {
			t.Fatalf("en missing Required/Optional markers: %q", got)
		}
	}
}

// T: D2-S15-A97-T04 — L5-MUPS-TAG-05 user frame control/data guide.
func TestRenderFrameFieldGuide_ObservePlan(t *testing.T) {
	for _, tc := range []struct {
		frame prompttags.FrameName
		loc   Locale
		want  string
	}{
		{prompttags.FrameObserveUser, LocaleZH, "[control]"},
		{prompttags.FramePlanUser, LocaleEN, "[data]"},
	} {
		got := RenderFrameFieldGuide(tc.frame, tc.loc)
		if !strings.Contains(got, tc.want) {
			t.Fatalf("frame %q loc %q guide missing %q:\n%s", tc.frame, tc.loc, tc.want, got)
		}
	}
}

func TestRenderSemanticAppendix_AllPhasesNonEmpty(t *testing.T) {
	for _, phase := range []contracts.MUPSPhase{
		contracts.MUPSPhaseObserve, contracts.MUPSPhasePlan, contracts.MUPSPhaseExecute,
	} {
		for _, loc := range []Locale{LocaleZH, LocaleEN} {
			got := RenderSemanticAppendix(phase, loc)
			if strings.TrimSpace(got) == "" {
				t.Fatalf("empty semantic appendix phase=%s loc=%s", phase, loc)
			}
		}
	}
}
