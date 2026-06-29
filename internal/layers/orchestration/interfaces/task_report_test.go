package interfaces

import (
	"errors"
	"sync"
	"testing"
	"time"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// TestNewTaskReport_HappyPath — D7-S20-A02-T01 sub-case 1.
func TestNewTaskReport_HappyPath(t *testing.T) {
	r, err := NewTaskReport("ts_abcdef12")
	if err != nil {
		t.Fatalf("NewTaskReport returned unexpected error: %v", err)
	}
	if r.TraceID != "ts_abcdef12" {
		t.Errorf("TraceID = %q, want ts_abcdef12", r.TraceID)
	}
	if r.Result.Kind != ResultKindPending {
		t.Errorf("default Result.Kind = %v, want ResultKindPending", r.Result.Kind)
	}
	if r.Dissent == nil {
		t.Error("Dissent should be a non-nil empty slice")
	}
	if r.Blockage == nil {
		t.Error("Blockage should be a non-nil empty slice")
	}
	if r.At.IsZero() {
		t.Error("At should be set")
	}
}

// TestNewTaskReport_EmptyTraceID — D7-S20-A02-T01 sub-case 2.
func TestNewTaskReport_EmptyTraceID(t *testing.T) {
	for _, id := range []string{"", " ", "\t"} {
		r, err := NewTaskReport(id)
		if err == nil {
			t.Errorf("NewTaskReport(%q) succeeded; want ErrTaskReportTraceIDEmpty", id)
			continue
		}
		if r != nil {
			t.Errorf("NewTaskReport(%q) returned non-nil report", id)
		}
		if !errors.Is(err, ErrTaskReportTraceIDEmpty) {
			t.Errorf("NewTaskReport(%q) error chain missing ErrTaskReportTraceIDEmpty: %v", id, err)
		}
		var sErr *sharederrors.SentinelError
		if !errors.As(err, &sErr) {
			t.Errorf("error is not a *sharederrors.SentinelError: %T", err)
		} else if sErr.Code != "ORCH_TASK_REPORT_TRACE_ID_EMPTY_7102" {
			t.Errorf("code = %q, want ORCH_TASK_REPORT_TRACE_ID_EMPTY_7102", sErr.Code)
		}
	}
}

// TestTaskReport_Validate — D7-S20-A02-T01 sub-case 3.
func TestTaskReport_Validate(t *testing.T) {
	r, err := NewTaskReport("ts_12345678")
	if err != nil {
		t.Fatalf("NewTaskReport: %v", err)
	}
	if err := r.Validate(); err != nil {
		t.Errorf("Validate on fresh report returned %v, want nil", err)
	}

	bad := *r
	bad.TraceID = ""
	if err := bad.Validate(); !errors.Is(err, ErrTaskReportTraceIDEmpty) {
		t.Errorf("Validate with empty TraceID returned %v, want ErrTaskReportTraceIDEmpty", err)
	}

	var nilR *TaskReport
	if err := nilR.Validate(); !errors.Is(err, ErrTaskReportTraceIDEmpty) {
		t.Errorf("Validate on nil report returned %v, want ErrTaskReportTraceIDEmpty", err)
	}
}

// TestTaskReport_WithImmutability — D7-S20-A02-T02.
func TestTaskReport_WithImmutability(t *testing.T) {
	base, err := NewTaskReport("ts_aaaaaaaa")
	if err != nil {
		t.Fatalf("NewTaskReport: %v", err)
	}
	originalKind := base.Result.Kind

	r1 := base.WithResult(Result{Kind: ResultKindPass, Confidence: 0.9})
	if base.Result.Kind != originalKind {
		t.Errorf("base.Result.Kind mutated: %v → %v", originalKind, base.Result.Kind)
	}
	if r1.Result.Kind != ResultKindPass {
		t.Errorf("r1.Result.Kind = %v, want ResultKindPass", r1.Result.Kind)
	}

	r2 := r1.WithEvidence(Evidence{TestResult: "5/5 pass", ArtifactHash: "deadbeef"})
	if len(base.Evidence.TestResult) != 0 {
		t.Errorf("base.Evidence mutated: %+v", base.Evidence)
	}
	if r2.Evidence.TestResult != "5/5 pass" {
		t.Errorf("r2.Evidence.TestResult = %q", r2.Evidence.TestResult)
	}

	// Distinct objects throughout.
	if r1 == r2 || base == r1 || base == r2 {
		t.Error("With* chain produced aliasing")
	}
}

// TestTaskReport_AppendDissent — D7-S20-A02-T02 + D7-S21-A01-T01.
func TestTaskReport_AppendDissent(t *testing.T) {
	base, _ := NewTaskReport("ts_bbbbbbbb")

	r1, err := base.AppendDissent(DissentEntry{
		Source: "scenario-B", Decision: "abort", Reason: "low confidence",
		Summary: "abc123", Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("AppendDissent #1: %v", err)
	}
	if len(r1.Dissent) != 1 {
		t.Errorf("after #1: len(Dissent) = %d, want 1", len(r1.Dissent))
	}
	if len(base.Dissent) != 0 {
		t.Errorf("base.Dissent mutated: %d", len(base.Dissent))
	}

	r2, _ := r1.AppendDissent(DissentEntry{Source: "scenario-C", Reason: "diverged", Summary: "def456", Timestamp: time.Now()})
	r3, _ := r2.AppendDissent(DissentEntry{Source: "scenario-D", Reason: "timeout", Summary: "ghi789", Timestamp: time.Now()})
	if len(r3.Dissent) != 3 {
		t.Errorf("after #3: len(Dissent) = %d, want 3", len(r3.Dissent))
	}

	// 4th must be silently truncated (top-N = 3).
	r4, err := r3.AppendDissent(DissentEntry{Source: "scenario-E", Reason: "yet another", Summary: "jkl012", Timestamp: time.Now()})
	if err != nil {
		t.Errorf("4th AppendDissent returned error; want silent truncation: %v", err)
	}
	if len(r4.Dissent) != 3 {
		t.Errorf("after #4: len(Dissent) = %d, want 3 (top-N truncation)", len(r4.Dissent))
	}
}

// TestTaskReport_AppendDissent_RejectsEmptyReason — D7-S21-A01-T01 negative.
func TestTaskReport_AppendDissent_RejectsEmptyReason(t *testing.T) {
	base, _ := NewTaskReport("ts_cccccccc")
	r, err := base.AppendDissent(DissentEntry{Source: "x", Decision: "y", Reason: "", Summary: "z"})
	if err == nil {
		t.Fatal("AppendDissent with empty Reason succeeded; want ErrDissentRejection")
	}
	if r != base {
		t.Error("AppendDissent on rejection must return the original receiver")
	}
	if !errors.Is(err, ErrDissentRejection) {
		t.Errorf("error chain missing ErrDissentRejection: %v", err)
	}
}

// TestTaskReport_WithBlockage — D7-S21-A02-T01.
func TestTaskReport_WithBlockage(t *testing.T) {
	base, _ := NewTaskReport("ts_dddddddd")
	r1 := base.WithBlockage(Blockage{Kind: BlockageMissing, Description: "need user input", Source: "verifier-1"})
	r2 := r1.WithBlockage(Blockage{Kind: BlockageInfeasible, Description: "tool not available", Source: "verifier-2"})
	r3 := r2.WithBlockage(Blockage{Kind: BlockageRequiredExternal, Description: "needs approval", Source: "verifier-3"})

	if len(r3.Blockage) != 3 {
		t.Errorf("Blockage count = %d, want 3", len(r3.Blockage))
	}
	if r3.Blockage[0].Kind != BlockageMissing {
		t.Errorf("Blockage[0].Kind = %v, want BlockageMissing", r3.Blockage[0].Kind)
	}
	if r3.Blockage[1].Kind != BlockageInfeasible {
		t.Errorf("Blockage[1].Kind = %v, want BlockageInfeasible", r3.Blockage[1].Kind)
	}
	if r3.Blockage[2].Kind != BlockageRequiredExternal {
		t.Errorf("Blockage[2].Kind = %v, want BlockageRequiredExternal", r3.Blockage[2].Kind)
	}

	// base must remain untouched.
	if len(base.Blockage) != 0 {
		t.Errorf("base.Blockage mutated: %d", len(base.Blockage))
	}
}

// TestBlockageKind_String — stable for spans.
func TestBlockageKind_String(t *testing.T) {
	cases := []struct {
		k    BlockageKind
		want string
	}{
		{BlockageMissing, "missing"},
		{BlockageInfeasible, "infeasible"},
		{BlockageRequiredExternal, "required_external"},
		{BlockageKind(99), "unknown"},
	}
	for _, c := range cases {
		if got := c.k.String(); got != c.want {
			t.Errorf("BlockageKind(%d).String() = %q, want %q", int(c.k), got, c.want)
		}
	}
}

// TestTaskReport_WithResource — D7-S21-A03-T01.
func TestTaskReport_WithResource(t *testing.T) {
	base, _ := NewTaskReport("ts_eeeeeeee")

	r, err := base.WithResource(Resource{
		TokensUsed: 500, TokensBudget: 1000,
		TimeElapsed: 100 * time.Millisecond, StepCount: 3, ToolInvocations: 2,
	})
	if err != nil {
		t.Fatalf("WithResource: %v", err)
	}
	if r.Resource.TokensUsed != 500 {
		t.Errorf("TokensUsed = %d, want 500", r.Resource.TokensUsed)
	}
	if r.Resource.TokensBudget != 1000 {
		t.Errorf("TokensBudget = %d, want 1000", r.Resource.TokensBudget)
	}
	if r.Resource.TimeElapsed != 100*time.Millisecond {
		t.Errorf("TimeElapsed = %v, want 100ms", r.Resource.TimeElapsed)
	}
	if r.Resource.StepCount != 3 {
		t.Errorf("StepCount = %d, want 3", r.Resource.StepCount)
	}
	if r.Resource.ToolInvocations != 2 {
		t.Errorf("ToolInvocations = %d, want 2", r.Resource.ToolInvocations)
	}
}

// TestTaskReport_WithResource_RejectsNegative — D7-S21-A03-T01 negative.
func TestTaskReport_WithResource_RejectsNegative(t *testing.T) {
	base, _ := NewTaskReport("ts_ffffffff")
	cases := []Resource{
		{TokensUsed: -1},
		{TokensBudget: -1},
		{TimeElapsed: -1 * time.Millisecond},
		{StepCount: -1},
		{ToolInvocations: -1},
	}
	for i, res := range cases {
		r, err := base.WithResource(res)
		if err == nil {
			t.Errorf("case %d: WithResource(%+v) succeeded; want ErrResourceInvalid", i, res)
		}
		if !errors.Is(err, ErrResourceInvalid) {
			t.Errorf("case %d: error chain missing ErrResourceInvalid: %v", i, err)
		}
		if r != base {
			t.Errorf("case %d: receiver mutated on rejection", i)
		}
	}
}

// TestTaskReport_WithFallbackUsed — D7-S20-A02-T02 (AC11 reserved for PR-B).
func TestTaskReport_WithFallbackUsed(t *testing.T) {
	base, _ := NewTaskReport("ts_11111111")
	r := base.WithFallbackUsed(true)
	if !r.FallbackUsed {
		t.Error("WithFallbackUsed(true) did not set FallbackUsed")
	}
	if base.FallbackUsed {
		t.Error("base.FallbackUsed mutated")
	}
}

// TestTaskReport_WithMVPArtifact — PR-B AC11 placeholder. PR-A keeps nil.
func TestTaskReport_WithMVPArtifact(t *testing.T) {
	base, _ := NewTaskReport("ts_22222222")
	mvp := &MVPArtifact{Output: "best-effort", RiskWarnings: []string{"truncated"}}
	r := base.WithMVPArtifact(mvp)
	if r.MVPArtifact == nil {
		t.Fatal("WithMVPArtifact did not set MVPArtifact")
	}
	if r.MVPArtifact.Output != "best-effort" {
		t.Errorf("MVPArtifact.Output = %q", r.MVPArtifact.Output)
	}
	if base.MVPArtifact != nil {
		t.Error("base.MVPArtifact mutated")
	}
}

// TestResultKind_String — stable for spans + LP-1 reputation lookup.
func TestResultKind_String(t *testing.T) {
	cases := []struct {
		k    ResultKind
		want string
	}{
		{ResultKindPending, "pending"},
		{ResultKindPass, "pass"},
		{ResultKindPartial, "partial"},
		{ResultKindIndeterminate, "indeterminate"},
		{ResultKindFailed, "failed"},
		{ResultKind(99), "unknown"},
	}
	for _, c := range cases {
		if got := c.k.String(); got != c.want {
			t.Errorf("ResultKind(%d).String() = %q, want %q", int(c.k), got, c.want)
		}
	}
}

// TestTaskReport_ConcurrentAppend — D7-S20-A02-T02 concurrency check.
func TestTaskReport_ConcurrentAppend(t *testing.T) {
	base, _ := NewTaskReport("ts_33333333")
	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			r, err := base.AppendDissent(DissentEntry{
				Source: "scenario-X", Decision: "abort",
				Reason: "concurrent", Summary: "sum", Timestamp: time.Now(),
			})
			if err != nil {
				t.Errorf("goroutine %d: AppendDissent: %v", i, err)
			}
			if len(r.Dissent) > DissentMaxEntries {
				t.Errorf("goroutine %d: derived spec exceeded top-N: %d", i, len(r.Dissent))
			}
		}(i)
	}
	wg.Wait()
	// Base must remain untouched.
	if len(base.Dissent) != 0 {
		t.Errorf("base.Dissent mutated under concurrency: %d", len(base.Dissent))
	}
}