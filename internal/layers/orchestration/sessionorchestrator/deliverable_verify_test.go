package sessionorchestrator

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestVerifyDeliverable_should_complete_when_p0_file_line(t *testing.T) {
	art := &wavescheduler.Artifact{
		Summary:  "P0: nil deref in internal/foo/bar.go:42",
		Metadata: map[string]any{"stop_reason": "final_answer"},
	}
	got := VerifyDeliverable(workmodel.DeliverableSchemaP0P1FileLine, art)
	if got.Status != workmodel.DeliverableStatusComplete {
		t.Fatalf("status = %q, want complete", got.Status)
	}
	if got.Payload == nil || len(got.Payload.Findings) == 0 {
		t.Fatal("expected parsed findings payload")
	}
}

func TestVerifyDeliverable_should_incomplete_when_max_iters_without_citation(t *testing.T) {
	art := &wavescheduler.Artifact{
		Summary:  "Let me continue exploring the kernel package.",
		Metadata: map[string]any{"stop_reason": "max_iters"},
	}
	got := VerifyDeliverable(workmodel.DeliverableSchemaP0P1FileLine, art)
	if got.Status != workmodel.DeliverableStatusIncomplete {
		t.Fatalf("status = %q, want incomplete", got.Status)
	}
	if got.Reason == "" {
		t.Fatal("expected reason for incomplete deliverable")
	}
}

func TestVerifyDeliverable_should_incomplete_when_exploration_transition(t *testing.T) {
	art := &wavescheduler.Artifact{
		Summary:  "继续探索 internal/layers/contextengine/kernel/",
		Metadata: map[string]any{"stop_reason": "final_answer"},
	}
	got := VerifyDeliverable(workmodel.DeliverableSchemaP0P1FileLine, art)
	if got.Status != workmodel.DeliverableStatusIncomplete {
		t.Fatalf("status = %q, want incomplete", got.Status)
	}
}

func TestVerifyDeliverable_should_not_apply_when_schema_not_applicable(t *testing.T) {
	got := VerifyDeliverable(workmodel.DeliverableSchemaNotApplicable, &wavescheduler.Artifact{Summary: "anything"})
	if got.Status != workmodel.DeliverableStatusNotApplicable {
		t.Fatalf("status = %q", got.Status)
	}
}

func TestVerifyArtifactForWorkItemWithSchema_should_downgrade_pass_when_incomplete(t *testing.T) {
	item := &workmodel.WorkItem{ID: "wi_1", Kind: workmodel.WorkKindExplore}
	art := &wavescheduler.Artifact{
		TaskID:   "wi_1",
		Summary:  "Let me read the next file.",
		ExitCode: 0,
		Metadata: map[string]any{"stop_reason": "max_iters"},
	}
	out := verifyArtifactForWorkItemWithSchema(art, item, nil, workmodel.DeliverableSchemaP0P1FileLine)
	if out.Verdict.Kind != types.VerdictPartial {
		t.Fatalf("verdict = %q, want partial", out.Verdict.Kind)
	}
	if out.Deliverable.Status != workmodel.DeliverableStatusIncomplete {
		t.Fatalf("deliverable = %q", out.Deliverable.Status)
	}
}
