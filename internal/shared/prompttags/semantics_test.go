package prompttags

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
)

func TestSemanticsForPhase_ObserveOutputRulesNonEmpty(t *testing.T) {
	sem := SemanticsForPhase(contracts.MUPSPhaseObserve)
	if sem.NodeRoleKey == "" {
		t.Fatal("observe node role key empty")
	}
	if len(sem.OutputRules) < 4 {
		t.Fatalf("observe output rules = %d, want >= 4", len(sem.OutputRules))
	}
	for _, rule := range sem.OutputRules {
		if rule.WhenUse == "" {
			t.Fatalf("empty WhenUse for %q", rule.Name)
		}
	}
}

func TestSemanticsForPhase_PlanExecutionModeEnforced(t *testing.T) {
	sem := SemanticsForPhase(contracts.MUPSPhasePlan)
	var found bool
	for _, rule := range sem.OutputRules {
		if rule.Name == "execution_mode" {
			found = true
			if !rule.Enforced {
				t.Fatal("execution_mode must be Enforced")
			}
		}
	}
	if !found {
		t.Fatal("missing execution_mode rule")
	}
}

func TestSemanticsForPhase_ExecuteRequiredTagsEnforced(t *testing.T) {
	sem := SemanticsForPhase(contracts.MUPSPhaseExecute)
	enforced := map[string]bool{}
	for _, rule := range sem.OutputRules {
		if rule.Enforced {
			enforced[rule.Name] = true
		}
	}
	for _, name := range []string{string(TagDeliverableContract), "findings_json"} {
		if !enforced[name] {
			t.Fatalf("%s must be Enforced", name)
		}
	}
}

// Enforced output rules must align with known Go gates (design §6).
func TestSemanticsEnforcedFlags_AlignWithGates(t *testing.T) {
	cases := []struct {
		phase contracts.MUPSPhase
		name  string
	}{
		{contracts.MUPSPhaseObserve, "max_proposals"},
		{contracts.MUPSPhasePlan, "execution_mode"},
		{contracts.MUPSPhasePlan, "child_specs"},
		{contracts.MUPSPhaseExecute, string(TagDeliverableContract)},
		{contracts.MUPSPhaseExecute, "findings_json"},
	}
	for _, c := range cases {
		sem := SemanticsForPhase(c.phase)
		var ok bool
		for _, rule := range sem.OutputRules {
			if rule.Name == c.name && rule.Enforced {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("phase %s rule %q must be Enforced", c.phase, c.name)
		}
	}
}

// Execute envelope semantics must cover all tagPhases execute tags (except linefield open_questions handled).
func TestExecuteSemantics_CoversTagPhases(t *testing.T) {
	sem := SemanticsForPhase(contracts.MUPSPhaseExecute)
	names := map[string]bool{}
	for _, rule := range sem.OutputRules {
		names[rule.Name] = true
	}
	for tag := range tagPhases {
		if tag == TagPriorVerifyReason {
			continue // verify-phase input, not LLM output semantics
		}
		if !names[string(tag)] && tag != TagDeliverableSchema {
			// deliverable_schema covered via findings_json / contract matrix
			if tag == TagOpenQuestions || tag == TagScopeContract || tag == TagDeliverableContract {
				continue
			}
			t.Errorf("execute semantics missing tagPhases entry %q", tag)
		}
	}
}

func TestFrameFieldPlane_ObserveControlFields(t *testing.T) {
	for _, name := range []TagName{TagPriorMean, TagIncrementalOnly} {
		if FrameFieldPlane(FrameObserveUser, name) != PlaneControl {
			t.Fatalf("%s should be control", name)
		}
	}
	if FrameFieldPlane(FrameObserveUser, TagDirective) != PlaneData {
		t.Fatal("directive should be data")
	}
}

func TestBuildAnnotatedLineFrame_Prefixes(t *testing.T) {
	got := BuildAnnotatedLineFrame(FrameObserveUser, ObserveUserFrame, map[TagName]any{
		TagWorkItemID: "wi_1",
		TagDirective:  "review",
		TagPriorMean:  0.5,
	})
	if !containsAllSubstrings(got, "[control] work_item_id:", "[data] directive:", "[control] prior_mean:") {
		t.Fatalf("annotated frame:\n%s", got)
	}
}

func containsAllSubstrings(s string, subs ...string) bool {
	for _, sub := range subs {
		if !containsSubstring(s, sub) {
			return false
		}
	}
	return true
}

func containsSubstring(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
