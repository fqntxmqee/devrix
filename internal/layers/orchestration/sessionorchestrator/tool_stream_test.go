package sessionorchestrator

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// T: W13 — ToolEventStream 上下文注入 + 提取。
// 背景: background task (delegate_wave / task_output 等) 在 ExecuteRound 中通过
// WithToolEventStream 注入的回调 emit EngineEvent, Turn loop 转发到 outbound channel。
func TestWithToolEventStream_AttachAndEmit(t *testing.T) {
	got := []*contracts.EngineEvent{}
	emit := func(ev *contracts.EngineEvent) { got = append(got, ev) }
	ctx := WithToolEventStream(context.Background(), emit)
	f := ToolEventStreamFrom(ctx)
	if f == nil {
		t.Fatal("ToolEventStreamFrom returned nil after WithToolEventStream")
	}
	f(&contracts.EngineEvent{Type: "test", SessionID: "s1"})
	if len(got) != 1 {
		t.Errorf("expected 1 emitted event, got %d", len(got))
	}
}

// T: W13 — nil emit 时 WithToolEventStream 返回原 ctx (不 panic)。
func TestWithToolEventStream_NilEmit_NoOp(t *testing.T) {
	ctx := WithToolEventStream(context.Background(), nil)
	f := ToolEventStreamFrom(ctx)
	if f != nil {
		t.Errorf("expected nil emit to short-circuit, got func %p", f)
	}
}

// T: W13 — ToolEventStreamFrom(nil ctx) 返回 nil (防 panic)。
func TestToolEventStreamFrom_NilCtx(t *testing.T) {
	f := ToolEventStreamFrom(nil) //nolint:staticcheck
	if f != nil {
		t.Errorf("expected nil for nil ctx, got func %p", f)
	}
}

// T: W13 — 后调用的 WithToolEventStream 覆盖前一次 (last-wins)。
// 期望: 第二个 callback 接收事件, 第一个不会被调。
func TestWithToolEventStream_LastWins(t *testing.T) {
	first := []*contracts.EngineEvent{}
	second := []*contracts.EngineEvent{}
	ctx := WithToolEventStream(context.Background(), func(ev *contracts.EngineEvent) {
		first = append(first, ev)
	})
	ctx = WithToolEventStream(ctx, func(ev *contracts.EngineEvent) {
		second = append(second, ev)
	})
	f := ToolEventStreamFrom(ctx)
	f(&contracts.EngineEvent{Type: "x"})
	if len(first) != 0 {
		t.Errorf("first callback should not be called (overwritten), got %d", len(first))
	}
	if len(second) != 1 {
		t.Errorf("second callback should receive 1 event, got %d", len(second))
	}
}
