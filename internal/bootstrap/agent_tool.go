package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/multiagent/tool"
	"github.com/devrix/devrix/internal/shared/types"
)

// agentToolPlugin wraps a single AgentTool from D4's Registry as a D2 PluginRunner.
// Each registered agent becomes a separate "call_<name>" tool for the LLM.
type agentToolPlugin struct {
	agent tool.AgentTool
	info  tool.Info
}

// toolName returns the tool name exposed to LLM, e.g. "call_cursor", "call_claude-code".
func (p *agentToolPlugin) toolName() string {
	return "call_" + p.info.Name
}

// newAgentToolPlugins creates one PluginRunner per registered agent tool.
func newAgentToolPlugins(registry *tool.Registry) []contextengine.PluginRunner {
	infos := registry.List()
	plugins := make([]contextengine.PluginRunner, 0, len(infos))
	for _, info := range infos {
		agt, err := registry.Get(info.Name)
		if err != nil {
			slog.Warn("agent tool not found in registry", "name", info.Name)
			continue
		}
		plugins = append(plugins, &agentToolPlugin{
			agent: agt,
			info:  info,
		})
	}
	return plugins
}

// Name returns "call_<agent_name>" for LLM tool calling.
func (p *agentToolPlugin) Name() string { return p.toolName() }

func (p *agentToolPlugin) Schema() contextengine.ToolSchema {
	desc := p.info.Role
	if desc == "" {
		// Fallback: auto-generate from description + capabilities
		desc = p.info.Description
		if len(p.info.Capabilities) > 0 {
			desc += "。擅长: " + strings.Join(p.info.Capabilities, ", ")
		}
	}
	return contextengine.ToolSchema{
		Name:        p.toolName(),
		Description: desc,
		Parameters: `{
  "type": "object",
  "properties": {
    "task": {
      "type": "string",
      "description": "发送给 Agent 的任务描述"
    },
    "work_dir": {
      "type": "string",
      "description": "工作目录（可选，默认使用会话工作目录）"
    }
  },
  "required": ["task"]
}`,
	}
}

func (p *agentToolPlugin) RiskLevel() types.RiskLevel { return types.RiskLevelHigh }

// callAgentInput is the JSON shape LLM sends to a call_<name> tool.
type callAgentInput struct {
	Task    string `json:"task"`
	WorkDir string `json:"work_dir,omitempty"`
}

func (p *agentToolPlugin) Execute(ctx context.Context, workDir, input string) (*contextengine.ToolResult, error) {
	var args callAgentInput
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return &contextengine.ToolResult{Error: fmt.Sprintf("invalid input for %s: %v", p.toolName(), err)}, nil
	}

	sessionID := contextengine.ToolSessionIDFromContext(ctx)

	if args.WorkDir == "" {
		args.WorkDir = workDir
	}

	// Apply a default 5-minute execution timeout.
	execCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	evtCh, err := p.agent.Execute(execCtx, sessionID, tool.Request{
		Task:    args.Task,
		WorkDir: args.WorkDir,
	})
	if err != nil {
		return &contextengine.ToolResult{Error: fmt.Sprintf("agent %s execution failed: %v", p.info.Name, err)}, nil
	}

	var parts []string
	for evt := range evtCh {
		switch evt.Type {
		case "text", "tool_use":
			if evt.Content != "" {
				parts = append(parts, evt.Content)
			}
		case "error":
			return &contextengine.ToolResult{Error: evt.Content}, nil
		case "complete":
			// Done
		}
	}

	return &contextengine.ToolResult{Output: strings.Join(parts, "\n")}, nil
}
