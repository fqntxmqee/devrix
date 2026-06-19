package contextengine

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// Tool types and registry constructors re-exported from tools (D2-S3).
type (
	ToolSchema    = tools.ToolSchema
	ToolCall      = tools.ToolCall
	ToolResult    = tools.ToolResult
	IToolRunner   = tools.IToolRunner
	IToolRegistry = tools.IToolRegistry
	PluginRunner  = tools.PluginRunner
	ToolRegistry  = tools.ToolRegistry
	ToolLimiter   = tools.ToolLimiter

	// AgentRoleToolFilter 从 enforce 包 re-export。
	AgentRoleToolFilter = enforce.AgentRoleToolFilter
)

var (
	NewToolRegistry                  = tools.NewToolRegistry
	NewBuiltinToolRegistry           = tools.NewBuiltinToolRegistry
	NewBuiltinToolRunner             = tools.NewBuiltinToolRunner
	NewBuiltinToolRunnerFromConfig   = tools.NewBuiltinToolRunnerFromConfig
	NewLimitedToolRunner             = tools.NewLimitedToolRunner
	NewTodoWriteRunner               = tools.NewTodoWriteRunner
	NewToolLimiter                   = tools.NewToolLimiter
	WithToolWorkDir                  = tools.WithToolWorkDir
	WithToolSessionID                = tools.WithToolSessionID
	WithToolSessionContext           = tools.WithToolSessionContext
	WithFilesAutoApproved            = tools.WithFilesAutoApproved
	DefaultCommandPolicy             = tools.DefaultCommandPolicy
	NewCommandPolicy                 = tools.NewCommandPolicy
)

// ToolStreamEvent is a mid-execution event from an agent tool (e.g. Claude Code).
type ToolStreamEvent struct {
	Type     string
	Content  string
	ToolName string
}

// ToolStreamEmitter forwards agent tool stream events to the gateway during execution.
type ToolStreamEmitter func(ToolStreamEvent)

type toolStreamEmitterKey struct{}

// WithToolStreamEmitter attaches a stream callback for agent tool mid-execution events.
func WithToolStreamEmitter(ctx context.Context, emit ToolStreamEmitter) context.Context {
	if emit == nil {
		return ctx
	}
	return context.WithValue(ctx, toolStreamEmitterKey{}, emit)
}

// ToolStreamEmitterFromContext returns the stream emitter, if any.
func ToolStreamEmitterFromContext(ctx context.Context) ToolStreamEmitter {
	if v, ok := ctx.Value(toolStreamEmitterKey{}).(ToolStreamEmitter); ok {
		return v
	}
	return nil
}

// Bridge helpers for contextengine callers still in the same domain package.
func ToolSessionContextFromContext(ctx context.Context) *types.SessionContext {
	return tools.ToolSessionContextFromContext(ctx)
}

func ToolWorkDirFromContext(ctx context.Context) string {
	return tools.ToolWorkDirFromContext(ctx)
}

func ToolSessionIDFromContext(ctx context.Context) string {
	return tools.ToolSessionIDFromContext(ctx)
}

func ToolContext(ctx context.Context, sc *types.SessionContext) context.Context {
	return ToolContextWithGate(ctx, sc, nil)
}

// ToolContextWithGate attaches session context and YOLO file policy from the permission gate.
func ToolContextWithGate(ctx context.Context, sc *types.SessionContext, gate contracts.IPermissionGate) context.Context {
	if sc == nil {
		return ctx
	}
	ctx = tools.WithToolWorkDir(ctx, sc.WorkDir)
	ctx = tools.WithToolSessionID(ctx, sc.SessionID)
	ctx = tools.WithToolSessionContext(ctx, sc)
	if gate != nil {
		if fa, ok := gate.(contracts.FileAutoApprover); ok {
			ctx = tools.WithFilesAutoApproved(ctx, fa.AutoApproveFiles())
		}
	}
	return ctx
}

// RegisterBackgroundTaskTools registers task_stop / task_output as LLM tools.
func RegisterBackgroundTaskTools(reg *ToolRegistry) error {
	return enforce.RegisterBackgroundTaskTools(reg)
}

// RegisterPlanModeTools registers enter/exit plan mode tools on the engine registry.
func RegisterPlanModeTools(reg *ToolRegistry, cfg *ContextEngineConfig) error {
	return enforce.RegisterPlanModeTools(reg, cfg)
}

// withToolStreamEmitter bridges ToolStreamEmitter events to EngineEvent emit.
func withToolStreamEmitter(
	ctx context.Context,
	emit func(*contracts.EngineEvent),
	sessionID, toolName string,
) context.Context {
	if emit == nil {
		return ctx
	}
	return WithToolStreamEmitter(ctx, func(ev ToolStreamEvent) {
		switch ev.Type {
		case "thinking":
			emit(&contracts.EngineEvent{
				Type: "thinking", Content: ev.Content, SessionID: sessionID,
				Metadata: map[string]string{"source": "agent_tool", "tool_name": toolName, "agent": ev.ToolName},
			})
		case "text":
			emit(&contracts.EngineEvent{
				Type: "text", Content: ev.Content, SessionID: sessionID,
				Metadata: map[string]string{"is_complete": "false", "source": "agent_tool", "tool_name": toolName, "agent": ev.ToolName},
			})
		case "tool_use":
			emit(&contracts.EngineEvent{
				Type: "info", Content: ev.Content, SessionID: sessionID,
				Metadata: map[string]string{"source": "agent_tool", "tool_name": toolName, "agent": ev.ToolName},
			})
		}
	})
}
