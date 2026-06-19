package toolrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/shared/lsp"
	"github.com/devrix/devrix/internal/shared/types"
)

// LSPConfig 单个 LSP server 注入到 toolrunner 的配置。
type LSPConfig struct {
	Enabled         bool
	MaxServers      int
	InitTimeoutMS   int
	RequestTimeoutMS int
	Servers         []lsp.ServerConfig
}

// lspRunner — lsp tool 入口。
type lspRunner struct {
	cfg *LSPConfig
	mgr *lsp.Manager
}

// LSPRunnerAlias is an exported wrapper used by the surface package.
// It is a type alias so callers outside toolrunner can hold a stable
// reference to an LSP runner without importing the package-private
// lspRunner type. (DM-20260617-007 devrix-tool-surface-contract)
type LSPRunnerAlias = lspRunner

// NewLSPRunnerForSurface returns an LSP runner suitable for embedding in
// surface.LSPToolSurface. It mirrors newLSPRunner but is exported.
func NewLSPRunnerForSurface(cfg *LSPConfig) *lspRunner { return newLSPRunner(cfg) }

// LSPToolJSONSchema is the exported JSON schema for the lsp tool. Mirrors
// lspToolJSONSchema but is exported for use by surface.LSPToolSurface.Tools().
const LSPToolJSONSchema = lspToolJSONSchema

func newLSPRunner(cfg *LSPConfig) *lspRunner {
	if cfg == nil {
		cfg = &LSPConfig{Enabled: false}
	}
	maxServers := cfg.MaxServers
	if maxServers <= 0 {
		maxServers = 4
	}
	mgr := lsp.NewManager(nil, maxServers,
		time.Duration(cfg.InitTimeoutMS)*time.Millisecond,
		time.Duration(cfg.RequestTimeoutMS)*time.Millisecond)
	return &lspRunner{cfg: cfg, mgr: mgr}
}

func (r *lspRunner) Name() string { return "lsp" }

func (r *lspRunner) Schema() ToolSchema {
	return ToolSchema{
		Name:        "lsp",
		Description: "LSP code intelligence. operations: definition | references | incoming_calls",
		Parameters:  lspToolJSONSchema,
	}
}

func (r *lspRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

const lspToolJSONSchema = `{
  "type": "object",
  "required": ["operation"],
  "properties": {
    "operation": {"enum": ["definition", "references", "incoming_calls", "hover", "workspace_symbol"]},
    "file_path": {"type": "string", "description": "Path to the file (absolute or relative to WorkDir). Required for definition/references/incoming_calls/hover."},
    "line": {"type": "integer", "minimum": 1, "description": "1-based line number. Required for definition/references/incoming_calls/hover."},
    "character": {"type": "integer", "minimum": 1, "description": "1-based character offset. Required for definition/references/incoming_calls/hover."},
    "query": {"type": "string", "description": "Symbol search query. Required for workspace_symbol."}
  }
}`

type lspInput struct {
	Operation string `json:"operation"`
	FilePath  string `json:"file_path"`
	Line      int    `json:"line"`
	Character int    `json:"character"`
	Query     string `json:"query"`
}

func (r *lspRunner) Execute(ctx context.Context, workDir, input string) (*ToolResult, error) {
	if r.cfg == nil || !r.cfg.Enabled {
		return &ToolResult{Error: "lsp: tool is disabled (LSPConfig.Enabled=false). Set LSPConfig.Enabled=true in devrix.yaml to enable."}, nil
	}
	if len(r.cfg.Servers) == 0 {
		return &ToolResult{Error: "lsp: no servers configured. Add LSP servers to devrix.yaml."}, nil
	}
	var in lspInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return &ToolResult{Error: fmt.Sprintf("lsp: parse input: %s", err)}, nil
	}
	if in.Operation == "" {
		return &ToolResult{Error: "lsp: operation is required"}, nil
	}
	// workspace_symbol 不需要 file_path / line / character。
	if in.Operation != "workspace_symbol" {
		if in.FilePath == "" {
			return &ToolResult{Error: "lsp: file_path is required for " + in.Operation}, nil
		}
		if in.Line <= 0 || in.Character <= 0 {
			return &ToolResult{Error: "lsp: line and character must be 1-based positive integers"}, nil
		}
	}
	if in.Operation == "workspace_symbol" && in.Query == "" {
		return &ToolResult{Error: "lsp: query is required for workspace_symbol"}, nil
	}

	// 解析路径
	target := in.FilePath
	if !filepath.IsAbs(target) && workDir != "" {
		target = filepath.Join(workDir, target)
	}
	target = filepath.Clean(target)

	// 文件存在检查
	if info, err := os.Stat(target); err != nil || info.IsDir() {
		return &ToolResult{Error: fmt.Sprintf("lsp: file not found or is a directory: %s", in.FilePath)}, nil
	}

	client, err := r.mgr.Acquire(ctx, target, r.cfg.Servers)
	if err != nil {
		return &ToolResult{Error: fmt.Sprintf("lsp: acquire: %s", err)}, nil
	}

	uri := fileURI(target)
	langID := lsp.LanguageForFile(target)

	// didOpen（让 server 知道这个文件存在）
	if data, err := os.ReadFile(target); err == nil {
		_ = client.DidOpen(ctx, uri, langID, string(data))
	}

	pos := lsp.Position{
		Line:      uint32(in.Line - 1),
		Character: uint32(in.Character - 1),
	}

	switch in.Operation {
	case "definition":
		locs, err := client.Definition(ctx, uri, pos)
		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("lsp: definition: %s", err)}, nil
		}
		return &ToolResult{Output: formatLocations(locs, workDir)}, nil
	case "references":
		locs, err := client.References(ctx, uri, pos, true)
		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("lsp: references: %s", err)}, nil
		}
		return &ToolResult{Output: formatLocations(locs, workDir)}, nil
	case "incoming_calls":
		items, err := client.PrepareCallHierarchy(ctx, uri, pos)
		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("lsp: prepareCallHierarchy: %s", err)}, nil
		}
		if len(items) == 0 {
			return &ToolResult{Output: "No call hierarchy item found at this position"}, nil
		}
		calls, err := client.IncomingCalls(ctx, items[0])
		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("lsp: incomingCalls: %s", err)}, nil
		}
		return &ToolResult{Output: formatIncomingCalls(calls, workDir)}, nil
	case "hover":
		hover, err := client.Hover(ctx, uri, pos)
		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("lsp: hover: %s", err)}, nil
		}
		if hover == nil || hover.Contents == "" {
			return &ToolResult{Output: "No hover information available at this position."}, nil
		}
		return &ToolResult{Output: formatHover(hover, workDir)}, nil
	case "workspace_symbol":
		syms, err := client.WorkspaceSymbol(ctx, in.Query)
		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("lsp: workspace_symbol: %s", err)}, nil
		}
		if len(syms) == 0 {
			return &ToolResult{Output: fmt.Sprintf("No symbols matched query %q.", in.Query)}, nil
		}
		return &ToolResult{Output: formatWorkspaceSymbols(syms, workDir)}, nil
	default:
		return &ToolResult{Error: fmt.Sprintf("lsp: unsupported operation %q", in.Operation)}, nil
	}
}

func fileURI(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	return "file://" + filepath.ToSlash(abs)
}

func formatLocations(locs []lsp.Location, workDir string) string {
	if len(locs) == 0 {
		return "No results found."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Found %d location(s):\n", len(locs))
	for i, l := range locs {
		path := l.URI
		if strings.HasPrefix(path, "file://") {
			path = path[7:]
		}
		// 相对路径化
		if workDir != "" {
			if rel, err := filepath.Rel(workDir, path); err == nil && !strings.HasPrefix(rel, "..") {
				path = rel
			}
		}
		fmt.Fprintf(&b, "  %d. %s:%d:%d\n", i+1, path, l.Range.Start.Line+1, l.Range.Start.Character+1)
		if l.Preview != "" {
			fmt.Fprintf(&b, "     %s\n", l.Preview)
		}
	}
	return b.String()
}

func formatIncomingCalls(calls []lsp.CallHierarchyIncomingCall, workDir string) string {
	if len(calls) == 0 {
		return "No incoming calls."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Found %d incoming call(s):\n", len(calls))
	for i, c := range calls {
		path := c.From.URI
		if strings.HasPrefix(path, "file://") {
			path = path[7:]
		}
		if workDir != "" {
			if rel, err := filepath.Rel(workDir, path); err == nil && !strings.HasPrefix(rel, "..") {
				path = rel
			}
		}
		fmt.Fprintf(&b, "  %d. %s — %s at %s:%d\n", i+1, c.From.Name, symbolKindName(c.From.Kind), path, c.From.Range.Start.Line+1)
	}
	return b.String()
}

func symbolKindName(k lsp.SymbolKind) string {
	switch k {
	case lsp.SymbolKindFunction:
		return "function"
	case lsp.SymbolKindMethod:
		return "method"
	case lsp.SymbolKindClass:
		return "class"
	case lsp.SymbolKindInterface:
		return "interface"
	case lsp.SymbolKindVariable:
		return "variable"
	case lsp.SymbolKindField:
		return "field"
	case lsp.SymbolKindConstructor:
		return "constructor"
	}
	return "symbol"
}

// formatHover DM-20260618-007 D2-S4-A01-F04 — render hover contents.
func formatHover(h *lsp.HoverInfo, workDir string) string {
	var b strings.Builder
	b.WriteString(h.Contents)
	if h.Range != nil {
		b.WriteString("\n")
		fmt.Fprintf(&b, "at %d:%d-%d:%d",
			h.Range.Start.Line+1, h.Range.Start.Character+1,
			h.Range.End.Line+1, h.Range.End.Character+1)
	}
	return b.String()
}

// formatWorkspaceSymbols DM-20260618-007 D2-S4-A01-F05 — render workspace/symbol results.
func formatWorkspaceSymbols(syms []lsp.SymbolInformation, workDir string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Found %d symbol(s):\n", len(syms))
	for i, s := range syms {
		path := s.Location.URI
		if strings.HasPrefix(path, "file://") {
			path = path[7:]
		}
		if workDir != "" {
			if rel, err := filepath.Rel(workDir, path); err == nil && !strings.HasPrefix(rel, "..") {
				path = rel
			}
		}
		container := s.ContainerName
		if container == "" {
			container = "<global>"
		}
		fmt.Fprintf(&b, "  %d. %s — %s in %s at %s:%d\n",
			i+1, s.Name, symbolKindName(s.Kind), container,
			path, s.Location.Range.Start.Line+1)
	}
	return b.String()
}
