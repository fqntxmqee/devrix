// Package bootstrap — T27 DiscardOnFallback wiring (DM-20260702-009 PR-F).
//
// Wiring: when the LLM retry layer (llmgateway/protect/retry.go) decides
// to fall back from primary to secondary model mid-stream, this hook
// invokes StreamingToolExecutor.Discard() to synthesize
// "streaming_fallback" results for any buffered tool_use blocks. The
// synthesized results are then injected as completed tool_result
// blocks into the next iteration's prompt — so the fallback model
// doesn't see orphan tool_use.
//
// 设计要点 (per TD-STE-03):
//
//  1. Wiring 触发点: LLM retry layer 在 switch primary → fallback
//     之前调 DiscardOnFallback.OnFallback(reason). reason 由 retry
//     layer 提供 (e.g. "primary_503", "primary_timeout").
//
//  2. 新 iteration: caller 用 NewStreamingToolExecutor() 拿 fresh
//     instance, 不复用 discarded 那个.
//
//  3. 防御: nil receiver / nil executor 都是 no-op (避免 panic 在
//     异常路径).
//
//  4. Metric 友好: 暴露 Counters 接口, 方便在 OnFallback 里 emit 一个
//     "streaming_fallback_discarded" 计数 — 跟其它可观测性指标一致.
//
// 不在此 wiring: 实际的 stream 监听 + tool_use 抽取. 那是 LLM 层
// (llmgateway/stream/) 的事, 跟 D7 编排正交. 本文件只定义协议 + 单元
// 测试 — 上层通过 StreamingToolExecutor.Buffer 推送 tool_use, 通过
// DiscardOnFallback.OnFallback 触发 discard.
//
// DSAFT: D7-S9-A50-T27 (DM-20260702-009 PR-F).
package bootstrap

import (
	"sync/atomic"

	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
)

// DiscardOnFallback is the bootstrap-side wiring that calls
// StreamingToolExecutor.Discard when the LLM retry layer triggers
// a model fallback. Stateless except for the executor reference +
// the discard counter (for observability).
//
// 防御: zero value 是合法 no-op (executor == nil 时 OnFallback 返 nil).
// 这样可以省略初始化检查, 调用方直接构造即可.
type DiscardOnFallback struct {
	// executor 是当前 iteration 的 StreamingToolExecutor. nil 时
	// OnFallback 返 nil (no-op).
	executor *StreamingToolExecutor

	// discardedCount 累计触发 discard 的次数 (跨多次 fallback). 只
	// 用于可观测性 — 实际业务逻辑不需要. atomic.Int64 避免读
	// 写竞争.
	discardedCount atomic.Int64
}

// NewDiscardOnFallback attaches a StreamingToolExecutor for the
// current LLM iteration. Pass nil for executor if there's no in-flight
// iteration (e.g. during shutdown) — OnFallback will be a no-op.
//
// The binding is per-iteration: after Discard fires, the caller MUST
// construct a new executor (and a new DiscardOnFallback) for the
// fallback iteration. Reusing the discarded executor is a programming
// error (it'll return nil from OnFallback).
func NewDiscardOnFallback(executor *StreamingToolExecutor) *DiscardOnFallback {
	return &DiscardOnFallback{executor: executor}
}

// OnFallback is called by the LLM retry layer BEFORE switching to
// the fallback model. Returns the synthetic "streaming_fallback"
// results for any buffered tool_use; the caller is expected to feed
// these into the fallback iteration's tool_result blocks so the
// fallback model doesn't see orphan tool_use.
//
// Idempotent: if the underlying executor is already discarded (or
// nil), returns nil and does NOT increment DiscardedCount. This makes
// OnFallback safe to call multiple times from different signals
// (e.g. both retry layer + engine loop might observe the fallback).
//
// DiscardedCount tracks the number of state transitions (1 if the
// underlying executor transitioned from active → discarded on this
// call, 0 otherwise). An empty buffer that still triggers Discard
// counts as 1 — the transition is what we observe.
//
// Reason is a short human-readable token from the retry layer, e.g.
// "primary_503", "primary_timeout", "primary_rate_limit". It is
// preserved in the synthetic tool_result.Error for post-mortem.
func (d *DiscardOnFallback) OnFallback(reason string) []sessionorchestrator.ToolResult {
	if d == nil || d.executor == nil {
		return nil
	}
	// 检查 executor 状态, 只在 transition 时计数. 避免 Empty buffer
	// + 已 discarded 场景下误计 0.
	if d.executor.IsDiscarded() {
		return nil
	}
	out := d.executor.Discard(reason)
	// state transition happened (executor went active → discarded).
	// 这时 count++ — 无论 buffer 是否空.
	d.discardedCount.Add(1)
	return out
}

// DiscardedCount returns the cumulative number of fallback discards
// observed by this DiscardOnFallback. Useful for observability + tests.
func (d *DiscardOnFallback) DiscardedCount() int64 {
	if d == nil {
		return 0
	}
	return d.discardedCount.Load()
}

// Executor returns the bound StreamingToolExecutor (for inspection
// only). Returns nil if the wiring was constructed without an
// executor. The caller MUST NOT mutate the executor's state directly
// — that would bypass the lock + break Discard's idempotency.
func (d *DiscardOnFallback) Executor() *StreamingToolExecutor {
	if d == nil {
		return nil
	}
	return d.executor
}