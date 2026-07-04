package tracer

import (
	"context"
	"sync"
	"testing"
)

type recordingExporter struct {
	mu      sync.Mutex
	exports int
}

func (e *recordingExporter) Export(_ context.Context, _ ReadableSpan) error {
	e.mu.Lock()
	e.exports++
	e.mu.Unlock()
	return nil
}

func (e *recordingExporter) Shutdown(_ context.Context) error { return nil }

func (e *recordingExporter) count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.exports
}

// T: D5-S1-A01-T01
func TestTracerProvider_should_flush_pending_spans_on_shutdown(t *testing.T) {
	exp := &recordingExporter{}
	tp := NewTracerProvider(nil, exp)
	tr := tp.Tracer("test")
	_, span := tr.Start(context.Background(), "pending-span")
	if len(tr.activeSpans) != 1 {
		t.Fatalf("expected 1 active span, got %d", len(tr.activeSpans))
	}
	_ = span
	if err := tp.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if got := exp.count(); got != 1 {
		t.Fatalf("expected 1 exported span, got %d", got)
	}
	if len(tr.activeSpans) != 0 {
		t.Fatalf("expected active spans cleared, got %d", len(tr.activeSpans))
	}
}

func TestTraceIDGeneration(t *testing.T) {
	tid1 := GenerateTraceID()
	tid2 := GenerateTraceID()
	if tid1 == tid2 {
		t.Error("trace IDs should be unique")
	}
}

func TestSpanIDGeneration(t *testing.T) {
	sid1 := GenerateSpanID()
	sid2 := GenerateSpanID()
	if sid1 == sid2 {
		t.Error("span IDs should be unique")
	}
}

// DM-20260704-003 / D5-S2-A01-T01+T02: parent-span continuity. The fix to
// tracingStepObserver.OnStep (compression/tracing_step_observer.go:28)
// restored ctx propagation, which means a child Start() with a parent
// SpanContext in ctx must (a) inherit the trace ID and (b) cause the
// returned ctx to carry the new span context so grandchildren can continue
// the chain. This is the unit-level guarantee that closes the Jaeger
// "200 spans missing parent" regression. (The Span interface does not
// expose Parent(), so we verify parent continuity indirectly via the
// returned ctx's SpanContext matching the child's.)
func TestTracer_Start_InheritsParentFromContext(t *testing.T) {
	exp := &recordingExporter{}
	tp := NewTracerProvider(nil, exp)
	tr := tp.Tracer("test")

	// Root span
	rootCtx, rootSpan := tr.Start(context.Background(), "root")
	rootSC := rootSpan.SpanContext()
	if !rootSC.IsValid() {
		t.Fatalf("root span context invalid: %+v", rootSC)
	}

	// Child span must inherit trace ID from root (proves the parent was
	// picked up from ctx, not freshly generated).
	childCtx, childSpan := tr.Start(rootCtx, "child")
	childSC := childSpan.SpanContext()
	if childSC.TraceID != rootSC.TraceID {
		t.Errorf("child TraceID = %s, want %s (root) — parent not inherited from ctx", childSC.TraceID, rootSC.TraceID)
	}
	if childSC.SpanID == rootSC.SpanID {
		t.Errorf("child SpanID = root SpanID; must differ")
	}

	// The returned ctx must carry the child SpanContext so callers (e.g.
	// tracingStepObserver.OnStep, after the fix) can pass it to grandchildren.
	grandSC := SpanContextFromContext(childCtx)
	if grandSC == nil {
		t.Fatalf("returned childCtx has no SpanContext; future grandchildren will be orphan")
	}
	if grandSC.SpanID != childSC.SpanID {
		t.Errorf("ctx SpanContext SpanID = %s, want %s (child)", grandSC.SpanID, childSC.SpanID)
	}

	childSpan.End()
	rootSpan.End()
}

// DM-20260704-003 / D5-S2-A01-T02 case 2 (scheduler dispatch style): a
// 3-level chain root → mid → leaf must all share the same trace ID and each
// returned ctx must carry the corresponding span context.
func TestTracer_Start_ThreeLevelChain(t *testing.T) {
	exp := &recordingExporter{}
	tp := NewTracerProvider(nil, exp)
	tr := tp.Tracer("test")

	ctxA, spanA := tr.Start(context.Background(), "A")
	scA := spanA.SpanContext()

	ctxB, spanB := tr.Start(ctxA, "B")
	scB := spanB.SpanContext()

	ctxC, spanC := tr.Start(ctxB, "C")
	scC := spanC.SpanContext()

	if scA.TraceID != scB.TraceID || scB.TraceID != scC.TraceID {
		t.Errorf("trace IDs diverged in 3-level chain: A=%s B=%s C=%s", scA.TraceID, scB.TraceID, scC.TraceID)
	}

	// Each ctx returned by Start must carry the corresponding span
	// context; otherwise grandchildren of B or C become orphan roots.
	scBinCtx := SpanContextFromContext(ctxB)
	if scBinCtx == nil || scBinCtx.SpanID != scB.SpanID {
		t.Errorf("ctxB SpanContext mismatch: got %+v, want SpanID=%s (B)", scBinCtx, scB.SpanID)
	}
	scCinCtx := SpanContextFromContext(ctxC)
	if scCinCtx == nil || scCinCtx.SpanID != scC.SpanID {
		t.Errorf("ctxC SpanContext mismatch: got %+v, want SpanID=%s (C)", scCinCtx, scC.SpanID)
	}

	spanC.End()
	spanB.End()
	spanA.End()
}
