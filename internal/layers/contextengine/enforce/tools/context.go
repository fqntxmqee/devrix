package tools

import (
	"context"
	"os"
	"path/filepath"

	"github.com/devrix/devrix/internal/shared/types"
)

type toolWorkDirKey struct{}
type toolSessionIDKey struct{}
type toolSessionContextKey struct{}
type toolFilesAutoApprovedKey struct{}

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
