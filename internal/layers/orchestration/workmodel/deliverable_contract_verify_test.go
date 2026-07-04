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
	jsonSummary := `{"findings":[{"severity":"P0","file":"internal/foo.go","line":42,"title":"nil deref"}]}`
	got = VerifyDeliverableContract(c, jsonSummary, "final_answer")
	if got.Status != DeliverableStatusComplete {
		t.Fatalf("status=%q reason=%q", got.Status, got.Reason)
	}
}

func TestVerifyDeliverableContract_planningOutsideValidFindingsJSON(t *testing.T) {
	// L5-D7-U-05: planning_meta reject applies to extracted JSON body only
	c := DeliverableContract{
		Citation:  DeliverableCitationFileLine,
		Severity:  DeliverableSeverityP0P1,
		Structure: DeliverableStructureFindingsJSON,
		Reject:    []DeliverableReject{DeliverableRejectPlanningMeta},
	}
	summary := "Let me first summarize what I found.\n```json\n{\"findings\":[{\"severity\":\"P0\",\"title\":\"nil deref\",\"file\":\"internal/foo.go\",\"line\":42,\"evidence\":\"missing check\"}]}\n```"
	got := VerifyDeliverableContract(c, summary, "final_answer")
	if got.Status != DeliverableStatusComplete {
		t.Fatalf("status=%q reason=%q, want complete", got.Status, got.Reason)
	}
}

func TestVerifyDeliverableContract_salvageCorruptFindingsWithPlanningProse(t *testing.T) {
	c := DeliverableContract{
		Citation:  DeliverableCitationFileLine,
		Severity:  DeliverableSeverityP0P1,
		Structure: DeliverableStructureFindingsJSON,
		Reject:    []DeliverableReject{DeliverableRejectPlanningMeta},
	}
	summary := "The user wants me to review plan code.\n```json\n{\n  \"review_target\": \"plan/\",\n  \"notCorrupt\": true,\n  \"findings\": [\n    {\"id\":\"F-001\",\"severity\":\"high\",\"file\":\"plan.go\",\"line_range\":\"107-114\",\"description\":\"MarshalJSON mismatch\",\"recommendation\":\"fix doc\"},\n    {\"id\":\"F-002\",\"severity\":\"medium\",\"file\":\"plan_struct.go\",\"line_range\":\"117-127\",\"description\":\"NaN strength\",\"recommendation\":\"guard NaN\"}\n  ]\n}\n```"
	got := VerifyDeliverableContract(c, summary, "final_answer")
	if got.Status != DeliverableStatusComplete {
		t.Fatalf("status=%q reason=%q, want complete via salvage", got.Status, got.Reason)
	}
	if got.Payload == nil || len(got.Payload.Findings) < 2 {
		t.Fatalf("expected salvaged findings, got %+v", got.Payload)
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
