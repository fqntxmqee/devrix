// Package toolcli — `devrix tool list` 子命令, dump 当前 surface 列表
// (按 surface name 字母序) + 各 surface 暴露的 ToolSpec, 方便 debug 阶段
// 检查 "LLM 看到哪些 tool / 当前 agent 模式过滤后剩哪些"。
//
// DM-20260617-007 W12 (AC12, AC13): 工具面契约化后, 7 个 surface 的可见性
// 不再是注册时的 side effect, 而是 BuildSurfaces(opts) 一次构造。所以
// CLI 直接复用 BuildSurfaces 即可, 不需要 LLM stack / multi-agent /
// observability 任何其他 bootstrap 状态。
package toolcli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/devrix/devrix/internal/bootstrap"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/orchestration/toolpolicy"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// ListCmd holds the inputs needed to render a tool list. Tests populate the
// fields directly; CLI dispatch (Run) fills them from args.
type ListCmd struct {
	Surfaces  []contracts.ToolSurface
	Filters   []contracts.ToolFilter
	AgentType string // "" | "main" | "explore" | "plan" | "fix" | "delegate" | "worker" | ...
	Format    string // "text" (default) | "json"
	Out       io.Writer
}

// BuildFromConfig constructs the canonical surface + filter list from the
// loaded config. Used by Run() to avoid duplicating BuildSurfaces logic.
//
// Notes:
//   - BuiltinSurface uses a registry populated by NewBuiltinToolRegistry
//     (the same 6 builtins the production engine registers: bash, read_file,
//     write_file, glob, grep, edit_file). No side effects, no I/O.
//   - LSP/Verify are always added (BuildSurfaces contract). FreeFork /
//     Tracker are nil-safe (BuildSurfaces drops them when nil).
//   - Filters come from DefaultFilters (toolpolicy.AsToolFilter per-agent
//     drop delegate_*). Main engine has nil filter chain → all surfaces
//     pass through.
func BuildFromConfig(ctxCfg *config.ContextEngineConfig, agentType string) ([]contracts.ToolSurface, []contracts.ToolFilter, error) {
	if ctxCfg == nil {
		ctxCfg = config.DefaultContextEngineConfig()
	}
	toolCfg := config.DefaultToolConfig()
	reg, err := toolrunner.NewBuiltinToolRegistry(toolCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("build builtin registry: %w", err)
	}
	// TODO(devrix-tool-surface-contract): wire LSPConfig from ctxCfg.Diagnostics
	// so the CLI reflects devrix.yaml's lsp settings. For W12 we pass nil
	// (LSP disabled → surface.Tools() still returns the lsp schema with
	// "lsp not enabled" reported at Execute time).
	surfaces := bootstrap.BuildSurfaces(bootstrap.SurfaceBuildOpts{
		ToolReg:   reg,
		LSPConfig: nil,
		Tracker:   nil,
		Forker:    nil,
	})
	// Filter to agent-specific view if requested.
	var filters []contracts.ToolFilter
	if agentType != "" && agentType != "main" {
		filters = append(filters, toolpolicy.AsToolFilter())
	}
	return surfaces, filters, nil
}

// Run is the CLI entry point: `devrix tool list [--agent TYPE] [--format FMT]`
//
//   --agent main|explore|plan|fix|delegate|worker    default: main (no filter)
//   --format text|json                              default: text
//   --workdir DIR                                   default: $PWD
//   --config PATH                                   default: $DEVRIX_CONFIG or ./devrix.yaml
func Run(args []string) error {
	return runWith(args, os.Stdout)
}

func runWith(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("tool list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	agentType := fs.String("agent", "main", "agent type (main|explore|plan|fix|delegate|worker)")
	format := fs.String("format", "text", "output format (text|json)")
	configPath := fs.String("config", "", "config file path (default: $DEVRIX_CONFIG or ./devrix.yaml)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if out == nil {
		out = os.Stdout
	}
	// Load config (silent on missing — fallback to defaults).
	path := *configPath
	if path == "" {
		path = os.Getenv("DEVRIX_CONFIG")
	}
	if path == "" {
		path = config.FindConfigFile()
	}
	_, _, _, ctxCfg, err := config.LoadConfig(path)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	surfaces, filters, err := BuildFromConfig(ctxCfg, *agentType)
	if err != nil {
		return err
	}
	cmd := &ListCmd{
		Surfaces:  surfaces,
		Filters:   filters,
		AgentType: *agentType,
		Format:    *format,
		Out:       out,
	}
	return cmd.Run()
}

// Run renders the tool list to Out using Format.
func (c *ListCmd) Run() error {
	ctx := contracts.FilterCtx{AgentType: c.AgentType}
	filtered := contracts.ApplyFilters(c.Surfaces, c.Filters, ctx)
	switch c.Format {
	case "json":
		return c.renderJSON(filtered)
	case "text", "":
		return c.renderText(filtered)
	default:
		return fmt.Errorf("unknown format %q (text|json)", c.Format)
	}
}

func (c *ListCmd) renderText(surfaces []contracts.ToolSurface) error {
	totalTools := 0
	for _, s := range surfaces {
		totalTools += len(s.Tools(context.Background(), "", ""))
	}
	header := fmt.Sprintf("=== %s engine tool list (%d surfaces, %d tools) ===\n",
		agentLabel(c.AgentType), len(surfaces), totalTools)
	if _, err := io.WriteString(c.Out, header); err != nil {
		return err
	}
	// Stable order: by surface name.
	sorted := make([]contracts.ToolSurface, len(surfaces))
	copy(sorted, surfaces)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name() < sorted[j].Name() })
	for _, s := range sorted {
		specs := s.Tools(context.Background(), "", "")
		if _, err := fmt.Fprintf(c.Out, "\n[%s] %d tools\n", s.Name(), len(specs)); err != nil {
			return err
		}
		sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })
		for _, sp := range specs {
			risk := sp.Risk
			if risk == "" {
				risk = types.RiskLevelLow
			}
			if _, err := fmt.Fprintf(c.Out, "  - %-32s %-8s  %s\n", sp.Name, strings.ToUpper(string(risk)), trim(sp.Description, 80)); err != nil {
				return err
			}
		}
	}
	return nil
}

type jsonTool struct {
	Surface string `json:"surface"`
	Name    string `json:"name"`
	Risk    string `json:"risk"`
	Desc    string `json:"description"`
}

type jsonReport struct {
	Agent    string     `json:"agent"`
	Surfaces int        `json:"surface_count"`
	Tools    int        `json:"tool_count"`
	Items    []jsonTool `json:"items"`
}

func (c *ListCmd) renderJSON(surfaces []contracts.ToolSurface) error {
	rep := jsonReport{Agent: c.AgentType}
	for _, s := range surfaces {
		specs := s.Tools(context.Background(), "", "")
		rep.Surfaces++
		sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })
		for _, sp := range specs {
			risk := sp.Risk
			if risk == "" {
				risk = types.RiskLevelLow
			}
			rep.Tools++
			rep.Items = append(rep.Items, jsonTool{
				Surface: s.Name(),
				Name:    sp.Name,
				Risk:    strings.ToUpper(string(risk)),
				Desc:    sp.Description,
			})
		}
	}
	bz, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	_, err = c.Out.Write(append(bz, '\n'))
	return err
}

func agentLabel(t string) string {
	if t == "" {
		return "main"
	}
	return t
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
