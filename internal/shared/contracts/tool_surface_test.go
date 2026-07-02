package contracts_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// Compile-time interface compliance check: stubSurface must satisfy
// contracts.ToolSurface. This is checked at build time but explicitly
// asserted at test time so refactors cannot silently drop a method.
var _ contracts.ToolSurface = (*stubSurface)(nil)

type stubSurface struct {
	name       string
	tools      []contracts.ToolSpec
	risk       types.RiskLevel
	interrupt  contracts.InterruptMode
	output     string
	execErr    error
	permReturn contracts.Decision
}

func (s *stubSurface) Name() string { return s.name }
func (s *stubSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	return s.tools
}
func (s *stubSurface) RiskLevel(_ string) types.RiskLevel { return s.risk }
func (s *stubSurface) InterruptBehavior(_ string) contracts.InterruptMode {
	if s.interrupt != "" {
		return s.interrupt
	}
	return contracts.InterruptBlock
}
func (s *stubSurface) CheckPermission(_ context.Context, _ contracts.ToolSpec, _ json.RawMessage) contracts.Decision {
	if s.permReturn != "" {
		return s.permReturn
	}
	return contracts.DecisionAllow
}
func (s *stubSurface) IsConcurrencySafe(_ json.RawMessage) bool { return false }
func (s *stubSurface) ToAutoClassifierInput(_ json.RawMessage) string {
	return ""
}
func (s *stubSurface) Execute(_ context.Context, _, _, _ string) (*contracts.ToolResult, error) {
	if s.execErr != nil {
		return nil, s.execErr
	}
	return &contracts.ToolResult{Output: s.output}, nil
}

// T: TOOL-SURFACE-1-T01 — ToolSurface interface has 4 methods and is
// implementable by any struct that exposes Name/Tools/RiskLevel/Execute.
func TestToolSurface_InterfaceCompliance(t *testing.T) {
	s := &stubSurface{
		name:   "stub",
		tools:  []contracts.ToolSpec{{Name: "t1"}},
		risk:   types.RiskLevelHigh,
		output: "ok",
	}
	if s.Name() != "stub" {
		t.Errorf("Name() = %q, want stub", s.Name())
	}
	if got := s.Tools(context.Background(), "", ""); len(got) != 1 || got[0].Name != "t1" {
		t.Errorf("Tools() = %+v, want 1 tool named t1", got)
	}
	if s.RiskLevel("anything") != types.RiskLevelHigh {
		t.Errorf("RiskLevel = %q, want HIGH", s.RiskLevel("anything"))
	}
	res, err := s.Execute(context.Background(), "t1", "{}", "/tmp")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Output != "ok" {
		t.Errorf("Execute output = %q, want ok", res.Output)
	}
}

// T: TOOL-SURFACE-1-T01 — ToolSpec is a struct with the expected fields.
func TestToolSpec_Struct(t *testing.T) {
	spec := contracts.ToolSpec{
		Name:        "free_fork",
		Description: "Batch fork N agents",
		Parameters:  `{"requests":[]}`,
		Risk:        types.RiskLevelHigh,
	}
	if spec.Name != "free_fork" {
		t.Errorf("Name = %q, want free_fork", spec.Name)
	}
	if spec.Risk != types.RiskLevelHigh {
		t.Errorf("Risk = %q, want HIGH", spec.Risk)
	}
}

// T: TOOL-SURFACE-1-T01 — ToolResult distinguishes Output vs Error.
func TestToolResult_Struct(t *testing.T) {
	r := contracts.ToolResult{Output: "hello"}
	if r.Error != "" {
		t.Errorf("Error = %q, want empty", r.Error)
	}
	if r.Output != "hello" {
		t.Errorf("Output = %q, want hello", r.Output)
	}
	r2 := contracts.ToolResult{Error: "permission denied"}
	if r2.Output != "" {
		t.Errorf("Output = %q, want empty on error", r2.Output)
	}
	if r2.Error != "permission denied" {
		t.Errorf("Error = %q, want permission denied", r2.Error)
	}
}

// T: TOOL-SURFACE-1-T01 — Surface may return an error from Execute
// (e.g. surface not initialized); the runner must propagate it.
func TestToolSurface_ExecuteError(t *testing.T) {
	s := &stubSurface{execErr: context.DeadlineExceeded}
	_, err := s.Execute(context.Background(), "t1", "{}", "/tmp")
	if err == nil {
		t.Fatal("Execute: expected error, got nil")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("Execute err = %v, want DeadlineExceeded", err)
	}
}

// T: D2-S15-A02-T12 — ToolSpec v3 has the 6 control plane fields and
// reasonable zero-value defaults. Position-struct-literal compatibility
// gate: the v3 fields are appended to the struct so existing named-field
// literals (the only style in the codebase) remain source-compatible.
func TestToolSpec_V3Fields_Exist(t *testing.T) {
	spec := contracts.ToolSpec{Name: "x"}
	// Type checks via assignment (the Go compiler enforces existence).
	var ec contracts.EmissionClass = spec.EmissionClass
	var cc contracts.ConvergenceContract = spec.ConvergenceContract
	var ib contracts.IterationBound = spec.IterationBound
	var su contracts.SourceUncertainty = spec.SourceUncertainty
	var max int = spec.MaxResultSizeChars
	var marker string = spec.TruncateMarkerText
	_ = ec
	_ = cc
	_ = ib
	_ = su
	_ = max
	_ = marker
}

// T: D2-S15-A02-T12 — Zero-value defaults per R3 cycle 0:
//   EmissionClass       = EC_Action (0)
//   ConvergenceContract = CC_None (0)
//   IterationBound      = IB_OpenEnded (0)
//   SourceUncertainty   = SK_Deterministic (0) + Value 0
//   MaxResultSizeChars  = 0 (no cap)
//   TruncateMarkerText  = "" (caller MUST set DefaultTruncateMarkerText)
func TestToolSpec_V3ZeroDefaults(t *testing.T) {
	spec := contracts.ToolSpec{Name: "x"}
	if spec.EmissionClass != contracts.EC_Action {
		t.Errorf("zero EmissionClass = %v, want EC_Action (0)", spec.EmissionClass)
	}
	if spec.ConvergenceContract.Kind != contracts.CC_None {
		t.Errorf("zero ConvergenceContract.Kind = %v, want CC_None (0)", spec.ConvergenceContract.Kind)
	}
	if spec.IterationBound.Kind != contracts.IB_OpenEnded {
		t.Errorf("zero IterationBound.Kind = %v, want IB_OpenEnded (0)", spec.IterationBound.Kind)
	}
	if spec.SourceUncertainty.Source != contracts.SK_Deterministic {
		t.Errorf("zero SourceUncertainty.Source = %v, want SK_Deterministic (0)", spec.SourceUncertainty.Source)
	}
	if spec.MaxResultSizeChars != 0 {
		t.Errorf("zero MaxResultSizeChars = %d, want 0", spec.MaxResultSizeChars)
	}
	if spec.TruncateMarkerText != "" {
		t.Errorf("zero TruncateMarkerText = %q, want empty", spec.TruncateMarkerText)
	}
}

// T: D2-S15-A02-T12 — EmissionClass String() returns the symbolic name
// used in logs / metrics / JSON tags.
func TestEmissionClass_String(t *testing.T) {
	cases := map[contracts.EmissionClass]string{
		contracts.EC_Action:     "Action",
		contracts.EC_Fact:       "Fact",
		contracts.EC_Probe:      "Probe",
		contracts.EC_Experiment: "Experiment",
	}
	for c, want := range cases {
		if got := c.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", c, got, want)
		}
	}
	if got := contracts.EmissionClass(99).String(); got != "Unknown" {
		t.Errorf("99.String() = %q, want Unknown", got)
	}
}

// T: D2-S15-A02-T12 — SourceKind / ConvergenceKind / IterationBoundKind
// all have a String() that returns the symbolic name.
func TestV3Kind_Strings(t *testing.T) {
	if got := contracts.CC_StateChangeRequired.String(); got != "StateChangeRequired" {
		t.Errorf("CC_StateChangeRequired.String() = %q", got)
	}
	if got := contracts.IB_Bounded.String(); got != "Bounded" {
		t.Errorf("IB_Bounded.String() = %q", got)
	}
	if got := contracts.SK_LLM.String(); got != "LLM" {
		t.Errorf("SK_LLM.String() = %q", got)
	}
}

// T: D2-S15-A02-T12 — DefaultTruncateMarkerText is the marker template
// with %d placeholders for (chars kept, total chars).
func TestDefaultTruncateMarkerText(t *testing.T) {
	got := contracts.DefaultTruncateMarkerText
	if !strings.Contains(got, "complete=false") {
		t.Errorf("DefaultTruncateMarkerText missing complete=false: %q", got)
	}
	if !strings.Contains(got, "%d") {
		t.Errorf("DefaultTruncateMarkerText missing %%d placeholders: %q", got)
	}
}
