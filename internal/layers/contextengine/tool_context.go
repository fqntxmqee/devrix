package contextengine

import (
	"context"
	"os"
	"path/filepath"
)

type toolWorkDirKey struct{}

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
