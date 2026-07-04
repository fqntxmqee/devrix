package workmodel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirectiveScopeCandidates_D2KernelDirectory(t *testing.T) {
	got := DirectiveScopeCandidates("review d2 领域 kernel目录下代码")
	want := "internal/layers/contextengine/kernel/"
	found := false
	for _, p := range got {
		if p == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("candidates = %v, want prefix %q", got, want)
	}
}

func TestPrepareStrategicScopeIn_D2KernelFallback(t *testing.T) {
	dir := t.TempDir()
	kernelDir := filepath.Join(dir, "internal/layers/contextengine/kernel")
	if err := os.MkdirAll(kernelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	directive := "review d2 领域 kernel目录下代码"
	got, _, reason := PrepareStrategicScopeIn(directive, nil, dir)
	if len(got) != 1 || got[0] != "internal/layers/contextengine/kernel/" {
		t.Fatalf("got scope %v reason=%q", got, reason)
	}
}

func TestFormatDeliverablePayloadForIM(t *testing.T) {
	got := FormatDeliverablePayloadForIM(&DeliverablePayload{
		Findings: []DeliverableFinding{
			{Severity: "p0", Title: "race in wire", File: "internal/foo.go", Line: 42, Evidence: "unsynchronized write", Impact: "data race", Recommendation: "add mutex"},
			{Severity: "P1", Title: "missing ctx", File: "internal/bar.go", Line: 10},
		},
	})
	if got == "" {
		t.Fatal("expected non-empty report")
	}
	for _, part := range []string{"**P0:** 1", "**P1:** 1", "race in wire", "internal/foo.go:42", "missing ctx", "证据:", "影响:", "建议:"} {
		if !strings.Contains(got, part) {
			t.Fatalf("report missing %q:\n%s", part, got)
		}
	}
}

func TestExtractSessionDeliverable_PrefersStructuredFindings(t *testing.T) {
	tm := NewTaskManager()
	sessionID := "sess-deliverable-im"
	goal, err := tm.EnsureGoal(sessionID, "review d2 kernel")
	if err != nil {
		t.Fatal(err)
	}
	round := &WorkItemPipelineRound{
		WorkItemID:        goal.ID,
		DeliverableStatus: DeliverableStatusIncomplete,
		ArtifactSummary:   "I'll explore the kernel directory first...",
		StructuredDeliverable: &DeliverablePayload{
			Findings: []DeliverableFinding{
				{Severity: "P0", Title: "data race", File: "internal/kernel/foo.go", Line: 1},
			},
		},
	}
	if err := tm.Tree().ApplyPipelineRound(sessionID, goal.ID, round, RoundPhaseIdle); err != nil {
		t.Fatal(err)
	}
	got := ExtractSessionDeliverable(tm, sessionID)
	if strings.Contains(got, "explore the kernel") {
		t.Fatalf("expected formatted findings, got planning prose: %q", got)
	}
	if !strings.Contains(got, "data race") {
		t.Fatalf("expected finding title in report: %q", got)
	}
}

func TestExtractSessionDeliverable_SalvageFromWorkItemArtifact(t *testing.T) {
	// L5-D7-U-05: CC-U2 session exit salvage before raw artifact fallback
	tm := NewTaskManager()
	sessionID := "sess-salvage"
	goal, err := tm.EnsureGoal(sessionID, "review kernel")
	if err != nil {
		t.Fatal(err)
	}
	child, err := tm.CreateWorkItem(sessionID, CreateWorkItemInput{
		ParentID:  goal.ID,
		Kind:      WorkKindImplement,
		Title:     "review slice",
		Directive: "review",
	})
	if err != nil {
		t.Fatal(err)
	}
	contract := DeliverableContract{
		Citation:  DeliverableCitationFileLine,
		Severity:  DeliverableSeverityP0P1,
		Structure: DeliverableStructureFindingsJSON,
	}
	jsonSummary := "Let me summarize findings.\n```json\n{\"findings\":[{\"severity\":\"P1\",\"issue\":\"dead code\",\"file\":\"internal/foo.go\",\"line\":10,\"evidence\":\"unused func\"}]}\n```"
	childRound := &WorkItemPipelineRound{
		WorkItemID:          child.ID,
		DeliverableContract: contract,
		DeliverableStatus:   DeliverableStatusIncomplete,
		ArtifactSummary:     jsonSummary,
	}
	if err := tm.Tree().ApplyPipelineRound(sessionID, child.ID, childRound, RoundPhaseIdle); err != nil {
		t.Fatal(err)
	}
	rootRound := &WorkItemPipelineRound{
		WorkItemID:        goal.ID,
		DeliverableStatus: DeliverableStatusIncomplete,
		ArtifactSummary:   "I'll wrap up the review...",
	}
	if err := tm.Tree().ApplyPipelineRound(sessionID, goal.ID, rootRound, RoundPhaseIdle); err != nil {
		t.Fatal(err)
	}
	got := ExtractSessionDeliverable(tm, sessionID)
	if !strings.Contains(got, "dead code") {
		t.Fatalf("expected salvaged finding, got: %q", got)
	}
	if strings.Contains(got, "wrap up the review") {
		t.Fatalf("expected salvage over root planning prose, got: %q", got)
	}
}
