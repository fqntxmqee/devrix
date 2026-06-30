package workmodel

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestStatusAfterSpawnNone_should_stay_in_progress_when_partial_without_deliverable(t *testing.T) {
	got := StatusAfterSpawnNone(types.VerdictPartial, DeliverableSchemaP0P1FileLine, DeliverableStatusIncomplete)
	if got != TaskStatusInProgress {
		t.Fatalf("status = %q, want in_progress", got)
	}
}

func TestStatusAfterSpawnNone_should_complete_when_partial_with_complete_deliverable(t *testing.T) {
	got := StatusAfterSpawnNone(types.VerdictPartial, DeliverableSchemaP0P1FileLine, DeliverableStatusComplete)
	if got != TaskStatusCompleted {
		t.Fatalf("status = %q, want completed", got)
	}
}

func TestStatusAfterSpawnNone_should_complete_when_pass_without_schema(t *testing.T) {
	got := StatusAfterSpawnNone(types.VerdictPass, DeliverableSchemaNotApplicable, DeliverableStatusNotApplicable)
	if got != TaskStatusCompleted {
		t.Fatalf("status = %q, want completed", got)
	}
}

func TestStatusAfterSpawnNone_should_fail_when_verdict_fail_and_incomplete(t *testing.T) {
	got := StatusAfterSpawnNone(types.VerdictFail, DeliverableSchemaP0P1FileLine, DeliverableStatusIncomplete)
	if got != TaskStatusFailed {
		t.Fatalf("status = %q, want failed", got)
	}
}
