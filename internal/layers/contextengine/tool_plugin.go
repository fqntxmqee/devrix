package contextengine

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// PluginRunner is a single pluggable tool implementation.
type PluginRunner interface {
	Name() string
	Schema() ToolSchema
	RiskLevel() types.RiskLevel
	Execute(ctx context.Context, workDir, input string) (*ToolResult, error)
}

// ToolRegistry manages plugin registration and implements IToolRunner + IToolRegistry.
type ToolRegistry struct {
	mu      sync.RWMutex
	runners map[string]PluginRunner
}

// NewToolRegistry creates an empty tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		runners: make(map[string]PluginRunner),
	}
}

// NewBuiltinToolRegistry registers bash, read_file, and write_file built-ins.
func NewBuiltinToolRegistry(toolCfg *config.ToolConfig) (*ToolRegistry, error) {
	if toolCfg == nil {
		toolCfg = config.DefaultToolConfig()
	}
	reg := NewToolRegistry()
	execCfg := newToolExecConfig(toolCfg)
	for _, runner := range []PluginRunner{
		newBashRunner(execCfg),
		newReadFileRunner(execCfg),
		newWriteFileRunner(execCfg),
		newGlobRunner(),
		newGrepRunner(),
		newEditFileRunner(execCfg),
	} {
		if err := reg.Register(runner); err != nil {
			return nil, fmt.Errorf("register builtin %s: %w", runner.Name(), err)
		}
	}
	return reg, nil
}

// Register adds a plugin runner. Duplicate names return an error.
func (r *ToolRegistry) Register(runner PluginRunner) error {
	if runner == nil {
		return fmt.Errorf("tool runner is nil")
	}
	name := runner.Name()
	if name == "" {
		return fmt.Errorf("tool name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.runners[name]; ok {
		return fmt.Errorf("tool already registered: %s", name)
	}
	r.runners[name] = runner
	return nil
}

// Execute dispatches a tool call to the registered plugin.
func (r *ToolRegistry) Execute(ctx context.Context, call ToolCall) (*ToolResult, error) {
	r.mu.RLock()
	runner, ok := r.runners[call.Name]
	r.mu.RUnlock()
	if !ok {
		return &ToolResult{Error: fmt.Sprintf("unknown tool: %s", call.Name)}, nil
	}

	workDir, err := ResolveToolWorkDir(ctx)
	if err != nil {
		return &ToolResult{Error: err.Error()}, nil
	}
	return runner.Execute(ctx, workDir, call.Input)
}

// ListTools returns schemas for all registered plugins.
func (r *ToolRegistry) ListTools(ctx context.Context, workDir string) ([]ToolSchema, error) {
	_ = ctx
	_ = workDir

	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.runners))
	for name := range r.runners {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]ToolSchema, 0, len(names))
	for _, name := range names {
		out = append(out, r.runners[name].Schema())
	}
	return out, nil
}

// RiskLevel returns the risk level for a registered tool.
func (r *ToolRegistry) RiskLevel(toolName string) types.RiskLevel {
	r.mu.RLock()
	runner, ok := r.runners[toolName]
	r.mu.RUnlock()
	if !ok {
		return types.RiskLevelLow
	}
	return runner.RiskLevel()
}
