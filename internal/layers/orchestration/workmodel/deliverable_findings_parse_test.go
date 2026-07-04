package workmodel

import "testing"

func TestParseFindingsJSONPayload_should_parse_valid_findings(t *testing.T) {
	summary := "```json\n{\"findings\":[{\"id\":\"F-1\",\"severity\":\"p1\",\"title\":\"dead helper\",\"file\":\"internal/kernel/context_engine.go\",\"line\":126,\"evidence\":\"noopEmit unused\",\"impact\":\"misleading comment\",\"recommendation\":\"delete helper\"}]}\n```"
	p, ok := parseFindingsJSONPayload(summary)
	if !ok || p == nil || len(p.Findings) != 1 {
		t.Fatalf("parse failed: ok=%v payload=%v", ok, p)
	}
	f := p.Findings[0]
	if f.Title != "dead helper" || f.Line != 126 || f.Evidence == "" {
		t.Fatalf("finding = %+v", f)
	}
}

func TestParseFindingsJSONPayload_should_reject_planning_prose_findings(t *testing.T) {
	summary := `{"findings":[{"severity":"p1","file":"internal/foo.go","line":1,"title":"x","message":"The user wants me to review more files first"}]}`
	p, ok := parseFindingsJSONPayload(summary)
	if ok {
		t.Fatalf("expected reject, got ok=%v payload=%+v", ok, p)
	}
}

func TestShouldSuppressFindingsArtifactStream(t *testing.T) {
	round := &WorkItemPipelineRound{
		DeliverableContract: DeliverableContract{Structure: DeliverableStructureFindingsJSON},
		ArtifactSummary:     "The user wants me to review\n```json\n{\"findings\":[]}\n```",
	}
	if !ShouldSuppressFindingsArtifactStream(round) {
		t.Fatal("expected suppress for planning+json draft")
	}
}

func TestVerifyDeliverableContract_rejects_regex_fallback_for_findings_json(t *testing.T) {
	c := DeliverableContract{
		Citation:  DeliverableCitationFileLine,
		Severity:  DeliverableSeverityP0P1,
		Structure: DeliverableStructureFindingsJSON,
	}
	prose := "The user wants me to review internal/layers/contextengine/kernel/context_engine.go:126 for noopEmit"
	got := VerifyDeliverableContract(c, prose, "final_answer")
	if got.Status != DeliverableStatusIncomplete {
		t.Fatalf("status=%q reason=%q", got.Status, got.Reason)
	}
}
