package surface_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner/surface"
	"github.com/devrix/devrix/internal/shared/lsp"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D2-S4-A01-T04 — LSPToolSurface.Tools returns 5 typed method specs
// (DM-20260618-007 W3 拆 5 spec 替代原 1 个 "lsp" spec)。
// 即使 LSP 关闭也返回 schemas 以便 LLM 看见工具列表 (legacy 行为)。
func TestLSPToolSurface_Disabled(t *testing.T) {
	s := surface.NewLSPToolSurface(nil)
	specs := s.Tools(context.Background(), "", "")
	if len(specs) != 5 {
		t.Errorf("disabled LSP Tools = %d specs, want 5", len(specs))
	}
	wantNames := map[string]bool{
		"lsp_go_to_definition": false,
		"lsp_find_references": false,
		"lsp_incoming_calls":  false,
		"lsp_hover":           false,
		"lsp_workspace_symbol": false,
	}
	for _, spec := range specs {
		if _, ok := wantNames[spec.Name]; !ok {
			t.Errorf("unexpected spec name: %q", spec.Name)
		}
		wantNames[spec.Name] = true
	}
	for name, seen := range wantNames {
		if !seen {
			t.Errorf("missing spec name: %q", name)
		}
	}
}

// T: D2-S4-A01-T04 — enabled with no servers still returns 5 specs (schemas
// 永远暴露; Execute 报 "no servers configured" 错误)。
func TestLSPToolSurface_EnabledNoServers(t *testing.T) {
	cfg := &toolrunner.LSPConfig{Enabled: true}
	s := surface.NewLSPToolSurface(cfg)
	specs := s.Tools(context.Background(), "", "")
	if len(specs) != 5 {
		t.Errorf("enabled-no-servers Tools = %d specs, want 5", len(specs))
	}
}

// T: D2-S4-A01-T04 — enabled with at least one server returns 5 specs with
// LOW risk + ReadOnly flags。
func TestLSPToolSurface_EnabledWithServers(t *testing.T) {
	cfg := &toolrunner.LSPConfig{
		Enabled: true,
		Servers: []lsp.ServerConfig{{LanguageID: "go", Command: []string{"gopls"}}},
	}
	s := surface.NewLSPToolSurface(cfg)
	specs := s.Tools(context.Background(), "", "")
	if len(specs) != 5 {
		t.Fatalf("len = %d, want 5", len(specs))
	}
	for _, spec := range specs {
		if spec.Risk != types.RiskLevelLow {
			t.Errorf("%s Risk = %q, want LOW", spec.Name, spec.Risk)
		}
		if !spec.ReadOnly {
			t.Errorf("%s ReadOnly = false, want true", spec.Name)
		}
	}
}

// T: D2-S4-A01-T04 — LSPToolSurface.Execute returns "disabled" error
// when cfg is nil.
func TestLSPToolSurface_Execute_Disabled(t *testing.T) {
	s := surface.NewLSPToolSurface(nil)
	res, err := s.Execute(context.Background(), "lsp_go_to_definition", `{"file_path":"a.go","line":1,"character":1}`, "")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Error == "" {
		t.Error("Error empty, want 'lsp: tool is disabled'")
	}
}

// T: D2-S4-A01-T04 — LSPToolSurface.Execute rejects unknown tool name.
func TestLSPToolSurface_Execute_UnknownTool(t *testing.T) {
	s := surface.NewLSPToolSurface(nil)
	res, _ := s.Execute(context.Background(), "nope", `{}`, "")
	if res.Error == "" {
		t.Error("Error empty, want 'unknown tool'")
	}
}

// T: D2-S4-A01-T04 — RiskLevel 返回 LOW 对所有 5 个 LSP method + 任何其他工具
// (防御性 default)。
func TestLSPToolSurface_RiskLevel(t *testing.T) {
	s := surface.NewLSPToolSurface(nil)
	for _, name := range []string{
		"lsp_go_to_definition", "lsp_find_references",
		"lsp_incoming_calls", "lsp_hover", "lsp_workspace_symbol",
	} {
		if s.RiskLevel(name) != types.RiskLevelLow {
			t.Errorf("%s risk = %q, want LOW", name, s.RiskLevel(name))
		}
	}
	if s.RiskLevel("other") != "" {
		t.Errorf("other risk = %q, want empty (surface does not claim)", s.RiskLevel("other"))
	}
}
