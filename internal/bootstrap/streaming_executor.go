// Package bootstrap — T27 StreamingToolExecutor (DM-20260702-009 PR-F).
//
// 模型: LLM 流式响应中, 工具调用 (tool_use) 是一边流一边累积的. 正常路径
// 是 LLM 流完后批量执行. 但当 LLM 主模型 fallback 到副模型时, 在途 /
// queued 的 tool_use 不能直接丢弃 — 否则 fallback 模型的下一轮会看到
// 悬空的 tool_use 而没有对应的 tool_result, 触发 "orphan tool_use"
// 错误 (clawcode streaming-executor.ts 中描述的 subtle bug).
//
// 解决: 在 fallback 触发前, 把 buffer 里的 tool_use 全部合成为
// "streaming_fallback" synthetic tool_result, 让 fallback 模型的下一轮
// 看到的是干净的 tool_result 列表 (新 iteration 使用 fresh executor).
//
// 这是 clawcode 的 `discardOnFallback` 镜像: tool_use 流 → buffer →
// fallback fire → synthetic result → fresh executor. 工程上对应:
//   - StreamingToolExecutor.Buffer(call)   (累积 tool_use)
//   - StreamingToolExecutor.Discard(reason) (合成 streaming_fallback result)
//   - 新 iteration 用 NewStreamingToolExecutor(...) 拿 fresh instance
//
// 注意: 这个 executor 是 **per LLM iteration** 的. 一旦 discard, 实例
// 失效; 新 iteration 必须 NewStreamingToolExecutor. 这是为了避免
// "上一轮 discard 后的 buffer 残留到本轮" 的状态污染.
//
// DSAFT: D7-S9-A50-T27 (DM-20260702-009 PR-F).
package bootstrap

import (
	"context"
	"fmt"
	"sync"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
)

// ErrStreamingFallbackSentinel is the canonical error string for the
// synthetic "streaming_fallback" tool_result. Callers that want to
// detect this state (e.g. for metrics, observability, or downstream
// branching) should match against this constant, not the message body.
//
// Format: "streaming_fallback: <reason>". The reason is a short
// human-readable token (e.g. "primary_503", "primary_timeout") from
// the LLM retry layer. It's preserved in the error so post-mortem
// inspection (transcript dumps, OTEL spans) can identify which
// fallback path fired.
const ErrStreamingFallbackSentinel = "streaming_fallback"

// StreamingToolExecutor buffers tool_use blocks as they stream from
// the LLM, and exposes Discard() to synthesize "streaming_fallback"
// results for any buffered-but-not-yet-executed calls. Per LLM
// iteration instance — one executor lives for one round-trip.
//
// Lifecycle:
//
//	exec := NewStreamingToolExecutor()
//	for chunk := range llmStream {
//	    if chunk.HasToolUse() {
//	        exec.Buffer(chunk.ToolCall)
//	    }
//	}
//	// Either:
//	//   1. 正常 — 调用方自己 iterate exec.Buffered() 调 ToolRoundExecutor
//	//   2. fallback — 调 Discard(reason) 拿到 synthetic results, 喂给副模型
//	// 然后开 NewStreamingToolExecutor() 给下一轮.
type StreamingToolExecutor struct {
	mu             sync.Mutex
	buffered       []llmgateway.ToolCall
	discarded      bool
	discardReason  string
}

// NewStreamingToolExecutor returns a fresh executor. Always call this
// per LLM iteration; do NOT reuse across iterations.
func NewStreamingToolExecutor() *StreamingToolExecutor {
	return &StreamingToolExecutor{}
}

// Buffer records a streamed tool_use block. Thread-safe; safe to call
// from the LLM stream consumer goroutine.
//
// Calling Buffer after Discard is a no-op (the executor is in
// terminal state). This avoids a subtle race where a late-arriving
// tool_use gets recorded into a discarded executor.
func (e *StreamingToolExecutor) Buffer(call llmgateway.ToolCall) {
	if e == nil || call.ID == "" {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.discarded {
		return
	}
	e.buffered = append(e.buffered, call)
}

// Buffered returns a snapshot of the currently buffered tool_use
// blocks (read-only copy). The caller can iterate and dispatch them
// via ToolRoundExecutor.ExecuteRound. Useful for the normal path
// (no fallback).
//
// Nil receiver: returns nil. The nil-tolerant design lets callers
// avoid initialization checks for the no-fallback-yet case.
func (e *StreamingToolExecutor) Buffered() []llmgateway.ToolCall {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.discarded {
		return nil
	}
	out := make([]llmgateway.ToolCall, len(e.buffered))
	copy(out, e.buffered)
	return out
}

// BufferedCount returns len(Buffered()) without copying. Cheap.
//
// Nil receiver: returns 0 (not panic).
func (e *StreamingToolExecutor) BufferedCount() int {
	if e == nil {
		return 0
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.buffered)
}

// IsDiscarded reports whether Discard() was called.
//
// Nil receiver: returns false (zero value).
func (e *StreamingToolExecutor) IsDiscarded() bool {
	if e == nil {
		return false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.discarded
}

// DiscardReason returns the reason passed to Discard. Returns "" if
// Discard was not called.
//
// Nil receiver: returns "".
func (e *StreamingToolExecutor) DiscardReason() string {
	if e == nil {
		return ""
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.discardReason
}

// Discard synthesizes "streaming_fallback" results for ALL buffered
// tool_use blocks. Returns the synthetic results; the caller is
// expected to feed them into the next LLM iteration's tool_result
// blocks (so the fallback model doesn't see orphan tool_use).
//
// Returns nil when the buffer is empty (no synthetic results needed).
// The state transition to `discarded` still happens — the executor is
// in terminal state either way, and the caller can detect this via
// IsDiscarded() / DiscardReason().
//
// Idempotent — second call returns nil (executor already in terminal
// state). This is essential for race safety: if both the LLM retry
// layer and the engine loop call Discard (e.g. on different signals),
// only the first one wins.
//
// After Discard, the executor is in terminal state. Buffer() becomes
// a no-op. A NEW executor must be constructed for the next iteration.
func (e *StreamingToolExecutor) Discard(reason string) []sessionorchestrator.ToolResult {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.discarded {
		return nil
	}
	e.discarded = true
	e.discardReason = reason
	if len(e.buffered) == 0 {
		// 没有 in-flight 工具, 但仍标记 discarded. 返 nil
		// 让 caller 用 len==0 当 "无 synthetic result" 判据.
		return nil
	}
	out := make([]sessionorchestrator.ToolResult, len(e.buffered))
	for i, call := range e.buffered {
		out[i] = sessionorchestrator.ToolResult{
			ToolCallID: call.ID,
			Error:      FormatStreamingFallbackError(reason),
		}
	}
	// 清空 buffer, 让 GC 回收. 不需要保留 — discard 是 terminal.
	e.buffered = nil
	return out
}

// FormatStreamingFallbackError formats a fallback reason into the
// canonical "streaming_fallback: <reason>" error string used as the
// synthetic tool_result.Error. Exposed for tests + callers that need
// to construct the same string outside Discard (rare).
//
// Empty reason → just "streaming_fallback" (no colon). This matches
// the sentinel-matching convention in tests (errors.Is on the
// sentinel prefix).
func FormatStreamingFallbackError(reason string) string {
	if reason == "" {
		return ErrStreamingFallbackSentinel
	}
	return fmt.Sprintf("%s: %s", ErrStreamingFallbackSentinel, reason)
}

// sentinel context import kept (avoid linter stripping it during refactors).
var _ = context.Background