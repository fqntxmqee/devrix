package contextengine

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/toolrunner"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// Tool types and registry constructors re-exported from toolrunner (D2-S5).
type (
	ToolSchema    = toolrunner.ToolSchema
	ToolCall      = toolrunner.ToolCall
	ToolResult    = toolrunner.ToolResult
	IToolRunner   = toolrunner.IToolRunner
	IToolRegistry = toolrunner.IToolRegistry
	PluginRunner  = toolrunner.PluginRunner
	ToolRegistry  = toolrunner.ToolRegistry
	ToolLimiter   = toolrunner.ToolLimiter
)

var (
	NewToolRegistry                  = toolrunner.NewToolRegistry
	NewBuiltinToolRegistry           = toolrunner.NewBuiltinToolRegistry
	NewBuiltinToolRunner             = toolrunner.NewBuiltinToolRunner
	NewBuiltinToolRunnerFromConfig   = toolrunner.NewBuiltinToolRunnerFromConfig
	NewLimitedToolRunner             = toolrunner.NewLimitedToolRunner
	NewTodoWriteRunner               = toolrunner.NewTodoWriteRunner
	NewToolLimiter                   = toolrunner.NewToolLimiter
	WithToolWorkDir                  = toolrunner.WithToolWorkDir
	WithToolSessionID                = toolrunner.WithToolSessionID
	WithToolSessionContext           = toolrunner.WithToolSessionContext
	WithFilesAutoApproved            = toolrunner.WithFilesAutoApproved
	DefaultCommandPolicy             = toolrunner.DefaultCommandPolicy
	NewCommandPolicy                 = toolrunner.NewCommandPolicy
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
	return toolrunner.ToolSessionContextFromContext(ctx)
}

func ToolWorkDirFromContext(ctx context.Context) string {
	return toolrunner.ToolWorkDirFromContext(ctx)
}

func ToolSessionIDFromContext(ctx context.Context) string {
	return toolrunner.ToolSessionIDFromContext(ctx)
}

func parseToolInput(input string) map[string]string {
	return toolrunner.ParseToolInput(input)
}

func toolInputString(input string, keys ...string) string {
	return toolrunner.ToolInputString(input, keys...)
}

func ToolContext(ctx context.Context, sc *types.SessionContext) context.Context {
	return ToolContextWithGate(ctx, sc, nil)
}

// ToolContextWithGate attaches session context and YOLO file policy from the permission gate.
func ToolContextWithGate(ctx context.Context, sc *types.SessionContext, gate contracts.IPermissionGate) context.Context {
	if sc == nil {
		return ctx
	}
	ctx = toolrunner.WithToolWorkDir(ctx, sc.WorkDir)
	ctx = toolrunner.WithToolSessionID(ctx, sc.SessionID)
	ctx = toolrunner.WithToolSessionContext(ctx, sc)
	if gate != nil {
		if fa, ok := gate.(contracts.FileAutoApprover); ok {
			ctx = toolrunner.WithFilesAutoApproved(ctx, fa.AutoApproveFiles())
		}
	}
	return ctx
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
