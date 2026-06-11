package contextengine

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/shared/types"
)

// QueryLoop returns a QueryLoop instance sharing PEVEngine dependencies.
func (e *ContextEngine) QueryLoop() *query.Loop {
	if e == nil || e.pev == nil {
		return nil
	}
	return e.pev.buildQueryLoop()
}

// ToolRegistry returns the engine tool registry.
func (e *ContextEngine) ToolRegistry() IToolRegistry {
	if e == nil {
		return nil
	}
	return e.toolsReg
}

func (e *PEVEngine) buildQueryLoop() *query.Loop {
	loop := &query.Loop{
		LLM:             &llmCaller{llm: e.llm},
		Tools:           &toolExecutor{tools: e.tools, toolsReg: e.toolsReg},
		Permission:      &permChecker{gate: e.permission, reg: e.toolsReg},
		Attachments:     e.queryLoop.Attachments,
		UserContext:     e.queryLoop.UserContext,
		WrapToolContext: func(ctx context.Context, sc *types.SessionContext) context.Context {
			return ToolContextWithGate(ctx, sc, e.permission)
		},
		WrapToolStreamContext: func(ctx context.Context, emit query.EmitFunc, sessionID, toolName string) context.Context {
			return withToolStreamEmitter(ctx, emit, sessionID, toolName)
		},
		SessionQueue:    e.queryLoop.SessionQueue,
		StreamingTools:  e.queryLoop.StreamingTools,
	}
	if e.queryLoop.Compress && e.queryLoop.CompressFn != nil {
		loop.Compress = e.queryLoop.CompressFn("")
	}
	return loop
}
