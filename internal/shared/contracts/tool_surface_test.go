package contracts_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// Compile-time interface compliance check: stubSurface must satisfy
// contracts.ToolSurface. This is checked at build time but explicitly
// asserted at test time so refactors cannot silently drop a method.
var _ contracts.ToolSurface = (*stubSurface)(nil)

type stubSurface struct {
	name    string
	tools   []contracts.ToolSpec
	risk    types.RiskLevel
	output  string
	execErr error
}

func (s *stubSurface) Name() string { return s.name }
func (s *stubSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	return s.tools
}
func (s *stubSurface) RiskLevel(_ string) types.RiskLevel { return s.risk }
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
