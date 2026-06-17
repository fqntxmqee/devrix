package surface

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// LSPToolSurface exposes the LSP code-intelligence tool. It is conditional:
// Tools() returns nil unless cfg.Enabled is true AND at least one server is
// configured. This mirrors RegisterLSPTool's runtime-disabled default.
type LSPToolSurface struct {
	cfg  *toolrunner.LSPConfig
	runn *toolrunner.LSPRunnerAlias
}

// NewLSPToolSurface constructs an LSP surface from an LSPConfig.
func NewLSPToolSurface(cfg *toolrunner.LSPConfig) *LSPToolSurface {
	return &LSPToolSurface{cfg: cfg, runn: toolrunner.NewLSPRunnerForSurface(cfg)}
}

// Name implements contracts.ToolSurface.
func (s *LSPToolSurface) Name() string { return "lsp" }

// Tools implements contracts.ToolSurface.
//
// W11 phase 2c: schema is always returned so the LLM can see the tool in
// its tool list (TOOL-SURFACE-1 SoT), matching the legacy RegisterLSPTool
// behavior of exposing the schema even when LSP is disabled. Execute is
// what actually reports "lsp not enabled" at call time.
func (s *LSPToolSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	rOnly, dest, openW, concSafe := OrthogonalFlagFor("lsp")
	return []contracts.ToolSpec{{
		Name:            "lsp",
		Description:     "LSP code intelligence. operations: definition | references | incoming_calls",
		Parameters:      toolrunner.LSPToolJSONSchema,
		Risk:            types.RiskLevelLow,
		ReadOnly:        rOnly,
		Destructive:     dest,
		OpenWorld:       openW,
		ConcurrencySafe: concSafe,
	}}
}

// InterruptBehavior implements contracts.ToolSurface.
func (s *LSPToolSurface) InterruptBehavior(name string) contracts.InterruptMode {
	return InterruptBehaviorFor(name)
}

// CheckPermission implements contracts.ToolSurface. Default Allow.
func (s *LSPToolSurface) CheckPermission(_ context.Context, _ contracts.ToolSpec, _ json.RawMessage) contracts.Decision {
	return contracts.DecisionAllow
}

// RiskLevel implements contracts.ToolSurface.
func (s *LSPToolSurface) RiskLevel(name string) types.RiskLevel {
	if name == "lsp" {
		return types.RiskLevelLow
	}
	return types.RiskLevelLow
}

// Execute implements contracts.ToolSurface. Returns the same error messages
// the underlying lspRunner would, so the existing test suite for lsp_tool
// continues to pass.
func (s *LSPToolSurface) Execute(ctx context.Context, name, input, workDir string) (*contracts.ToolResult, error) {
	if name != "lsp" {
		return &contracts.ToolResult{Error: fmt.Sprintf("lsp: unknown tool %q", name)}, nil
	}
	if s.runn == nil {
		return &contracts.ToolResult{Error: "lsp: tool is disabled (LSPConfig.Enabled=false). Set LSPConfig.Enabled=true in devrix.yaml to enable."}, nil
	}
	res, err := s.runn.Execute(ctx, workDir, input)
	if err != nil {
		return nil, fmt.Errorf("lsp: execute: %w", err)
	}
	if res == nil {
		return &contracts.ToolResult{Error: "lsp: nil result"}, nil
	}
	return &contracts.ToolResult{Output: res.Output, Error: res.Error}, nil
}
