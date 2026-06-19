package surface

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// LSPToolSurface exposes the LSP code-intelligence tools. It is conditional:
// Tools() returns nil unless cfg.Enabled is true AND at least one server is
// configured. This mirrors RegisterLSPTool's runtime-disabled default.
//
// DM-20260618-007 W3 — 暴露 5 个独立 spec (lsp_go_to_definition /
// lsp_find_references / lsp_incoming_calls / lsp_hover /
// lsp_workspace_symbol) 让 LLM 单独调用, 而非 1 个 spec 配 operation 参数。
// 同时每个 method 调用通过 LSPMethodLatency metrics (D5 SUG-2) 记录延迟。
type LSPToolSurface struct {
	cfg  *tools.LSPConfig
	runn *tools.LSPRunnerAlias
}

// NewLSPToolSurface constructs an LSP surface from an LSPConfig.
func NewLSPToolSurface(cfg *tools.LSPConfig) *LSPToolSurface {
	return &LSPToolSurface{cfg: cfg, runn: tools.NewLSPRunnerForSurface(cfg)}
}

// Name implements contracts.ToolSurface.
func (s *LSPToolSurface) Name() string { return "lsp" }

// LSP method names (DM-20260618-007 D2-S4-A01-F01~F05).
const (
	LSPGoToDefinition    = "lsp_go_to_definition"
	LSPFindReferences    = "lsp_find_references"
	LSPIncomingCalls     = "lsp_incoming_calls"
	LSPHover             = "lsp_hover"
	LSPWorkspaceSymbol   = "lsp_workspace_symbol"
)

// lspMethodOpMap DM-20260618-007 W3 — surface spec name -> lsp_tool operation.
var lspMethodOpMap = map[string]string{
	LSPGoToDefinition:  "definition",
	LSPFindReferences:  "references",
	LSPIncomingCalls:   "incoming_calls",
	LSPHover:           "hover",
	LSPWorkspaceSymbol: "workspace_symbol",
}

// lspMethodJSONSchemas DM-20260618-007 W3 — 每个 method 的 typed JSON schema.
// 比单 "lsp" spec + operation 更类型安全; LLM 工具选择更准确。
var lspMethodJSONSchemas = map[string]string{
	LSPGoToDefinition: `{
  "type": "object",
  "required": ["file_path", "line", "character"],
  "properties": {
    "file_path": {"type": "string", "description": "Path to the source file."},
    "line": {"type": "integer", "minimum": 1, "description": "1-based line number."},
    "character": {"type": "integer", "minimum": 1, "description": "1-based character offset."}
  }
}`,
	LSPFindReferences: `{
  "type": "object",
  "required": ["file_path", "line", "character"],
  "properties": {
    "file_path": {"type": "string"},
    "line": {"type": "integer", "minimum": 1},
    "character": {"type": "integer", "minimum": 1},
    "include_declaration": {"type": "boolean", "default": true}
  }
}`,
	LSPIncomingCalls: `{
  "type": "object",
  "required": ["file_path", "line", "character"],
  "properties": {
    "file_path": {"type": "string"},
    "line": {"type": "integer", "minimum": 1},
    "character": {"type": "integer", "minimum": 1}
  }
}`,
	LSPHover: `{
  "type": "object",
  "required": ["file_path", "line", "character"],
  "properties": {
    "file_path": {"type": "string"},
    "line": {"type": "integer", "minimum": 1},
    "character": {"type": "integer", "minimum": 1}
  }
}`,
	LSPWorkspaceSymbol: `{
  "type": "object",
  "required": ["query"],
  "properties": {
    "query": {"type": "string", "description": "Symbol name or substring to search for."}
  }
}`,
}

// Tools implements contracts.ToolSurface.
//
// DM-20260618-007 W3 — 返回 5 个 typed LSP spec (替代原 1 个 "lsp" spec)。
// 即使 LSP 关闭也返回 schemas 以便 LLM 看见工具列表 (legacy 行为)。
func (s *LSPToolSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	rOnly, dest, openW, concSafe := OrthogonalFlagFor("lsp")
	specs := make([]contracts.ToolSpec, 0, len(lspMethodOpMap))
	for name, op := range lspMethodOpMap {
		specs = append(specs, contracts.ToolSpec{
			Name:            name,
			Description:     fmt.Sprintf("LSP code intelligence: %s. Read-only.", op),
			Parameters:      lspMethodJSONSchemas[name],
			Risk:            types.RiskLevelLow,
			ReadOnly:        rOnly,
			Destructive:     dest,
			OpenWorld:       openW,
			ConcurrencySafe: concSafe,
		})
	}
	return specs
}

// InterruptBehavior implements contracts.ToolSurface.
func (s *LSPToolSurface) InterruptBehavior(name string) contracts.InterruptMode {
	return InterruptBehaviorFor(name)
}

// CheckPermission implements contracts.ToolSurface. LSP is a pure
// read-only code-intelligence tool — always Allow.
func (s *LSPToolSurface) CheckPermission(_ context.Context, _ contracts.ToolSpec, _ json.RawMessage) contracts.Decision {
	return contracts.DecisionAllow
}

// RiskLevel implements contracts.ToolSurface.
func (s *LSPToolSurface) RiskLevel(name string) types.RiskLevel {
	if _, ok := lspMethodOpMap[name]; ok {
		return types.RiskLevelLow
	}
	return ""
}

// Execute implements contracts.ToolSurface. 路由 5 个 typed method 到
// 底层 lspRunner, 同时通过 metrics.LSPMethodLatency 记录延迟 (D5 SUG-2)。
func (s *LSPToolSurface) Execute(ctx context.Context, name, input, workDir string) (*contracts.ToolResult, error) {
	op, ok := lspMethodOpMap[name]
	if !ok {
		return &contracts.ToolResult{Error: fmt.Sprintf("lsp: unknown tool %q", name)}, nil
	}
	if s.runn == nil {
		return &contracts.ToolResult{Error: "lsp: tool is disabled (LSPConfig.Enabled=false). Set LSPConfig.Enabled=true in devrix.yaml to enable."}, nil
	}
	// DM-20260618-007 W3 (SUG-2) — D5 metrics 记录 method 延迟。
	timer := metrics.StartLSPMethodTimer(ctx, name)
	res, err := s.runn.Execute(ctx, workDir, buildLSPInput(op, input))
	if err != nil {
		timer.Fail()
		return nil, fmt.Errorf("lsp: execute: %w", err)
	}
	timer.Done()
	if res == nil {
		return &contracts.ToolResult{Error: "lsp: nil result"}, nil
	}
	return &contracts.ToolResult{Output: res.Output, Error: res.Error}, nil
}

// buildLSPInput DM-20260618-007 W3 — 5 个 typed method 共享的 lspInput JSON。
// 把 typed method-specific JSON 转换为 lspRunner 期望的 lspInput schema。
// workspace_symbol 字段为 query; 其余 4 个 method 字段为 file_path/line/character (可选 include_declaration)。
func buildLSPInput(op, input string) string {
	// 解析 typed input 为通用 map
	var fields map[string]any
	if err := json.Unmarshal([]byte(input), &fields); err != nil {
		// 解析失败回退原样 (lspRunner 会报错)
		return fmt.Sprintf(`{"operation":%q}`, op)
	}
	out := map[string]any{"operation": op}
	if v, ok := fields["query"]; ok {
		out["query"] = v
	}
	if v, ok := fields["file_path"]; ok {
		out["file_path"] = v
	}
	if v, ok := fields["line"]; ok {
		out["line"] = v
	}
	if v, ok := fields["character"]; ok {
		out["character"] = v
	}
	if v, ok := fields["include_declaration"]; ok {
		out["include_declaration"] = v
	}
	b, err := json.Marshal(out)
	if err != nil {
		return fmt.Sprintf(`{"operation":%q}`, op)
	}
	return string(b)
}
