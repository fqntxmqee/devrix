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

func TestParseFindingsJSONPayload_locationFieldAlias(t *testing.T) {
	summary := `{"findings":[{"id":"F-01","severity":"high","location":"planner.go:NewPlanID","title":"non-deterministic hash","detail":"map iteration order"}]}`
	p, ok := parseFindingsJSONPayload(summary)
	if !ok || p == nil || len(p.Findings) != 1 {
		t.Fatalf("parse failed: ok=%v payload=%v", ok, p)
	}
	f := p.Findings[0]
	if f.File != "planner.go" || f.Severity != "P0" {
		t.Fatalf("finding = %+v", f)
	}
}

func TestSalvageDeliverablePayload_corruptMixedSummary(t *testing.T) {
	contract := DeliverableContract{
		Citation:  DeliverableCitationFileLine,
		Severity:  DeliverableSeverityP0P1,
		Structure: DeliverableStructureFindingsJSON,
	}
	summary := "Let me read files.\n```json\n{\"module\":\"x\",\"findings\":[{\"id\":\"F-01\",\"severity\":\"high\",\"location\":\"planner.go:NewPlanID\",\"title\":\"hash bug\",\"detail\":\"sort before hash\"}]}\n```"
	payload := SalvageDeliverablePayload(summary, contract)
	if !FindingsPayloadPresentable(payload) {
		t.Fatalf("expected salvage, got %+v", payload)
	}
	got := VerifyDeliverableContract(contract, summary, "max_iters")
	if got.Status != DeliverableStatusComplete && got.Reason != "findings_json_incomplete" {
		if got.Status == DeliverableStatusIncomplete && got.Reason == "findings_json_required" {
			t.Fatalf("verify should salvage findings, got reason=%q", got.Reason)
		}
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

func TestParseFindingsJSONPayload_issueFieldAlias(t *testing.T) {
	// L5-D7-U-05: alias registry maps issue → Title
	summary := `{"findings":[{"severity":"P1","issue":"race on map","file":"internal/foo.go","line":42,"evidence":"unsynchronized write"}]}`
	p, ok := parseFindingsJSONPayload(summary)
	if !ok || p == nil || len(p.Findings) != 1 {
		t.Fatalf("parse failed: ok=%v payload=%v", ok, p)
	}
	if p.Findings[0].Title != "race on map" {
		t.Fatalf("Title = %q, want race on map", p.Findings[0].Title)
	}
}

func TestExtractDeliverableJSONObject_proseBeforeJSONFence(t *testing.T) {
	// L5-D7-U-05: structural fence extraction skips prose before first `{`
	summary := "The user wants me to finish the review.\n\n```json\nHere is the summary:\n{\"findings\":[{\"severity\":\"P1\",\"title\":\"nil deref\",\"file\":\"internal/foo.go\",\"line\":1,\"evidence\":\"missing check\"}]}\n```"
	if raw := extractDeliverableJSONObject(summary); raw == nil {
		t.Fatal("expected extracted JSON object")
	}
	p, ok := parseFindingsJSONPayload(summary)
	if !ok || p == nil || len(p.Findings) != 1 {
		t.Fatalf("parse failed: ok=%v payload=%v", ok, p)
	}
	if p.Findings[0].Title != "nil deref" {
		t.Fatalf("finding = %+v", p.Findings[0])
	}
}
