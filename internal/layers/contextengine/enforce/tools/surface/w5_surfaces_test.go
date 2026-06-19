package surface_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner/surface"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// stubPluginRunner is a hand-rolled PluginRunner for surface tests. It is
// NOT used outside the test package and is the minimum needed to drive
// the dispatch logic in PluginSurface.
type stubPluginRunner struct {
	name string
	risk types.RiskLevel
	out  string
	err  string
	fail error
}

func (s *stubPluginRunner) Name() string { return s.name }
func (s *stubPluginRunner) RiskLevel() types.RiskLevel {
	if s.risk == "" {
		return types.RiskLevelLow
	}
	return s.risk
}
func (s *stubPluginRunner) Schema() toolrunner.ToolSchema {
	return toolrunner.ToolSchema{
		Name:        s.name,
		Description: "stub " + s.name,
		Parameters:  `{"type":"object"}`,
	}
}
func (s *stubPluginRunner) Execute(_ context.Context, _, _ string) (*toolrunner.ToolResult, error) {
	if s.fail != nil {
		return nil, s.fail
	}
	return &toolrunner.ToolResult{Output: s.out, Error: s.err}, nil
}

// T: TOOL-SURFACE-1-T05 — PluginSurface dispatches by name and returns
// the runner's output verbatim.
func TestPluginSurface_Dispatch_HappyPath(t *testing.T) {
	s := surface.NewPluginSurface("test", []toolrunner.PluginRunner{
		&stubPluginRunner{name: "alpha", out: "alpha-out"},
		&stubPluginRunner{name: "beta", out: "beta-out"},
	})
	res, err := s.Execute(context.Background(), "beta", `{}`, "")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("Error = %q", res.Error)
	}
	if res.Output != "beta-out" {
		t.Errorf("Output = %q, want beta-out", res.Output)
	}
}

// T: TOOL-SURFACE-1-T05 — PluginSurface returns "unknown tool" envelope
// for names not in the dispatch table.
func TestPluginSurface_Dispatch_UnknownTool(t *testing.T) {
	s := surface.NewPluginSurface("test", []toolrunner.PluginRunner{
		&stubPluginRunner{name: "alpha"},
	})
	res, _ := s.Execute(context.Background(), "missing", `{}`, "")
	if res.Error == "" {
		t.Error("Error empty, want 'unknown tool'")
	}
}

// T: TOOL-SURFACE-1-T05 — PluginSurface propagates runner Go errors as
// the wrapped error return (NOT into the ToolResult envelope).
func TestPluginSurface_Dispatch_RunnerGoError(t *testing.T) {
	s := surface.NewPluginSurface("test", []toolrunner.PluginRunner{
		&stubPluginRunner{name: "alpha", fail: errors.New("boom")},
	})
	_, err := s.Execute(context.Background(), "alpha", `{}`, "")
	if err == nil {
		t.Fatal("err nil, want non-nil Go error")
	}
}

// T: TOOL-SURFACE-1-T05 — PluginSurface.Tools preserves the order the
// runners were passed in (NOT map iteration order).
func TestPluginSurface_Tools_OrderPreserved(t *testing.T) {
	s := surface.NewPluginSurface("test", []toolrunner.PluginRunner{
		&stubPluginRunner{name: "c"},
		&stubPluginRunner{name: "a"},
		&stubPluginRunner{name: "b"},
	})
	specs := s.Tools(context.Background(), "", "")
	if len(specs) != 3 {
		t.Fatalf("len = %d, want 3", len(specs))
	}
	want := []string{"c", "a", "b"}
	for i, w := range want {
		if specs[i].Name != w {
			t.Errorf("specs[%d].Name = %q, want %q", i, specs[i].Name, w)
		}
	}
}

// T: TOOL-SURFACE-1-T05 — Duplicate runner names: the first wins, the
// rest are silently dropped.
func TestPluginSurface_DuplicateRunners_FirstWins(t *testing.T) {
	s := surface.NewPluginSurface("test", []toolrunner.PluginRunner{
		&stubPluginRunner{name: "x", out: "first"},
		&stubPluginRunner{name: "x", out: "second"},
	})
	specs := s.Tools(context.Background(), "", "")
	if len(specs) != 1 {
		t.Fatalf("len = %d, want 1 (duplicate dropped)", len(specs))
	}
	res, _ := s.Execute(context.Background(), "x", `{}`, "")
	if res.Output != "first" {
		t.Errorf("Output = %q, want first", res.Output)
	}
}

// T: TOOL-SURFACE-1-T05 — nil runners in the input are dropped, not stored.
func TestPluginSurface_NilRunnersIgnored(t *testing.T) {
	s := surface.NewPluginSurface("test", []toolrunner.PluginRunner{
		nil,
		&stubPluginRunner{name: "x"},
		nil,
	})
	if got := s.Tools(context.Background(), "", ""); len(got) != 1 {
		t.Errorf("len = %d, want 1 (nils dropped)", len(got))
	}
}

// T: TOOL-SURFACE-1-T05 — Empty runner list: surface is empty but safe.
func TestPluginSurface_Empty_Graceful(t *testing.T) {
	s := surface.NewPluginSurface("test", nil)
	if got := s.Tools(context.Background(), "", ""); got != nil {
		t.Errorf("Tools = %v, want nil", got)
	}
	if got := s.RiskLevel("x"); got != types.RiskLevelLow {
		t.Errorf("RiskLevel = %q, want LOW default", got)
	}
	res, _ := s.Execute(context.Background(), "x", `{}`, "")
	if res.Error == "" {
		t.Error("Error empty, want 'unknown tool'")
	}
}

// T: TOOL-SURFACE-1-T05 — DelegateSurface name is "delegate" and the
// constructor wraps delegate_* + delegate_status runners.
func TestDelegateSurface_Dispatch_Explore(t *testing.T) {
	s := surface.NewDelegateSurface(
		&stubPluginRunner{name: "delegate_explore", out: `{"status":"started"}`},
		&stubPluginRunner{name: "delegate_plan"},
		&stubPluginRunner{name: "delegate_implement"},
		&stubPluginRunner{name: "delegate_status"},
	)
	if s.Name() != "delegate" {
		t.Errorf("Name = %q, want delegate", s.Name())
	}
	res, _ := s.Execute(context.Background(), "delegate_explore", `{"directive":"look around"}`, "")
	if res.Error != "" {
		t.Fatalf("Error = %q", res.Error)
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(res.Output), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["status"] != "started" {
		t.Errorf("status = %q, want started", out["status"])
	}
}

// T: TOOL-SURFACE-1-T05 — BackgroundTaskSurface name is "background" and
// dispatches task_stop / task_output / task_list_background.
func TestBackgroundTaskSurface_Execute_TaskOutput(t *testing.T) {
	s := surface.NewBackgroundTaskSurface(
		&stubPluginRunner{name: "task_stop", risk: types.RiskLevelMedium},
		&stubPluginRunner{name: "task_output", out: `{"status":"completed"}`},
		&stubPluginRunner{name: "task_list_background"},
	)
	if s.Name() != "background" {
		t.Errorf("Name = %q, want background", s.Name())
	}
	if got := s.RiskLevel("task_stop"); got != types.RiskLevelMedium {
		t.Errorf("RiskLevel(task_stop) = %q, want MEDIUM", got)
	}
	res, _ := s.Execute(context.Background(), "task_output", `{"task_id":"bg_1"}`, "")
	if res.Error != "" {
		t.Fatalf("Error = %q", res.Error)
	}
	if res.Output != `{"status":"completed"}` {
		t.Errorf("Output = %q", res.Output)
	}
}

// T: TOOL-SURFACE-1-T05 — All 2 new surfaces satisfy contracts.ToolSurface.
func TestW5_InterfaceCompliance(t *testing.T) {
	var _ contracts.ToolSurface = surface.NewDelegateSurface()
	var _ contracts.ToolSurface = surface.NewBackgroundTaskSurface()
}
