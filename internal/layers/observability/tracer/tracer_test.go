package tracer

import (
	"context"
	"testing"
	"time"
)

func TestSpanCreateAndEnd(t *testing.T) {
	cfg := &struct {
		Type string
		Rate float64
	}{Type: "always_on", Rate: 1.0}
	
	tp := NewTracerProvider(nil, nil)
	tr := tp.Tracer("test")

	ctx, span := tr.Start(context.Background(), "test.span")

	if span == nil {
		t.Fatal("span should not be nil")
	}
	if !span.IsRecording() {
		t.Error("span should be recording")
	}

	sc := span.SpanContext()
	if !sc.TraceID.IsValid() {
		t.Error("trace ID should be valid")
	}
	if !sc.SpanID.IsValid() {
		t.Error("span ID should be valid")
	}

	span.End()

	if span.IsRecording() {
		t.Error("span should not be recording after End")
	}
}

func TestSpanContextPropagation(t *testing.T) {
	tp := NewTracerProvider(nil, nil)
	tr := tp.Tracer("test")

	ctx, span1 := tr.Start(context.Background(), "parent.span")
	span1.End()

	sc := span1.SpanContext()
	ctx2, span2 := tr.Start(ctx, "child.span")
	span2.End()

	childSc := span2.SpanContext()
	if childSc.TraceID != sc.TraceID {
		t.Error("child span should inherit trace ID")
	}
}

func TestSampler(t *testing.T) {
	sampler := NewSampler(&struct {
		Type string
		Rate float64
	}{Type: "always_on", Rate: 1.0})

	tid := GenerateTraceID()
	if !sampler.ShouldSample(tid) {
		t.Error("always_on sampler should always sample")
	}
}

func TestTraceIDGeneration(t *testing.T) {
	tid1 := GenerateTraceID()
	tid2 := GenerateTraceID()

	if tid1 == tid2 {
		t.Error("trace IDs should be unique")
	}

	if !tid1.IsValid() {
		t.Error("generated trace ID should be valid")
	}
}

func TestSpanIDGeneration(t *testing.T) {
	sid1 := GenerateSpanID()
	sid2 := GenerateSpanID()

	if sid1 == sid2 {
		t.Error("span IDs should be unique")
	}

	if !sid1.IsValid() {
		t.Error("generated span ID should be valid")
	}
}

func TestSpanAttributes(t *testing.T) {
	tp := NewTracerProvider(nil, nil)
	tr := tp.Tracer("test")

	_, span := tr.Start(context.Background(), "test.span",
		WithSpanAttributes(
			Attribute{Key: "key1", Value: "value1"},
			Attribute{Key: "key2", Value: 42},
		),
	)

	span.SetAttributes(Attribute{Key: "key3", Value: "value3"})

	attrs := span.Attributes()
	if attrs["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %v", attrs["key1"])
	}
	if attrs["key3"] != "value3" {
		t.Errorf("expected key3=value3, got %v", attrs["key3"])
	}

	span.End()
}

func TestSpanStatus(t *testing.T) {
	tp := NewTracerProvider(nil, nil)
	tr := tp.Tracer("test")

	_, span := tr.Start(context.Background(), "test.span")
	span.SetStatus(StatusCodeOk, "success")
	span.End()

	if span.Status().Code != StatusCodeOk {
		t.Errorf("expected status Ok, got %v", span.Status().Code)
	}
}

func TestSpanDuration(t *testing.T) {
	tp := NewTracerProvider(nil, nil)
	tr := tp.Tracer("test")

	start := time.Now()
	_, span := tr.Start(context.Background(), "test.span")
	time.Sleep(10 * time.Millisecond)
	span.End()
	duration := span.Duration()

	if duration < 10*time.Millisecond {
		t.Errorf("duration should be >= 10ms, got %v", duration)
	}
	_ = start // silence unused variable warning
}
