package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/multiagent/tool"
	"github.com/devrix/devrix/internal/shared/types"
)

// agentToolPlugin bridges D4's tool.Registry to D2's PluginRunner.
type agentToolPlugin struct {
	registry *tool.Registry
}

// callAgentInput is the JSON shape LLM sends to call_agent.
type callAgentInput struct {
	AgentName string `json:"agent_name"`
	Task      string `json:"task"`
	WorkDir   string `json:"work_dir,omitempty"`
}

func newAgentToolPlugin(registry *tool.Registry) *agentToolPlugin {
	return &agentToolPlugin{registry: registry}
}

func (p *agentToolPlugin) Name() string { return "call_agent" }

func (p *agentToolPlugin) Schema() contextengine.ToolSchema {
	tools := p.registry.List()
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Name)
	}

	enumJSON, _ := json.Marshal(names)
	params := fmt.Sprintf(`{
  "type": "object",
  "properties": {
    "agent_name": {
      "type": "string",
      "enum": %s,
      "description": "目标 Agent 工具"
    },
    "task": {
      "type": "string",
      "description": "发送给 Agent 的任务描述"
    },
    "work_dir": {
      "type": "string",
      "description": "工作目录（可选，默认使用会话工作目录）"
    }
  },
  "required": ["agent_name", "task"]
}`, string(enumJSON))

	return contextengine.ToolSchema{
		Name:        "call_agent",
		Description: "调用外部 Agent 工具执行任务并返回结果。可用工具: " + strings.Join(names, ", "),
		Parameters:  params,
	}
}

func (p *agentToolPlugin) RiskLevel() types.RiskLevel { return types.RiskLevelHigh }

func (p *agentToolPlugin) Execute(ctx context.Context, workDir, input string) (*contextengine.ToolResult, error) {
	var args callAgentInput
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return &contextengine.ToolResult{Error: fmt.Sprintf("invalid call_agent input: %v", err)}, nil
	}

	agentTool, err := p.registry.Get(args.AgentName)
	if err != nil {
		return &contextengine.ToolResult{Error: fmt.Sprintf("unknown agent tool: %s", args.AgentName)}, nil
	}

	sessionID := contextengine.ToolSessionIDFromContext(ctx)

	if args.WorkDir == "" {
		args.WorkDir = workDir
	}

	req := tool.Request{
		Task:    args.Task,
		WorkDir: args.WorkDir,
	}

	// Apply a default 5-minute execution timeout.
	execCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	evtCh, err := agentTool.Execute(execCtx, sessionID, req)
	if err != nil {
		return &contextengine.ToolResult{Error: fmt.Sprintf("agent tool execution failed: %v", err)}, nil
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
