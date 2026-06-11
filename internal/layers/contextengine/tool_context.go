package contextengine

import (
	"context"
	"os"
	"path/filepath"

	"github.com/devrix/devrix/internal/shared/types"
)

type toolWorkDirKey struct{}
type toolSessionIDKey struct{}
type toolSessionContextKey struct{}
type toolStreamEmitterKey struct{}
type toolFilesAutoApprovedKey struct{}

// ToolStreamEvent is a mid-execution event from an agent tool (e.g. Claude Code).
type ToolStreamEvent struct {
	Type     string // thinking | text | tool_use
	Content  string
	ToolName string // parent tool name, e.g. call_claude-code
}

// ToolStreamEmitter forwards agent tool stream events to the gateway during execution.
type ToolStreamEmitter func(ToolStreamEvent)

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

// WithToolWorkDir attaches the session workspace directory to ctx for tool execution.
func WithToolWorkDir(ctx context.Context, workDir string) context.Context {
	if workDir == "" {
		return ctx
	}
	return context.WithValue(ctx, toolWorkDirKey{}, filepath.Clean(workDir))
}

// ToolWorkDirFromContext returns the workspace directory for tool execution.
func ToolWorkDirFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(toolWorkDirKey{}).(string); ok {
		return v
	}
	return ""
}

// WithToolSessionID attaches the session ID to ctx for tool execution.
func WithToolSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, toolSessionIDKey{}, sessionID)
}

// ToolSessionIDFromContext returns the session ID for tool execution.
func ToolSessionIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(toolSessionIDKey{}).(string); ok {
		return v
	}
	return ""
}

// WithToolSessionContext attaches the live session context for permission-aware tools.
func WithToolSessionContext(ctx context.Context, sc *types.SessionContext) context.Context {
	if sc == nil {
		return ctx
	}
	return context.WithValue(ctx, toolSessionContextKey{}, sc)
}

// ToolSessionContextFromContext returns the session context when attached.
func ToolSessionContextFromContext(ctx context.Context) *types.SessionContext {
	if v, ok := ctx.Value(toolSessionContextKey{}).(*types.SessionContext); ok {
		return v
	}
	return nil
}

// WithFilesAutoApproved marks whether YOLO allows workspace writes under plan mode.
func WithFilesAutoApproved(ctx context.Context, approved bool) context.Context {
	if !approved {
		return ctx
	}
	return context.WithValue(ctx, toolFilesAutoApprovedKey{}, true)
}

// FilesAutoApprovedFromContext reports YOLO workspace write bypass for plan mode.
func FilesAutoApprovedFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(toolFilesAutoApprovedKey{}).(bool)
	return v
}

// ToolContext wraps standard tool execution context values.
func ToolContext(ctx context.Context, sc *types.SessionContext) context.Context {
	return ToolContextWithGate(ctx, sc, nil)
}

// ToolContextWithGate attaches session context and YOLO file policy from the permission gate.
func ToolContextWithGate(ctx context.Context, sc *types.SessionContext, gate IPermissionGate) context.Context {
	if sc == nil {
		return ctx
	}
	ctx = WithToolWorkDir(ctx, sc.WorkDir)
	ctx = WithToolSessionID(ctx, sc.SessionID)
	ctx = WithToolSessionContext(ctx, sc)
	if gate != nil {
		if fa, ok := gate.(FileAutoApprover); ok {
			ctx = WithFilesAutoApproved(ctx, fa.AutoApproveFiles())
		}
	}
	return ctx
}

// ResolveToolWorkDir returns a cleaned workspace path, falling back to os.Getwd().
func ResolveToolWorkDir(ctx context.Context) (string, error) {
	if wd := ToolWorkDirFromContext(ctx); wd != "" {
		return wd, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Clean(cwd), nil
}
