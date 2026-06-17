package surface_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner/surface"
	"github.com/devrix/devrix/internal/shared/lsp"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: TOOL-SURFACE-1-T03 — LSPToolSurface.Tools returns nil when disabled
// (cfg == nil → defaults to Enabled=false).
func TestLSPToolSurface_Disabled(t *testing.T) {
	s := surface.NewLSPToolSurface(nil)
	if got := s.Tools(context.Background(), "", ""); got != nil {
		t.Errorf("disabled LSP Tools = %v, want nil", got)
	}
}

// T: TOOL-SURFACE-1-T03 — LSPToolSurface.Tools returns nil when Enabled
// but no servers configured.
func TestLSPToolSurface_EnabledNoServers(t *testing.T) {
	cfg := &toolrunner.LSPConfig{Enabled: true}
	s := surface.NewLSPToolSurface(cfg)
	if got := s.Tools(context.Background(), "", ""); got != nil {
		t.Errorf("enabled-no-servers Tools = %v, want nil", got)
	}
}

// T: TOOL-SURFACE-1-T03 — LSPToolSurface.Tools returns 1 spec when
// enabled with at least one server.
func TestLSPToolSurface_EnabledWithServers(t *testing.T) {
	cfg := &toolrunner.LSPConfig{
		Enabled: true,
		Servers: []lsp.ServerConfig{{LanguageID: "go", Command: []string{"gopls"}}},
	}
	s := surface.NewLSPToolSurface(cfg)
	specs := s.Tools(context.Background(), "", "")
	if len(specs) != 1 {
		t.Fatalf("len = %d, want 1", len(specs))
	}
	if specs[0].Name != "lsp" {
		t.Errorf("Name = %q, want lsp", specs[0].Name)
	}
	if specs[0].Risk != types.RiskLevelLow {
		t.Errorf("Risk = %q, want LOW", specs[0].Risk)
	}
}

// T: TOOL-SURFACE-1-T04 — LSPToolSurface.Execute returns "disabled" error
// when cfg is nil.
func TestLSPToolSurface_Execute_Disabled(t *testing.T) {
	s := surface.NewLSPToolSurface(nil)
	res, err := s.Execute(context.Background(), "lsp", `{}`, "")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Error == "" {
		t.Error("Error empty, want 'lsp: tool is disabled'")
	}
}

// T: TOOL-SURFACE-1-T04 — LSPToolSurface.Execute rejects unknown tool name.
func TestLSPToolSurface_Execute_UnknownTool(t *testing.T) {
	s := surface.NewLSPToolSurface(nil)
	res, _ := s.Execute(context.Background(), "nope", `{}`, "")
	if res.Error == "" {
		t.Error("Error empty, want 'unknown tool'")
	}
}

// T: TOOL-SURFACE-1-T04 — LSPToolSurface.RiskLevel returns LOW for "lsp"
// and LOW (defensive default) for anything else.
func TestLSPToolSurface_RiskLevel(t *testing.T) {
	s := surface.NewLSPToolSurface(nil)
	if s.RiskLevel("lsp") != types.RiskLevelLow {
		t.Errorf("lsp risk = %q, want LOW", s.RiskLevel("lsp"))
	}
	if s.RiskLevel("other") != types.RiskLevelLow {
		t.Errorf("other risk = %q, want LOW (default)", s.RiskLevel("other"))
	}
}
