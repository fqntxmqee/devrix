package surface_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner/surface"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: TOOL-SURFACE-1-T03 — BuiltinSurface exposes 6 tools.
func TestBuiltinSurface_Tools(t *testing.T) {
	reg, err := toolrunner.NewBuiltinToolRegistry(config.DefaultToolConfig())
	if err != nil {
		t.Fatalf("NewBuiltinToolRegistry: %v", err)
	}
	s := surface.NewBuiltinSurface(reg)
	if s.Name() != "builtin" {
		t.Errorf("Name = %q, want builtin", s.Name())
	}
	specs := s.Tools(context.Background(), "", "session-x")
	if len(specs) != 6 {
		t.Errorf("len(Tools) = %d, want 6 (bash, read_file, write_file, edit_file, glob, grep)", len(specs))
	}
	names := make(map[string]bool)
	for _, sp := range specs {
		names[sp.Name] = true
		if sp.Risk == "" {
			t.Errorf("tool %q has empty Risk", sp.Name)
		}
	}
	for _, want := range []string{"bash", "read_file", "write_file", "edit_file", "glob", "grep"} {
		if !names[want] {
			t.Errorf("missing tool %q in surface.Tools()", want)
		}
	}
}

// T: TOOL-SURFACE-1-T03 — BuiltinSurface.RiskLevel delegates to registry.
func TestBuiltinSurface_RiskLevel(t *testing.T) {
	reg, _ := toolrunner.NewBuiltinToolRegistry(config.DefaultToolConfig())
	s := surface.NewBuiltinSurface(reg)
	if s.RiskLevel("bash") == types.RiskLevelLow {
		t.Errorf("bash should not be LOW (sandbox-mediated)")
	}
	if s.RiskLevel("read_file") != types.RiskLevelLow {
		t.Errorf("read_file risk = %q, want LOW", s.RiskLevel("read_file"))
	}
	if s.RiskLevel("nonexistent") != types.RiskLevelLow {
		t.Errorf("unknown tool risk = %q, want LOW (default)", s.RiskLevel("nonexistent"))
	}
}

// T: TOOL-SURFACE-1-T03 — BuiltinSurface.Execute delegates to registry.
func TestBuiltinSurface_Execute_Glob(t *testing.T) {
	reg, _ := toolrunner.NewBuiltinToolRegistry(config.DefaultToolConfig())
	s := surface.NewBuiltinSurface(reg)
	res, err := s.Execute(context.Background(), "glob", `{"pattern":"*"}`, t.TempDir())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Error != "" {
		t.Errorf("Error = %q, want empty", res.Error)
	}
}

// T: TOOL-SURFACE-1-T03 — BuiltinSurface with nil registry is safe.
func TestBuiltinSurface_NilRegistry(t *testing.T) {
	s := surface.NewBuiltinSurface(nil)
	if got := s.Tools(context.Background(), "", ""); got != nil {
		t.Errorf("nil reg Tools = %v, want nil", got)
	}
	res, err := s.Execute(context.Background(), "read_file", `{"path":"/tmp"}`, "")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Error == "" {
		t.Error("Execute Error empty, want 'registry not initialized'")
	}
}

// T: TOOL-SURFACE-1-T04 — BuiltinSurface implements contracts.ToolSurface
// (compile-time check, but also runtime sanity).
func TestBuiltinSurface_InterfaceCompliance(t *testing.T) {
	reg, _ := toolrunner.NewBuiltinToolRegistry(config.DefaultToolConfig())
	var _ contracts.ToolSurface = surface.NewBuiltinSurface(reg)
}
