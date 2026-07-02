package surface_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools/surface"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: TOOL-SURFACE-1-T03 — BuiltinSurface exposes 6 tools.
func TestBuiltinSurface_Tools(t *testing.T) {
	reg, err := tools.NewBuiltinToolRegistry(config.DefaultToolConfig())
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
	reg, _ := tools.NewBuiltinToolRegistry(config.DefaultToolConfig())
	s := surface.NewBuiltinSurface(reg)
	if s.RiskLevel("bash") == types.RiskLevelLow {
		t.Errorf("bash should not be LOW (sandbox-mediated)")
	}
	if s.RiskLevel("read_file") != types.RiskLevelLow {
		t.Errorf("read_file risk = %q, want LOW", s.RiskLevel("read_file"))
	}
	if s.RiskLevel("nonexistent") != "" {
		t.Errorf("unknown tool risk = %q, want empty (surface does not claim)", s.RiskLevel("nonexistent"))
	}
}

// T: TOOL-SURFACE-1-T03 — BuiltinSurface.Execute delegates to registry.
func TestBuiltinSurface_Execute_Glob(t *testing.T) {
	reg, _ := tools.NewBuiltinToolRegistry(config.DefaultToolConfig())
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
	reg, _ := tools.NewBuiltinToolRegistry(config.DefaultToolConfig())
	var _ contracts.ToolSurface = surface.NewBuiltinSurface(reg)
}

// T: D2-S15-A02-T17 — BuiltinSurface.IsConcurrencySafe dispatches by
// input shape (command for bash, file_path/path for read/write/edit).
// Pattern matches: AC18 8K 回归锁 — read_file is always concurrency-safe.
func TestBuiltinSurface_IsConcurrencySafe(t *testing.T) {
	reg, _ := tools.NewBuiltinToolRegistry(config.DefaultToolConfig())
	s := surface.NewBuiltinSurface(reg)

	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"bash_read_only", `{"command": "ls -la"}`, true},
		{"bash_write", `{"command": "rm -rf foo"}`, false},
		// PR-A limitation: file_path/path input shape can't disambiguate
		// read vs write vs edit (read_file = true, write/edit = false).
		// Conservative default = false (serial). T18 partitionToolCalls
		// will provide explicit tool name and use the per-tool dispatch.
		{"read_file", `{"file_path": "/a/b.go"}`, false},
		{"read_file_alt", `{"path": "/a/b.go"}`, false},
		{"write_or_edit_path", `{"file_path": "/a/b.go", "content": "x"}`, false},
		{"empty", ``, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := s.IsConcurrencySafe(json.RawMessage(c.input))
			if got != c.want {
				t.Errorf("IsConcurrencySafe(%q) = %v, want %v", c.input, got, c.want)
			}
		})
	}
}

// T: D2-S15-A02-T17 — BuiltinSurface.ToAutoClassifierInput projects
// the per-input classifier string (command for bash, file_path/path
// for read/write/edit).
func TestBuiltinSurface_ToAutoClassifierInput(t *testing.T) {
	reg, _ := tools.NewBuiltinToolRegistry(config.DefaultToolConfig())
	s := surface.NewBuiltinSurface(reg)

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"bash_command", `{"command": "ls -la"}`, "ls -la"},
		{"read_file", `{"file_path": "/a/b.go"}`, "/a/b.go"},
		{"read_file_alt", `{"path": "/a/b.go"}`, "/a/b.go"},
		{"write_file", `{"file_path": "/a/b.go", "content": "x"}`, "/a/b.go"},
		{"edit_file", `{"file_path": "/a/b.go", "old_string": "a", "new_string": "b"}`, "/a/b.go"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := s.ToAutoClassifierInput(json.RawMessage(c.input))
			if got != c.want {
				t.Errorf("ToAutoClassifierInput(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

// (avoid unused import warnings if the v4 tests above are the only users)
var _ = types.RiskLevelLow
