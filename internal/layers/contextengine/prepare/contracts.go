// Package prepare — D2-S1 PrepareContext: 执行前上下文准备。
//
// S1 编排 4 个 A 层:
//   - A01 LoadSession: 快照加载、Worker fork、模型解析
//   - A02 RecallMemory: 长期记忆召回、记忆上下文格式化
//   - A03 CompressContext: Token 预算检查、压缩管道、Autocompact
//   - A04 AssemblePrompt: System prompt 组装（agents + memory + workspace + attachment + Hub-Spoke）
package prepare

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/types"
)

// SpanStarter is the hook used by orchestrator and adapters to emit OTel spans.
// Defined here (in the contracts package) to avoid an import cycle: adapters
// and orchestrator both depend on this type; the concrete implementation lives
// in facade/engine_compression.go::startSpan.
type SpanStarter func(ctx context.Context, operation string, kind tracer.SpanKind, attrs ...tracer.Attribute) (context.Context, tracer.Span)

// SessionLoader loads or initializes a session context from snapshot.
//
// Returns (sc, isNew, error):
//   - isNew = true when the session was just initialized (no prior snapshot);
//   - isNew = false when an existing snapshot was restored.
//
// DSAFT: D2-S1-A01 (LoadSession)
type SessionLoader interface {
	LoadOrInit(ctx context.Context, session *types.Session, model string) (sc *types.SessionContext, isNew bool, err error)
}

// MemoryRecaller retrieves long-term memory entries relevant to a query.
//
// DSAFT: D2-S1-A02 (RecallMemory)
type MemoryRecaller interface {
	RecallLongTermEntries(ctx context.Context, query string) ([]memory.MemoryEntry, error)
}

// ContextCompressor compresses messages when they exceed the token budget.
//
// DSAFT: D2-S1-A03 (CompressContext)
type ContextCompressor interface {
	ShouldCompress(msgs []types.Message, budget types.TokenBudget) bool
	Run(ctx context.Context, msgs []types.Message, systemPrompt string, budget types.TokenBudget) ([]types.Message, types.CompressionReport, error)
}

// PromptAssembler builds the system prompt from all context sources.
//
// DSAFT: D2-S1-A04 (AssemblePrompt)
type PromptAssembler interface {
	Build(ctx context.Context, input prompt.SystemPromptBuildInput) (string, prompt.SystemPromptBuildReport)
}