package i18n

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/prompttags"
)

// T: D2-S15-A97-T01 — L5-MUPS-TAG-01 Observe appendix includes obs_uncertainty machine rule + glossary.
func TestObservationTaskAppendix_IncludesObserveKindSemantics(t *testing.T) {
	for _, loc := range []Locale{LocaleZH, LocaleEN} {
		got := ObservationTaskAppendix(loc)
		if !strings.Contains(got, `"target":"obs_uncertainty"`) {
			t.Fatalf("loc %q missing obs_uncertainty machine rule: %q", loc, got)
		}
		if loc == LocaleZH && !strings.Contains(got, "范围/目标不清") {
			t.Fatalf("zh missing scope_unclear glossary: %q", got)
		}
		if loc == LocaleEN && !strings.Contains(got, "scope/goal unclear") {
			t.Fatalf("en missing scope_unclear glossary: %q", got)
		}
		if !strings.Contains(got, `{"kind":"obs_fact`) {
			t.Fatalf("schema line missing after semantics: %q", got)
		}
	}
}

// T: D2-S15-A97-T02 — L5-MUPS-TAG-02 Plan appendix includes execution_mode decision tree glossary.
func TestStrategicPlanAppendix_IncludesExecutionModeSemantics(t *testing.T) {
	dims := `{"citation":["file_line"]}`
	for _, loc := range []Locale{LocaleZH, LocaleEN} {
		got := StrategicPlanAppendix(loc, dims)
		if !strings.Contains(got, `"target":"execution_mode"`) {
			t.Fatalf("loc %q missing execution_mode machine rule: %q", loc, got)
		}
		if !strings.Contains(got, "uncertainty_mean") {
			t.Fatalf("loc %q missing uncertainty gate glossary: %q", loc, got)
		}
		if !strings.Contains(got, "decompose") {
			t.Fatalf("loc %q missing decompose branch: %q", loc, got)
		}
	}
}

// T: D2-S15-A97-T03 — L5-MUPS-TAG-03 Execute hints include Required/Optional matrix via glossary.
func TestWorkItemExecuteOutputHints_IncludesRequiredOptionalMatrix(t *testing.T) {
	for _, loc := range []Locale{LocaleZH, LocaleEN} {
		got := WorkItemExecuteOutputHints(loc)
		if !strings.Contains(got, `"target":"deliverable_contract"`) {
			t.Fatalf("loc %q missing deliverable_contract machine rule: %q", loc, got)
		}
		if !strings.Contains(got, `"target":"findings_json"`) {
			t.Fatalf("loc %q missing findings_json machine rule: %q", loc, got)
		}
		if loc == LocaleZH {
			if !strings.Contains(got, "必填") || !strings.Contains(got, "可选") {
				t.Fatalf("zh missing 必填/可选 glossary markers: %q", got)
			}
		} else if !strings.Contains(got, "Required") && !strings.Contains(got, "Optional") {
			t.Fatalf("en missing Required/Optional glossary markers: %q", got)
		}
	}
}

// T: D2-S15-A97-T04 — L5-MUPS-TAG-05 user frame field guide (no plane prefixes).
func TestRenderFrameFieldGuide_ObservePlan(t *testing.T) {
	for _, tc := range []struct {
		frame prompttags.FrameName
		loc   Locale
		want  string
	}{
		{prompttags.FrameObserveUser, LocaleZH, "User 帧字段"},
		{prompttags.FramePlanUser, LocaleEN, "User frame fields"},
	} {
		got := RenderFrameFieldGuide(tc.frame, tc.loc)
		if !strings.Contains(got, tc.want) {
			t.Fatalf("frame %q loc %q guide missing %q:\n%s", tc.frame, tc.loc, tc.want, got)
		}
		if strings.Contains(got, "[control]") || strings.Contains(got, "[data]") {
			t.Fatalf("plane prefixes should be removed:\n%s", got)
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
