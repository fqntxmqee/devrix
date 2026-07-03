package workmodel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateChildSpecScope_RejectsBlocklistedPath(t *testing.T) {
	ok, _ := ValidateChildSpecScope(nil, ChildSpec{ScopeIn: []string{"../etc/passwd"}}, "")
	if ok {
		t.Fatal("expected blocklisted path to be rejected")
	}
}

func TestValidateChildSpecScope_AcceptsExistingPath(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "internal", "foo")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	ok, reason := ValidateChildSpecScope(nil, ChildSpec{ScopeIn: []string{"internal/foo"}}, dir)
	if !ok {
		t.Fatalf("expected valid path, got reason %q", reason)
	}
}

func TestFilterValidatedChildSpecs_FallbackWhenAllRejected(t *testing.T) {
	parent := &WorkItem{
		ScopeContract: &ScopeContract{InScope: []string{"valid/"}},
	}
	p0Tag := DeliverableSchemaTag(FirstRegisteredDeliverableSchema())
	specs := []ChildSpec{
		{Title: "bad", ScopeIn: []string{"../escape"}, ExpectedReturn: p0Tag},
	}
	filtered := FilterValidatedChildSpecs(parent, specs, t.TempDir())
	if len(filtered) != 0 {
		t.Fatalf("filtered = %d, want 0 rejected specs", len(filtered))
	}
}

func TestPrepareDecomposeSpecs_RejectsHallucinatedScope(t *testing.T) {
	tm := NewTaskManager()
	dir := t.TempDir()
	real := filepath.Join(dir, "internal", "layers")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	tm.SetSessionWorkDir("s1", dir)
	parent, _ := tm.EnsureGoal("s1", "review")
	parent.ScopeContract = &ScopeContract{InScope: []string{"internal/layers"}}
	p0Tag := DeliverableSchemaTag(FirstRegisteredDeliverableSchema())
	round := &WorkItemPipelineRound{
		WorkItemID:     parent.ID,
		SpawnPolicy:    SpawnDecompose,
		PlanID:         "p1",
		VerdictID:      "v1",
		ObservationIDs: []string{"o1"},
		ChildSpecs: []ChildSpec{
			{Title: "real", ScopeIn: []string{"internal/layers"}, ExpectedReturn: p0Tag},
			{Title: "fake", ScopeIn: []string{"internal/hallucinated"}, ExpectedReturn: p0Tag},
		},
	}
	if err := PrepareDecomposeSpecs("s1", parent, round, tm); err != nil {
		t.Fatalf("PrepareDecomposeSpecs: %v", err)
	}
	if len(round.ChildSpecs) != 1 {
		t.Fatalf("child specs = %d, want 1 after rejecting hallucinated path", len(round.ChildSpecs))
	}
	if round.ChildSpecs[0].Title != "real" {
		t.Fatalf("remaining spec title = %q, want real", round.ChildSpecs[0].Title)
	}
}
