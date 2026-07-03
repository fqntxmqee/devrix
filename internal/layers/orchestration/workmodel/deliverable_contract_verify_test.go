package workmodel

import "testing"

func TestVerifyDeliverableContract_FindingsJSONRequired(t *testing.T) {
	c := DeliverableContract{
		Citation:  DeliverableCitationFileLine,
		Severity:  DeliverableSeverityP0P1,
		Structure: DeliverableStructureFindingsJSON,
		Reject:    []DeliverableReject{DeliverableRejectPlanningMeta},
	}
	got := VerifyDeliverableContract(c, "Let me first read the files.", "final_answer")
	if got.Status != DeliverableStatusIncomplete || got.Reason != "planning_meta" {
		t.Fatalf("status=%q reason=%q, want planning_meta", got.Status, got.Reason)
	}
	jsonSummary := `{"findings":[{"severity":"P0","file":"internal/foo.go","line":42,"message":"nil deref"}]}`
	got = VerifyDeliverableContract(c, jsonSummary, "final_answer")
	if got.Status != DeliverableStatusComplete {
		t.Fatalf("status=%q reason=%q", got.Status, got.Reason)
	}
}

func TestDeliverableInlineWouldExhaust(t *testing.T) {
	ctx := TreeEvalContext{InlineRetriesAtMaxDepth: 2, MaxInlineRetriesAtMaxDepth: 3}
	if !deliverableInlineWouldExhaust(ctx) {
		t.Fatal("expected next inline to exhaust budget")
	}
	ctx.InlineRetriesAtMaxDepth = 1
	if deliverableInlineWouldExhaust(ctx) {
		t.Fatal("should not exhaust yet")
	}
}
