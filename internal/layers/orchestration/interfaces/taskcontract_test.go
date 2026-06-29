package interfaces

import (
	"strings"
	"testing"
	"time"
)

// TestTaskContract_RoundTrip — D7-S20-A01 + D7-S20-A02 integration: spec →
// report → spec round-trip preserves the TraceID across both contracts.
func TestTaskContract_RoundTrip(t *testing.T) {
	spec, err := NewTaskSpec("implement feature X")
	if err != nil {
		t.Fatalf("NewTaskSpec: %v", err)
	}

	report, err := NewTaskReport(spec.TraceID)
	if err != nil {
		t.Fatalf("NewTaskReport: %v", err)
	}

	if report.TraceID != spec.TraceID {
		t.Errorf("TraceID mismatch: spec=%q report=%q", spec.TraceID, report.TraceID)
	}

	// Simulate a typical pipeline: build the report through all the With* builders.
	// WithResult/WithEvidence/WithFallbackUsed return *TaskReport only;
	// WithResource/AppendDissent return (*TaskReport, error) — break the chain
	// at those boundaries.
	finalReport := report.
		WithResult(Result{Kind: ResultKindPass, Confidence: 0.92, Message: "all checks passed", At: time.Now()}).
		WithEvidence(Evidence{TestResult: "12/12 pass, coverage 91%", ArtifactHash: "abcdef1234567890"}).
		WithFallbackUsed(false)

	finalReport, err = finalReport.WithResource(Resource{
		TokensUsed: 1500, TokensBudget: 2000,
		TimeElapsed: 5 * time.Second, StepCount: 4, ToolInvocations: 3,
	})
	if err != nil {
		t.Fatalf("WithResource: %v", err)
	}
	finalReport, _ = finalReport.AppendDissent(DissentEntry{
		Source: "scenario-A", Decision: "fallback", Reason: "kept for record",
		Summary: "sum-hash", Timestamp: time.Now(),
	})

	if finalReport.TraceID != spec.TraceID {
		t.Errorf("after pipeline: TraceID mismatch: spec=%q report=%q", spec.TraceID, finalReport.TraceID)
	}
	if finalReport.Result.Kind != ResultKindPass {
		t.Errorf("Result.Kind = %v, want ResultKindPass", finalReport.Result.Kind)
	}
	if finalReport.Result.Confidence != 0.92 {
		t.Errorf("Confidence = %v, want 0.92", finalReport.Result.Confidence)
	}
	if finalReport.Resource.TokensUsed != 1500 {
		t.Errorf("Resource.TokensUsed = %d, want 1500", finalReport.Resource.TokensUsed)
	}
	if len(finalReport.Dissent) != 1 {
		t.Errorf("Dissent count = %d, want 1", len(finalReport.Dissent))
	}
	if finalReport.FallbackUsed {
		t.Error("FallbackUsed = true, want false")
	}
	if err := finalReport.Validate(); err != nil {
		t.Errorf("final report Validate: %v", err)
	}
	if err := spec.Validate(); err != nil {
		t.Errorf("spec Validate: %v", err)
	}
}

// TestTaskContract_DissentTopN — D7-S21-A01-T01 end-to-end: top-3 cap.
func TestTaskContract_DissentTopN(t *testing.T) {
	spec, _ := NewTaskSpec("exploration test")
	r, _ := NewTaskReport(spec.TraceID)
	r, _ = r.AppendDissent(DissentEntry{Source: "A", Reason: "r1", Summary: "s1", Timestamp: time.Now()})
	r, _ = r.AppendDissent(DissentEntry{Source: "B", Reason: "r2", Summary: "s2", Timestamp: time.Now()})
	r, _ = r.AppendDissent(DissentEntry{Source: "C", Reason: "r3", Summary: "s3", Timestamp: time.Now()})
	// 4th and 5th must be silently truncated.
	r, _ = r.AppendDissent(DissentEntry{Source: "D", Reason: "r4", Summary: "s4", Timestamp: time.Now()})
	r, _ = r.AppendDissent(DissentEntry{Source: "E", Reason: "r5", Summary: "s5", Timestamp: time.Now()})
	if len(r.Dissent) != DissentMaxEntries {
		t.Errorf("final Dissent count = %d, want %d", len(r.Dissent), DissentMaxEntries)
	}
	if r.Dissent[0].Source != "A" || r.Dissent[2].Source != "C" {
		t.Errorf("Dissent order broken: %+v", r.Dissent)
	}
}

// TestTaskContract_Blockage3Kind — D7-S21-A02-T01 end-to-end.
func TestTaskContract_Blockage3Kind(t *testing.T) {
	spec, _ := NewTaskSpec("blockage test")
	r, _ := NewTaskReport(spec.TraceID)
	r = r.WithBlockage(Blockage{Kind: BlockageMissing, Description: "missing", Source: "v"})
	r = r.WithBlockage(Blockage{Kind: BlockageInfeasible, Description: "infeasible", Source: "v"})
	r = r.WithBlockage(Blockage{Kind: BlockageRequiredExternal, Description: "external", Source: "v"})

	if len(r.Blockage) != 3 {
		t.Errorf("Blockage count = %d, want 3", len(r.Blockage))
	}
	// 3 kinds must coexist (no top-N cap on blockage).
	kinds := map[BlockageKind]bool{}
	for _, b := range r.Blockage {
		kinds[b.Kind] = true
	}
	for _, k := range []BlockageKind{BlockageMissing, BlockageInfeasible, BlockageRequiredExternal} {
		if !kinds[k] {
			t.Errorf("Blockage missing kind %v", k)
		}
	}
}

// TestTaskContract_ResourceFromBudget — D7-S21-A03-T01 realistic bridge shape.
func TestTaskContract_ResourceFromBudget(t *testing.T) {
	specRaw, _ := NewTaskSpec("budget test")
	spec := specRaw.WithCostBudget(CostQuota{Tokens: 1000})
	r, _ := NewTaskReport(spec.TraceID)

	// Mimic decisionplanning.Decompose.resourceFromBudget output.
	res := Resource{
		TokensUsed:      500,
		TokensBudget:    spec.CostBudget.Tokens,
		TimeElapsed:     100 * time.Millisecond,
		StepCount:       3,
		ToolInvocations: 2,
	}
	r, err := r.WithResource(res)
	if err != nil {
		t.Fatalf("WithResource: %v", err)
	}

	if r.Resource.TokensBudget != spec.CostBudget.Tokens {
		t.Errorf("Resource.TokensBudget = %d, want spec.CostBudget.Tokens = %d",
			r.Resource.TokensBudget, spec.CostBudget.Tokens)
	}
}

// TestTaskContract_TraceIDFormat — every produced spec/report pair must carry
// the canonical "ts_<8 hex>" TraceID format. Defensive against silent format
// drift.
func TestTaskContract_TraceIDFormat(t *testing.T) {
	for i := 0; i < 100; i++ {
		s, _ := NewTaskSpec("loop")
		if !strings.HasPrefix(s.TraceID, TraceIDPrefix) {
			t.Fatalf("iteration %d: TraceID %q missing prefix %q", i, s.TraceID, TraceIDPrefix)
		}
		if len(s.TraceID) != len(TraceIDPrefix)+8 {
			t.Fatalf("iteration %d: TraceID %q wrong length %d, want %d",
				i, s.TraceID, len(s.TraceID), len(TraceIDPrefix)+8)
		}
	}
}