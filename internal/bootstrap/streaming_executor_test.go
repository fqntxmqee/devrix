// T: D7-S9-A50-T27 — StreamingToolExecutor 单测 (DM-20260702-009 PR-F).
//
// 9 个单测覆盖 lifecycle / discard 语义 / 并发安全:
//   1. TestStreamingExecutor_BufferAndRead      — 基础 buffer 累积
//   2. TestStreamingExecutor_DiscardSynthesizes — discard 产出 streaming_fallback results
//   3. TestStreamingExecutor_Discard_Idempotent — 第二次 discard 返 nil
//   4. TestStreamingExecutor_DiscardAfterBuffer_BufferNoop — discard 后 buffer 失效
//   5. TestStreamingExecutor_DiscardEmpty_NoResults — 空 buffer 返 nil
//   6. TestStreamingExecutor_Discard_NilReceiver — nil receiver 返 nil, 不 panic
//   7. TestStreamingExecutor_DiscardPreservesIDs — synthetic result 顺序与 buffered 一致
//   8. TestStreamingExecutor_Concurrent_Buffer   — 并发 buffer 无 race / panic
//   9. TestStreamingExecutor_BufferEmptyID      — 跳过空 ID (defensive)
package bootstrap

import (
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
)

func TestStreamingExecutor_BufferAndRead(t *testing.T) {
	e := NewStreamingToolExecutor()
	defer e.Discard("never") // cleanup

	e.Buffer(llmgateway.ToolCall{ID: "c1", Name: "bash", Input: `{"command":"ls"}`})
	e.Buffer(llmgateway.ToolCall{ID: "c2", Name: "read_file", Input: `{"path":"a.go"}`})

	if got := e.BufferedCount(); got != 2 {
		t.Errorf("BufferedCount=%d, want 2", got)
	}
	buffered := e.Buffered()
	if len(buffered) != 2 {
		t.Fatalf("Buffered() returned %d items, want 2", len(buffered))
	}
	if buffered[0].ID != "c1" || buffered[1].ID != "c2" {
		t.Errorf("Buffered order broken: got [%s, %s], want [c1, c2]",
			buffered[0].ID, buffered[1].ID)
	}
}

func TestStreamingExecutor_DiscardSynthesizes(t *testing.T) {
	e := NewStreamingToolExecutor()
	e.Buffer(llmgateway.ToolCall{ID: "c1", Name: "bash", Input: `{"command":"ls"}`})
	e.Buffer(llmgateway.ToolCall{ID: "c2", Name: "read_file", Input: `{"path":"a.go"}`})

	results := e.Discard("primary_503")
	if len(results) != 2 {
		t.Fatalf("Discard returned %d results, want 2", len(results))
	}

	// 每个 result 应是 streaming_fallback synthetic.
	for i, r := range results {
		if r.Error == "" {
			t.Errorf("results[%d].Error is empty; want streaming_fallback sentinel", i)
		}
		// 检查 sentinel 前缀.
		if r.Error != FormatStreamingFallbackError("primary_503") {
			t.Errorf("results[%d].Error=%q, want %q", i, r.Error,
				FormatStreamingFallbackError("primary_503"))
		}
	}

	// 状态: discarded, reason 记录, buffer 清空.
	if !e.IsDiscarded() {
		t.Error("IsDiscarded() should be true after Discard")
	}
	if got := e.DiscardReason(); got != "primary_503" {
		t.Errorf("DiscardReason()=%q, want %q", got, "primary_503")
	}
	if got := e.BufferedCount(); got != 0 {
		t.Errorf("BufferedCount after Discard=%d, want 0", got)
	}
}

func TestStreamingExecutor_Discard_Idempotent(t *testing.T) {
	e := NewStreamingToolExecutor()
	e.Buffer(llmgateway.ToolCall{ID: "c1", Name: "bash", Input: `{}`})

	first := e.Discard("first")
	if len(first) != 1 {
		t.Fatalf("first Discard returned %d, want 1", len(first))
	}

	// 第二次 discard: 已 terminal, 返 nil.
	second := e.Discard("second")
	if second != nil {
		t.Errorf("second Discard returned %v, want nil (idempotent)", second)
	}
	// reason 保留第一次的, 不被覆盖.
	if got := e.DiscardReason(); got != "first" {
		t.Errorf("DiscardReason=%q, want %q (first call's reason preserved)", got, "first")
	}
}

func TestStreamingExecutor_DiscardAfterBuffer_BufferNoop(t *testing.T) {
	e := NewStreamingToolExecutor()
	e.Buffer(llmgateway.ToolCall{ID: "c1", Name: "bash", Input: `{}`})
	e.Discard("fallback")

	// discard 后再 buffer 应 no-op (terminal state).
	e.Buffer(llmgateway.ToolCall{ID: "c2", Name: "read_file", Input: `{}`})

	if got := e.BufferedCount(); got != 0 {
		t.Errorf("BufferedCount after post-discard Buffer=%d, want 0", got)
	}
	if buf := e.Buffered(); len(buf) != 0 {
		t.Errorf("Buffered() after post-discard Buffer returned %d items, want 0", len(buf))
	}
}

func TestStreamingExecutor_DiscardEmpty_NoResults(t *testing.T) {
	e := NewStreamingToolExecutor()
	results := e.Discard("no_buffer")
	if results != nil {
		t.Errorf("Discard on empty buffer returned %v, want nil", results)
	}
	if !e.IsDiscarded() {
		t.Error("IsDiscarded() should be true even with empty buffer")
	}
	if got := e.DiscardReason(); got != "no_buffer" {
		t.Errorf("DiscardReason()=%q, want %q", got, "no_buffer")
	}
}

func TestStreamingExecutor_Discard_NilReceiver(t *testing.T) {
	var e *StreamingToolExecutor // nil
	// 所有方法都该 no-op, 不 panic.
	e.Buffer(llmgateway.ToolCall{ID: "c1", Name: "bash", Input: `{}`})
	if got := e.BufferedCount(); got != 0 {
		t.Errorf("nil.BufferedCount()=%d, want 0", got)
	}
	if buf := e.Buffered(); buf != nil {
		t.Errorf("nil.Buffered() returned %v, want nil", buf)
	}
	if e.IsDiscarded() {
		t.Error("nil.IsDiscarded()=true, want false")
	}
	if got := e.DiscardReason(); got != "" {
		t.Errorf("nil.DiscardReason()=%q, want empty", got)
	}
	results := e.Discard("nil_test")
	if results != nil {
		t.Errorf("nil.Discard() returned %v, want nil", results)
	}
}

func TestStreamingExecutor_DiscardPreservesIDs(t *testing.T) {
	e := NewStreamingToolExecutor()
	ids := []string{"tc_01", "tc_02", "tc_03", "tc_04", "tc_05"}
	for _, id := range ids {
		e.Buffer(llmgateway.ToolCall{ID: id, Name: "bash", Input: `{}`})
	}
	results := e.Discard("preserve_test")
	if len(results) != len(ids) {
		t.Fatalf("Discard returned %d, want %d", len(results), len(ids))
	}
	for i, r := range results {
		if r.ToolCallID != ids[i] {
			t.Errorf("results[%d].ToolCallID=%q, want %q (order broken)", i, r.ToolCallID, ids[i])
		}
	}
}

func TestStreamingExecutor_Concurrent_Buffer(t *testing.T) {
	e := NewStreamingToolExecutor()
	defer e.Discard("concurrent_test")

	var wg sync.WaitGroup
	const workers = 50
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := "tc_" + string(rune('A'+i%26)) + string(rune('0'+i%10))
			e.Buffer(llmgateway.ToolCall{ID: id, Name: "bash", Input: `{}`})
		}(i)
	}
	wg.Wait()

	// 50 个 unique IDs (不是 50 个, 因为 ID 生成可能冲突; 用 count 验证就行)
	if got := e.BufferedCount(); got != workers {
		t.Errorf("BufferedCount after concurrent Buffer=%d, want %d", got, workers)
	}
}

func TestStreamingExecutor_BufferEmptyID(t *testing.T) {
	e := NewStreamingToolExecutor()
	defer e.Discard("cleanup")

	e.Buffer(llmgateway.ToolCall{ID: "", Name: "bash", Input: `{}`}) // 应该 skip
	e.Buffer(llmgateway.ToolCall{ID: "valid", Name: "read_file", Input: `{}`})

	if got := e.BufferedCount(); got != 1 {
		t.Errorf("BufferedCount=%d, want 1 (empty ID should be skipped)", got)
	}
}

// FormatStreamingFallbackError 自身要单测: 验证 sentinel + reason 组合
// 在不同 reason 下的格式. 这是 LLM 层 transcript dump 时依赖的契约.
func TestFormatStreamingFallbackError(t *testing.T) {
	cases := []struct {
		reason string
		want   string
	}{
		{"", ErrStreamingFallbackSentinel},
		{"primary_503", "streaming_fallback: primary_503"},
		{"primary_timeout", "streaming_fallback: primary_timeout"},
		{"primary_rate_limit", "streaming_fallback: primary_rate_limit"},
	}
	for _, c := range cases {
		got := FormatStreamingFallbackError(c.reason)
		if got != c.want {
			t.Errorf("FormatStreamingFallbackError(%q)=%q, want %q", c.reason, got, c.want)
		}
	}
}

// sentinel: keep sessionorchestrator import live in case tests evolve.
var _ = sessionorchestrator.ToolResult{}