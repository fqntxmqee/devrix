// Adapter unit tests for D2-S15 prepare scenario.
//
// These tests verify the adapter package's behavior in isolation from the
// facade. They use nil-safe paths (no observability hooks) and minimal mocks
// for the underlying Manager/Pipeline/Assembler.
//
// T: D2-S15-A01-T* (SessionLoader)
// T: D2-S15-A02-T* (MemoryRecaller)
// T: D2-S15-A03-T* (ContextCompressor)
// T: D2-S15-A04-T* (PromptAssembler)
package adapters

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/compression"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// --- Hooks tests ---

func TestHooks_NilDefaults(t *testing.T) {
	h := applyHooks(nil)
	if h.StartSpan != nil {
		t.Errorf("expected nil SpanStarter, got %v", h.StartSpan)
	}
	if h.Emit != nil {
		t.Errorf("expected nil EventEmitter, got %v", h.Emit)
	}
	ctx, span := h.startSpan(context.Background(), "test.op", 0)
	if span != nil {
		t.Errorf("expected nil span from nil hook, got %v", span)
	}
	if ctx == nil {
		t.Error("expected non-nil ctx from nil hook")
	}
	h.emit(nil) // no panic
}

func TestHooks_WithOptions(t *testing.T) {
	called := false
	h := applyHooks([]HooksOption{
		WithSpanStarter(func(ctx context.Context, op string, kind tracer.SpanKind, attrs ...tracer.Attribute) (context.Context, tracer.Span) {
			called = true
			_ = op
			_ = kind
			_ = attrs
			return ctx, nil
		}),
		WithEventEmitter(func(*contracts.EngineEvent) {}),
	})
	if h.StartSpan == nil || h.Emit == nil {
		t.Fatalf("expected hooks to be set, got StartSpan=%v Emit=%v", h.StartSpan, h.Emit)
	}
	ctx, span := h.startSpan(context.Background(), "test.op", tracer.SpanKindInternal)
	if span != nil {
		t.Errorf("expected nil span from custom no-op starter, got %v", span)
	}
	if ctx == nil {
		t.Error("expected non-nil ctx")
	}
	if !called {
		t.Error("expected custom SpanStarter to be called")
	}
}

// --- SessionLoader adapter tests ---

func TestSessionLoaderAdapter_Construction(t *testing.T) {
	// Use nil manager — adapter only invokes manager methods inside LoadOrInit;
	// constructor itself must accept nil for test convenience.
	a := NewSessionLoaderAdapter(nil)
	if a == nil {
		t.Fatal("NewSessionLoaderAdapter returned nil")
	}
	if a.manager != nil {
		t.Errorf("expected nil manager, got %v", a.manager)
	}
}

func TestSessionLoaderAdapter_LoadOrInit_NilManager_PanicsOrErrors(t *testing.T) {
	a := NewSessionLoaderAdapter(nil)
	// With nil manager, LoadOrInit should panic (defensive); recover to assert.
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when manager is nil")
		}
	}()
	_, _ = a.LoadOrInit(&types.Session{SessionID: "s1"}, "")
}

// --- MemoryRecaller adapter tests ---

func TestMemoryRecallerAdapter_WorkerLocalSkipsRecall(t *testing.T) {
	a := NewMemoryRecallerAdapter(nil).WithWorkerLocalChecker(func() bool { return true })
	entries, err := a.RecallLongTermEntries(context.Background(), "test query")
	if err != nil {
		t.Errorf("expected nil err on worker-local skip, got %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil entries on worker-local skip, got %v", entries)
	}
}

func TestMemoryRecallerAdapter_NilManager_PanicsOnActualRecall(t *testing.T) {
	a := NewMemoryRecallerAdapter(nil) // workerLocal nil ⇒ not skipped
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when manager is nil and not worker-local")
		}
	}()
	_, _ = a.RecallLongTermEntries(context.Background(), "test query")
}

func TestMemoryRecallerAdapter_JoinStringsHelper(t *testing.T) {
	cases := []struct {
		in   []string
		sep  string
		want string
	}{
		{nil, ",", ""},
		{[]string{}, ",", ""},
		{[]string{"a"}, ",", "a"},
		{[]string{"a", "b"}, ",", "a,b"},
		{[]string{"a", "b", "c"}, "|", "a|b|c"},
	}
	for _, c := range cases {
		got := joinStrings(c.in, c.sep)
		if got != c.want {
			t.Errorf("joinStrings(%v, %q) = %q, want %q", c.in, c.sep, got, c.want)
		}
	}
}

// --- Compressor adapter tests ---

func TestCompressorAdapter_SkipWhenCompressPerTurnFalse(t *testing.T) {
	called := false
	factory := func(_ string) *compression.Pipeline {
		called = true
		return nil
	}
	a := NewCompressorAdapter(factory).WithCompressPerTurnSkip(func() bool { return false })
	out, report, err := a.Run(context.Background(), []types.Message{{Role: types.MessageRoleUser}}, "", types.TokenBudget{})
	if err != nil {
		t.Errorf("expected nil err, got %v", err)
	}
	if report.OriginalTokens != 0 {
		t.Errorf("expected zero report, got %+v", report)
	}
	if len(out) != 1 || out[0].Role != types.MessageRoleUser {
		t.Errorf("expected passthrough of 1 user msg, got %+v", out)
	}
	if called {
		t.Error("factory should not be called when CompressPerTurn=false")
	}
}

func TestCompressorAdapter_ShouldCompress_HonorsSkipFlag(t *testing.T) {
	a := NewCompressorAdapter(func(_ string) *compression.Pipeline { return nil }).
		WithCompressPerTurnSkip(func() bool { return false })
	if a.ShouldCompress(nil, types.TokenBudget{}) {
		t.Error("ShouldCompress must return false when CompressPerTurn=false")
	}
}

// --- Assembler adapter tests ---

func TestAssemblerAdapter_Construction(t *testing.T) {
	a := NewAssemblerAdapter(nil)
	if a == nil {
		t.Fatal("NewAssemblerAdapter returned nil")
	}
}

func TestAssemblerAdapter_Build_NilAssembler_Panics(t *testing.T) {
	a := NewAssemblerAdapter(nil)
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when assembler is nil")
		}
	}()
	_, _ = a.Build(prompt.SystemPromptBuildInput{})
}

// --- helpers ---

var _ = memory.MemoryEntry{} // keep import (used in interfaces, not directly here)