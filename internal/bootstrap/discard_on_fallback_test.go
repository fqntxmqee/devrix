// T: D7-S9-A50-T27 — DiscardOnFallback wiring 单测 (DM-20260702-009 PR-F).
//
// 6 个单测覆盖 wiring 行为:
//   1. TestDiscardOnFallback_OnFallback_PropagatesResults
//   2. TestDiscardOnFallback_OnFallback_EmptyBuffer
//   3. TestDiscardOnFallback_OnFallback_TracksCount
//   4. TestDiscardOnFallback_NilExecutor_NoOp
//   5. TestDiscardOnFallback_NilReceiver_NoPanic
//   6. TestDiscardOnFallback_AfterDiscard_NoResults
package bootstrap

import (
	"strings"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
)

func TestDiscardOnFallback_OnFallback_PropagatesResults(t *testing.T) {
	exec := NewStreamingToolExecutor()
	exec.Buffer(llmgateway.ToolCall{ID: "c1", Name: "bash", Input: `{}`})
	exec.Buffer(llmgateway.ToolCall{ID: "c2", Name: "read_file", Input: `{}`})

	d := NewDiscardOnFallback(exec)
	results := d.OnFallback("primary_503")
	if len(results) != 2 {
		t.Fatalf("OnFallback returned %d results, want 2", len(results))
	}
	for i, r := range results {
		if !strings.HasPrefix(r.Error, ErrStreamingFallbackSentinel) {
			t.Errorf("results[%d].Error=%q, want prefix %q", i, r.Error, ErrStreamingFallbackSentinel)
		}
		if !strings.Contains(r.Error, "primary_503") {
			t.Errorf("results[%d].Error=%q, want to contain reason 'primary_503'", i, r.Error)
		}
	}
}

func TestDiscardOnFallback_OnFallback_EmptyBuffer(t *testing.T) {
	exec := NewStreamingToolExecutor()
	d := NewDiscardOnFallback(exec)
	results := d.OnFallback("empty_buffer")
	if results != nil {
		t.Errorf("OnFallback with empty buffer returned %v, want nil", results)
	}
	// 但 executor 状态仍变更为 discarded.
	if !exec.IsDiscarded() {
		t.Error("executor should be marked discarded even with empty buffer")
	}
}

func TestDiscardOnFallback_OnFallback_TracksCount(t *testing.T) {
	exec := NewStreamingToolExecutor()
	d := NewDiscardOnFallback(exec)

	// 多次 OnFallback (idempotent): count 只 +1 (只有第一次返非空).
	d.OnFallback("first")
	d.OnFallback("second") // no-op
	d.OnFallback("third")  // no-op

	if got := d.DiscardedCount(); got != 1 {
		t.Errorf("DiscardedCount=%d, want 1 (only first call counts)", got)
	}
}

func TestDiscardOnFallback_NilExecutor_NoOp(t *testing.T) {
	d := NewDiscardOnFallback(nil) // explicit nil
	results := d.OnFallback("nil_exec")
	if results != nil {
		t.Errorf("OnFallback with nil executor returned %v, want nil", results)
	}
	if got := d.DiscardedCount(); got != 0 {
		t.Errorf("DiscardedCount=%d, want 0 (no discard fired)", got)
	}
}

func TestDiscardOnFallback_NilReceiver_NoPanic(t *testing.T) {
	var d *DiscardOnFallback // nil
	// 所有方法都该 no-op, 不 panic.
	if results := d.OnFallback("nil_recv"); results != nil {
		t.Errorf("nil.OnFallback()=%v, want nil", results)
	}
	if got := d.DiscardedCount(); got != 0 {
		t.Errorf("nil.DiscardedCount()=%d, want 0", got)
	}
	if exec := d.Executor(); exec != nil {
		t.Errorf("nil.Executor()=%v, want nil", exec)
	}
}

func TestDiscardOnFallback_AfterDiscard_NoResults(t *testing.T) {
	exec := NewStreamingToolExecutor()
	exec.Buffer(llmgateway.ToolCall{ID: "c1", Name: "bash", Input: `{}`})

	d := NewDiscardOnFallback(exec)
	first := d.OnFallback("first")
	if len(first) != 1 {
		t.Fatalf("first OnFallback returned %d, want 1", len(first))
	}
	second := d.OnFallback("second")
	if second != nil {
		t.Errorf("second OnFallback returned %v, want nil (executor already discarded)", second)
	}
}

// 并发 fallback: 多个 goroutine 同时调 OnFallback. 只有第一个拿到
// non-nil, 其它都 no-op. 验证 thread-safety.
func TestDiscardOnFallback_ConcurrentOnFallback(t *testing.T) {
	exec := NewStreamingToolExecutor()
	exec.Buffer(llmgateway.ToolCall{ID: "c1", Name: "bash", Input: `{}`})
	exec.Buffer(llmgateway.ToolCall{ID: "c2", Name: "read_file", Input: `{}`})

	d := NewDiscardOnFallback(exec)

	var wg sync.WaitGroup
	const workers = 10
	nonNilCount := 0
	var mu sync.Mutex

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results := d.OnFallback("concurrent")
			if len(results) > 0 {
				mu.Lock()
				nonNilCount++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	// 多个 worker 同时调, 由于 Discard 内部有锁, 只有第一个
	// 能成功拿到非空. 其它都 nil (idempotent). count == 1.
	if nonNilCount != 1 {
		t.Errorf("nonNilCount=%d, want 1 (only first concurrent caller wins)", nonNilCount)
	}
	if got := d.DiscardedCount(); got != 1 {
		t.Errorf("DiscardedCount=%d, want 1", got)
	}
}

// 集成场景: 用 DiscardOnFallback 模拟 "LLM 切到 fallback model" 的整流程.
// 1. 创建 streaming executor, buffer 2 个 tool_use
// 2. DiscardOnFallback.OnFallback 触发
// 3. 验证返的 synthetic results 可以"喂"给 fallback 模型的下一轮
// 4. 创建 fresh executor 给下一轮
func TestDiscardOnFallback_EndToEnd_FallbackFlow(t *testing.T) {
	// Iteration 1: primary LLM 累积 2 个 tool_use.
	exec1 := NewStreamingToolExecutor()
	exec1.Buffer(llmgateway.ToolCall{ID: "iter1_t1", Name: "bash", Input: `{"command":"ls"}`})
	exec1.Buffer(llmgateway.ToolCall{ID: "iter1_t2", Name: "bash", Input: `{"command":"pwd"}`})
	d1 := NewDiscardOnFallback(exec1)

	// Primary model fails → fallback fires.
	synth := d1.OnFallback("primary_503")
	if len(synth) != 2 {
		t.Fatalf("expected 2 synthetic results, got %d", len(synth))
	}

	// 把 synth 喂给下一轮 (实际由 caller 写进 fallback 模型的 tool_result).
	// 这里只验证: synth[0].ToolCallID == "iter1_t1" + synth[1].ToolCallID == "iter1_t2".
	if synth[0].ToolCallID != "iter1_t1" || synth[1].ToolCallID != "iter1_t2" {
		t.Errorf("synthetic result IDs broken: got [%s, %s], want [iter1_t1, iter1_t2]",
			synth[0].ToolCallID, synth[1].ToolCallID)
	}

	// Iteration 2: 用 fresh executor, 不复用 exec1.
	exec2 := NewStreamingToolExecutor()
	if exec2 == exec1 {
		t.Error("iteration 2 must use a fresh executor (not reused)")
	}
	if exec2.IsDiscarded() {
		t.Error("fresh exec2 should not be discarded")
	}
	exec2.Buffer(llmgateway.ToolCall{ID: "iter2_t1", Name: "read_file", Input: `{"path":"a.go"}`})
	if got := exec2.BufferedCount(); got != 1 {
		t.Errorf("fresh exec2 BufferedCount=%d, want 1", got)
	}
}