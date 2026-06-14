package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/multiagent/tool"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
	"github.com/devrix/devrix/internal/shared/types"
)

// agentToolPlugin wraps a single AgentTool from D4's Registry as a D2 PluginRunner.
// Each registered agent becomes a separate "call_<name>" tool for the LLM.
type agentToolPlugin struct {
	agent  tool.AgentTool
	info   tool.Info
	bridge *observability.Bridge
}

// toolName returns the tool name exposed to LLM, e.g. "call_cursor", "call_claude-code".
func (p *agentToolPlugin) toolName() string {
	return "call_" + p.info.Name
}

// newAgentToolPlugins creates one PluginRunner per registered agent tool.
func newAgentToolPlugins(registry *tool.Registry, bridge *observability.Bridge) []contextengine.PluginRunner {
	infos := registry.List()
	plugins := make([]contextengine.PluginRunner, 0, len(infos))
	for _, info := range infos {
		agt, err := registry.Get(info.Name)
		if err != nil {
			slog.Warn("agent tool not found in registry", "name", info.Name)
			continue
		}
		plugins = append(plugins, &agentToolPlugin{
			agent:  agt,
			info:   info,
			bridge: bridge,
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

func (p *agentToolPlugin) startSpan(ctx context.Context, operation string, kind tracer.SpanKind, attrs ...tracer.Attribute) (context.Context, tracer.Span) {
	if p.bridge == nil || p.bridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(kind),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(operation, attrs...)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return p.bridge.Tracer().Start(ctx, operation, opts...)
}

func (p *agentToolPlugin) Execute(ctx context.Context, workDir, input string) (*contextengine.ToolResult, error) {
	ctx, span := p.startSpan(ctx, telemetry.OpD4_S4_Agent_Tool_Call, tracer.SpanKindInternal,
		tracer.Attribute{Key: "agent.name", Value: p.info.Name},
		tracer.Attribute{Key: "agent.tool", Value: p.toolName()},
		tracer.Attribute{Key: "task.len", Value: len(input)},
	)
	if span != nil {
		defer span.End()
	}

	var args callAgentInput
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(tracer.StatusCodeError, fmt.Sprintf("invalid input: %v", err))
		}
		return &contextengine.ToolResult{Error: fmt.Sprintf("invalid input for %s: %v", p.toolName(), err)}, nil
	}

	sessionID := contextengine.ToolSessionIDFromContext(ctx)

	if args.WorkDir == "" {
		args.WorkDir = workDir
	}
	args.WorkDir = resolveAgentWorkDir(args.WorkDir, workDir)

	execTimeout := 5 * time.Minute
	if withTimeout, ok := p.agent.(interface{ ExecutionTimeout() time.Duration }); ok {
		if d := withTimeout.ExecutionTimeout(); d > 0 {
			execTimeout = d
		}
	}
	execCtx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()

	evtCh, err := p.agent.Execute(execCtx, sessionID, tool.Request{
		Task:    args.Task,
		WorkDir: args.WorkDir,
	})
	if err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(tracer.StatusCodeError, err.Error())
		}
		return &contextengine.ToolResult{Error: fmt.Sprintf("agent %s execution failed: %v", p.info.Name, err)}, nil
	}

	var parts []string
	streamEmit := contextengine.ToolStreamEmitterFromContext(ctx)
	agentLabel := p.info.DisplayName
	if agentLabel == "" {
		agentLabel = p.info.Name
	}

	for evt := range evtCh {
		switch evt.Type {
		case "thinking":
			if streamEmit != nil && evt.Content != "" {
				streamEmit(contextengine.ToolStreamEvent{
					Type: "thinking", Content: evt.Content, ToolName: agentLabel,
				})
			}
		case "text", "tool_use":
			if streamEmit != nil && evt.Content != "" {
				streamEmit(contextengine.ToolStreamEvent{
					Type: evt.Type, Content: evt.Content, ToolName: agentLabel,
				})
			}
			if evt.Content != "" {
				parts = append(parts, evt.Content)
			}
		case "error":
			if span != nil {
				span.RecordError(fmt.Errorf("agent error: %s", evt.Content))
				span.SetStatus(tracer.StatusCodeError, evt.Content)
			}
			return &contextengine.ToolResult{Error: evt.Content}, nil
		case "complete":
			if evt.Content != "" && len(parts) == 0 {
				parts = append(parts, evt.Content)
			}
		}
	}

	if span != nil {
		span.SetAttributes(tracer.Attribute{Key: "result.len", Value: len(strings.Join(parts, "\n"))})
		span.SetStatus(tracer.StatusCodeOk, "")
	}
	return &contextengine.ToolResult{Output: strings.Join(parts, "\n")}, nil
}

func resolveAgentWorkDir(requested, sessionDir string) string {
	requested = strings.TrimSpace(requested)
	if requested != "" {
		cleaned := filepath.Clean(requested)
		if info, err := os.Stat(cleaned); err == nil && info.IsDir() {
			return cleaned
		}
		slog.Warn("agent tool: invalid work_dir, falling back to session workspace",
			"requested", requested,
			"session", sessionDir,
		)
	}
	if sessionDir = strings.TrimSpace(sessionDir); sessionDir != "" {
		cleaned := filepath.Clean(sessionDir)
		if info, err := os.Stat(cleaned); err == nil && info.IsDir() {
			return cleaned
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return filepath.Clean(cwd)
	}
	return "."
}
