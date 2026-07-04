package sessionorchestrator

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

func TestPriorDeliverableRetryHint_ScopeAndReason(t *testing.T) {
	contract := workmodel.DeliverableContract{
		Citation:  workmodel.DeliverableCitationFileLine,
		Severity:  workmodel.DeliverableSeverityP0P1,
		Structure: workmodel.DeliverableStructureFindingsJSON,
		Reject:    []workmodel.DeliverableReject{workmodel.DeliverableRejectPlanningMeta},
	}
	item := &workmodel.WorkItem{
		ScopeContract: &workmodel.ScopeContract{
			InScope: []string{"internal/layers/orchestration/plan/"},
		},
		LastRound: &workmodel.WorkItemPipelineRound{
			DeliverableStatus: workmodel.DeliverableStatusIncomplete,
			ArtifactSummary:   "Let me read openspec files first.",
		},
	}
	got := PriorDeliverableRetryHint(item, contract)
	for _, want := range []string{
		"ScopeIn: internal/layers/orchestration/plan/",
		"PriorDeliverableFailure: planning_meta",
		"synthesize findings_json",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("hint missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "scope_disjoint") {
		t.Fatalf("must not include spawn rationale: %q", got)
	}
}

func TestEffectiveExecuteMaxIters_FindingsJSONFloor(t *testing.T) {
	c := workmodel.DeliverableContract{Structure: workmodel.DeliverableStructureFindingsJSON}
	if got := workmodel.EffectiveExecuteMaxIters(3, 5, c); got != 5 {
		t.Fatalf("got %d want 5", got)
	}
	if got := workmodel.EffectiveExecuteMaxIters(0, 5, c); got != 5 {
		t.Fatalf("default got %d want 5", got)
	}
	free := workmodel.DeliverableContract{Structure: workmodel.DeliverableStructureFreeText}
	if got := workmodel.EffectiveExecuteMaxIters(3, 5, free); got != 3 {
		t.Fatalf("free text got %d want 3", got)
	}
}
