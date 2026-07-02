package toolcli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: TOOL-SURFACE-1-T11 — text output contains every surface name and tool name.
func TestListCmd_TextOutput(t *testing.T) {
	reg, err := tools.NewBuiltinToolRegistry(nil)
	if err != nil {
		t.Fatalf("NewBuiltinToolRegistry: %v", err)
	}
	surfaces := buildTestSurfaces(reg)
	cmd := &ListCmd{
		Surfaces:  surfaces,
		AgentType: "main",
		Format:    "text",
		Out:       &bytes.Buffer{},
	}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := cmd.Out.(*bytes.Buffer).String()
	if !strings.Contains(out, "=== main engine tool list") {
		t.Errorf("missing header: %s", out)
	}
	for _, want := range []string{"[builtin]", "[lsp]", "[verify]"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing surface header %q in output:\n%s", want, out)
		}
	}
	for _, want := range []string{"bash", "edit_file", "glob", "grep", "read_file", "write_file"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing builtin tool %q in output", want)
		}
	}
	if !strings.Contains(out, "lsp") {
		t.Errorf("missing lsp tool in output")
	}
}

// T: D2-i18n — default zh-CN locale localizes LLM-facing tool descriptions in CLI output.
func TestListCmd_LocalizedChineseOutput(t *testing.T) {
	reg, err := tools.NewBuiltinToolRegistry(nil)
	if err != nil {
		t.Fatalf("NewBuiltinToolRegistry: %v", err)
	}
	cmd := &ListCmd{
		Surfaces:     buildTestSurfaces(reg),
		AgentType:    "main",
		Format:       "text",
		PromptLocale: i18n.LocaleZH,
		Out:          &bytes.Buffer{},
	}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := cmd.Out.(*bytes.Buffer).String()
	if strings.Contains(out, "Execute a shell command") {
		t.Errorf("expected localized bash description, got English in:\n%s", out)
	}
	if !strings.Contains(out, "shell 命令") && !strings.Contains(out, "命令") {
		t.Errorf("expected Chinese bash description in output:\n%s", out)
	}
}

// T: TOOL-SURFACE-1-T11 — JSON output is well-formed and contains expected fields.
func TestListCmd_JSONOutput(t *testing.T) {
	reg, _ := tools.NewBuiltinToolRegistry(nil)
	surfaces := buildTestSurfaces(reg)
	cmd := &ListCmd{
		Surfaces:  surfaces,
		AgentType: "main",
		Format:    "json",
		Out:       &bytes.Buffer{},
	}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var rep jsonReport
	if err := json.Unmarshal(cmd.Out.(*bytes.Buffer).Bytes(), &rep); err != nil {
		t.Fatalf("Unmarshal: %v\n%s", err, cmd.Out.(*bytes.Buffer).String())
	}
	if rep.Agent != "main" {
		t.Errorf("agent = %q, want main", rep.Agent)
	}
	if rep.Surfaces < 2 {
		t.Errorf("surfaces = %d, want >= 2", rep.Surfaces)
	}
	if rep.Tools < 7 {
		t.Errorf("tools = %d, want >= 7", rep.Tools)
	}
	foundLSP := false
	for _, it := range rep.Items {
		if it.Name == "lsp" && it.Surface == "lsp" {
			foundLSP = true
		}
	}
	if !foundLSP {
		t.Errorf("lsp item missing in JSON: %+v", rep.Items)
	}
}

// T: TOOL-SURFACE-1-T11 — unknown format returns an error.
func TestListCmd_UnknownFormat(t *testing.T) {
	cmd := &ListCmd{Format: "yaml", Out: &bytes.Buffer{}}
	if err := cmd.Run(); err == nil {
		t.Error("expected error for unknown format")
	}
}

// T: TOOL-SURFACE-1-T11 — explore filter (Allow-by-allowlist wrapper) keeps
// builtin tools and drops the synthetic delegate_explore. Mirrors what
// toolpolicy.AsToolFilter does in production.
func TestListCmd_AgentFilterDropsDelegate(t *testing.T) {
	surfaces := []contracts.ToolSurface{
		&staticSurface{
			name: "builtin",
			specs: []contracts.ToolSpec{
				{Name: "read_file", Risk: types.RiskLevelLow},
			},
		},
		&staticSurface{
			name: "delegate_explore",
			specs: []contracts.ToolSpec{
				{Name: "delegate_explore", Risk: types.RiskLevelHigh},
			},
		},
	}
	// allowlist filter that drops delegate_*
	allow := contracts.Allow("read_file", "bash", "edit_file", "glob", "grep", "write_file")
	filtered := contracts.ApplyFilters(surfaces,
		[]contracts.ToolFilter{allow}, contracts.FilterCtx{AgentType: "explore"})
	for _, s := range filtered {
		for _, sp := range s.Tools(context.Background(), "", "") {
			if sp.Name == "delegate_explore" {
				t.Error("allow filter should drop delegate_explore, but it survived")
			}
		}
	}
}

// staticSurface is a minimal contracts.ToolSurface for tests.
type staticSurface struct {
	name  string
	specs []contracts.ToolSpec
	risk  map[string]types.RiskLevel
}

func (s *staticSurface) Name() string                                              { return s.name }
func (s *staticSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec { return s.specs }
func (s *staticSurface) RiskLevel(name string) types.RiskLevel {
	if s.risk != nil {
		if r, ok := s.risk[name]; ok {
			return r
		}
	}
	return types.RiskLevelLow
}
func (s *staticSurface) Execute(_ context.Context, _, _, _ string) (*contracts.ToolResult, error) {
	return &contracts.ToolResult{Error: "static test surface"}, nil
}
func (s *staticSurface) InterruptBehavior(_ string) contracts.InterruptMode {
	return contracts.InterruptBlock
}
func (s *staticSurface) CheckPermission(_ context.Context, _ contracts.ToolSpec, _ json.RawMessage) contracts.Decision {
	return contracts.DecisionAllow
}
func (s *staticSurface) IsConcurrencySafe(_ json.RawMessage) bool { return false }
func (s *staticSurface) ToAutoClassifierInput(_ json.RawMessage) string {
	return ""
}

// buildTestSurfaces constructs a small canonical list for testing. Mirrors
// the production BuildSurfaces output (BuiltinSurface + LSPToolSurface +
// VerifySurface) using staticSurface wrappers.
func buildTestSurfaces(reg *tools.ToolRegistry) []contracts.ToolSurface {
	tools, _ := reg.ListTools(context.Background(), "")
	specs := make([]contracts.ToolSpec, 0, len(tools))
	for _, t := range tools {
		specs = append(specs, contracts.ToolSpec{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
			Risk:        types.RiskLevelLow, // ToolSchema doesn't carry risk; default to low
		})
	}
	return []contracts.ToolSurface{
		&staticSurface{name: "builtin", specs: specs},
		&staticSurface{name: "lsp", specs: []contracts.ToolSpec{{
			Name: "lsp", Description: "LSP code intelligence", Risk: types.RiskLevelLow,
		}}},
		&staticSurface{name: "verify", specs: []contracts.ToolSpec{{
			Name: "verify_plan_execution", Description: "verify tasks.md", Risk: types.RiskLevelLow,
		}}},
	}
}
