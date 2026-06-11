package contextengine

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

type stubPluginRunner struct {
	name string
}

func (s *stubPluginRunner) Name() string { return s.name }

func (s *stubPluginRunner) Schema() ToolSchema {
	return ToolSchema{Name: s.name, Description: "stub"}
}

func (s *stubPluginRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (s *stubPluginRunner) Execute(ctx context.Context, workDir, input string) (*ToolResult, error) {
	_ = ctx
	return &ToolResult{Output: workDir + ":" + input}, nil
}

// Covers: L5-TOOL-03
func TestToolRegistry_should_invoke_registered_plugin(t *testing.T) {
	reg := NewToolRegistry()
	if err := reg.Register(&stubPluginRunner{name: "grep_search"}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ctx := WithToolWorkDir(context.Background(), "/tmp/ws")
	result, err := reg.Execute(ctx, ToolCall{Name: "grep_search", Input: "pattern"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Output != "/tmp/ws:pattern" {
		t.Fatalf("output = %q", result.Output)
	}
}

// Covers: L5-TOOL-03
func TestToolRegistry_should_reject_duplicate_registration(t *testing.T) {
	reg := NewToolRegistry()
	if err := reg.Register(&stubPluginRunner{name: "bash"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	err := reg.Register(&stubPluginRunner{name: "bash"})
	if err == nil {
		t.Fatal("expected duplicate registration error")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("error = %q", err.Error())
	}
}

// Covers: L5-TOOL-03
func TestToolRegistry_should_return_unknown_tool_error(t *testing.T) {
	reg, err := NewBuiltinToolRegistry(nil)
	if err != nil {
		t.Fatalf("NewBuiltinToolRegistry: %v", err)
	}
	result, err := reg.Execute(context.Background(), ToolCall{Name: "nonexistent"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error == "" || !strings.Contains(result.Error, "unknown tool") {
		t.Fatalf("error = %q", result.Error)
	}
}

// Covers: L5-TOOL-03
func TestNewBuiltinToolRegistry_should_register_builtin_tools(t *testing.T) {
	reg, err := NewBuiltinToolRegistry(nil)
	if err != nil {
		t.Fatalf("NewBuiltinToolRegistry: %v", err)
	}
	for _, name := range []string{"bash", "read_file", "write_file"} {
		if reg.RiskLevel(name) == types.RiskLevelLow && name == "bash" {
			t.Fatal("bash should not be LOW risk")
		}
	}

	schemas, err := reg.ListTools(context.Background(), "")
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(schemas) != 3 {
		t.Fatalf("schema count = %d, want 3", len(schemas))
	}
	for _, s := range schemas {
		if s.Name == "" || s.Description == "" {
			t.Fatalf("invalid schema: %+v", s)
		}
	}
}

func TestNewBuiltinToolRegistry_should_set_bash_high_risk(t *testing.T) {
	reg, err := NewBuiltinToolRegistry(nil)
	if err != nil {
		t.Fatalf("NewBuiltinToolRegistry: %v", err)
	}
	if reg.RiskLevel("bash") != types.RiskLevelHigh {
		t.Fatalf("bash risk = %v, want HIGH", reg.RiskLevel("bash"))
	}
}
