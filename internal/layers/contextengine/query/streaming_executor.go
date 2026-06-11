package query

import (
	"context"
	"sync"

	"github.com/devrix/devrix/internal/shared/types"
	"golang.org/x/sync/errgroup"
)

// IsConcurrencySafeTool reports whether a tool may run in parallel with others in a batch.
func IsConcurrencySafeTool(name string) bool {
	switch name {
	case "read_file", "glob", "grep", "list_dir", "task_get", "task_list":
		return true
	default:
		return false
	}
}

// BatchToolRef identifies one tool invocation in a streaming batch.
type BatchToolRef struct {
	ID    string
	Name  string
	Input string
}

// StreamingToolExecutor runs tool batches with parallel execution for safe tools.
type StreamingToolExecutor struct {
	Tools           ToolExecutor
	Permission      PermissionChecker
	WrapToolContext func(ctx context.Context, sc *types.SessionContext) context.Context
	Emit            EmitFunc
	WrapToolStreamEmitter func(ctx context.Context, emit EmitFunc, sessionID, toolName string) context.Context
}

// ExecuteBatch runs tool calls, parallelizing concurrency-safe tools when possible.
func (e *StreamingToolExecutor) ExecuteBatch(
	ctx context.Context,
	sc *types.SessionContext,
	refs []BatchToolRef,
) []BatchResult {
	if e == nil || e.Tools == nil || len(refs) == 0 {
		return nil
	}
	results := make([]BatchResult, len(refs))
	if !anyConcurrencySafe(refs) || !allConcurrencySafe(refs) {
		for i, ref := range refs {
			results[i] = e.runOne(ctx, sc, ref)
		}
		return results
	}

	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	for i, ref := range refs {
		i, ref := i, ref
		g.Go(func() error {
			res := e.runOne(gctx, sc, ref)
			mu.Lock()
			results[i] = res
			mu.Unlock()
			return res.execErr
		})
	}
	_ = g.Wait()
	return results
}

type BatchResult struct {
	ID      string
	Name    string
	Input   string
	Output  string
	Error   string
	execErr error
}

func (e *StreamingToolExecutor) runOne(ctx context.Context, sc *types.SessionContext, ref BatchToolRef) BatchResult {
	if e.Permission != nil && sc != nil && !e.Permission.Request(ctx, sc.SessionID, ref.Name, ref.Input) {
		return BatchResult{ID: ref.ID, Name: ref.Name, Input: ref.Input, Error: "permission denied"}
	}
	toolCtx := ctx
	if e.WrapToolContext != nil && sc != nil {
		toolCtx = e.WrapToolContext(ctx, sc)
	}
	if e.Emit != nil && e.WrapToolStreamEmitter != nil && sc != nil {
		toolCtx = e.WrapToolStreamEmitter(toolCtx, e.Emit, sc.SessionID, ref.Name)
	}
	out, errMsg, execErr := e.Tools.Execute(toolCtx, ToolCall{ID: ref.ID, Name: ref.Name, Input: ref.Input})
	return BatchResult{ID: ref.ID, Name: ref.Name, Input: ref.Input, Output: out, Error: errMsg, execErr: execErr}
}

func anyConcurrencySafe(refs []BatchToolRef) bool {
	for _, r := range refs {
		if IsConcurrencySafeTool(r.Name) {
			return true
		}
	}
	return false
}

func allConcurrencySafe(refs []BatchToolRef) bool {
	for _, r := range refs {
		if !IsConcurrencySafeTool(r.Name) {
			return false
		}
	}
	return true
}
