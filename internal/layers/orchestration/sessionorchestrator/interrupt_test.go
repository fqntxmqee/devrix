package sessionorchestrator

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// stubEventSink captures published events for assertions.
type stubEventSink struct {
	events atomic.Int64
	last   atomic.Pointer[contracts.EngineEvent]
}

func (s *stubEventSink) Publish(_ context.Context, ev *contracts.EngineEvent) {
	s.events.Add(1)
	s.last.Store(ev)
}

// TestHandleInterrupt_AllStepsFail_JoinsErrors verifies that when all 3
// cancel steps fail, Handle returns errors.Join containing all 3 wrapped
// errors and increments the per-step failure counters + handle_errored.
//
// DM-20260621-010 PR-A: replaces the previous "all warn + nil" anti-pattern
// with errors.Join aggregation.
func TestHandleInterrupt_AllStepsFail_JoinsErrors(t *testing.T) {
	waveErr := errors.New("wave cancel failed")
	d4Err := errors.New("d4 cancel failed")
	procErr := errors.New("process cancel failed")

	sink := &stubEventSink{}
	metrics := &InterruptMetrics{}
	h := NewInterruptHandler(nil, InterruptOptions{
		WaveCanceler:     func(string) error { return waveErr },
		DelegateCanceler: func(string) error { return d4Err },
		ProcessCanceler:  func(string) error { return procErr },
		Sink:             sink,
		Metrics:          metrics,
	})

	err := h.Handle(context.Background(), "sess-A")
	if err == nil {
		t.Fatal("expected non-nil error from Handle when all cancel steps fail")
	}

	// errors.Is / errors.As: each wrapped error should be reachable
	if !errors.Is(err, waveErr) {
		t.Errorf("errors.Join should contain waveErr, got %v", err)
	}
	if !errors.Is(err, d4Err) {
		t.Errorf("errors.Join should contain d4Err, got %v", err)
	}
	if !errors.Is(err, procErr) {
		t.Errorf("errors.Join should contain procErr, got %v", err)
	}

	// Metrics: all 3 step counters + handle_errored
	snap := metrics.Snapshot()
	if snap.WaveCancelFailed != 1 {
		t.Errorf("WaveCancelFailed = %d, want 1", snap.WaveCancelFailed)
	}
	if snap.D4CancelFailed != 1 {
		t.Errorf("D4CancelFailed = %d, want 1", snap.D4CancelFailed)
	}
	if snap.ProcessCancelFailed != 1 {
		t.Errorf("ProcessCancelFailed = %d, want 1", snap.ProcessCancelFailed)
	}
	if snap.HandleErrored != 1 {
		t.Errorf("HandleErrored = %d, want 1", snap.HandleErrored)
	}
	if snap.HandleCompleted != 0 {
		t.Errorf("HandleCompleted = %d, want 0", snap.HandleCompleted)
	}

	// "stopped" event still emitted best-effort
	if got := sink.events.Load(); got != 1 {
		t.Errorf("sink should receive 1 'stopped' event, got %d", got)
	}
	last := sink.last.Load()
	if last == nil || last.Type != "stopped" {
		t.Errorf("expected 'stopped' event, got %+v", last)
	}
}

// TestHandleInterrupt_PartialFailure_ReturnsPartialErr verifies that when
// some steps fail and others succeed, the returned error contains only the
// failed steps' wrapped errors.
func TestHandleInterrupt_PartialFailure_ReturnsPartialErr(t *testing.T) {
	waveErr := errors.New("wave cancel failed")
	sink := &stubEventSink{}
	metrics := &InterruptMetrics{}

	h := NewInterruptHandler(nil, InterruptOptions{
		WaveCanceler:     func(string) error { return waveErr },
		DelegateCanceler: func(string) error { return nil },
		ProcessCanceler:  func(string) error { return nil },
		Sink:             sink,
		Metrics:          metrics,
	})

	err := h.Handle(context.Background(), "sess-A")
	if err == nil {
		t.Fatal("expected non-nil error from Handle when 1 step fails")
	}
	if !errors.Is(err, waveErr) {
		t.Errorf("errors.Join should contain waveErr, got %v", err)
	}
	// Should NOT contain 2 other (nil) errors

	snap := metrics.Snapshot()
	if snap.WaveCancelFailed != 1 {
		t.Errorf("WaveCancelFailed = %d, want 1", snap.WaveCancelFailed)
	}
	if snap.D4CancelFailed != 0 {
		t.Errorf("D4CancelFailed = %d, want 0", snap.D4CancelFailed)
	}
	if snap.ProcessCancelFailed != 0 {
		t.Errorf("ProcessCancelFailed = %d, want 0", snap.ProcessCancelFailed)
	}
	if snap.HandleErrored != 1 {
		t.Errorf("HandleErrored = %d, want 1", snap.HandleErrored)
	}
}

// TestHandleInterrupt_AllSuccess_ReturnsNil verifies that when all 3 steps
// succeed, Handle returns nil and increments handle_completed.
func TestHandleInterrupt_AllSuccess_ReturnsNil(t *testing.T) {
	sink := &stubEventSink{}
	metrics := &InterruptMetrics{}

	h := NewInterruptHandler(nil, InterruptOptions{
		WaveCanceler:     func(string) error { return nil },
		DelegateCanceler: func(string) error { return nil },
		ProcessCanceler:  func(string) error { return nil },
		Sink:             sink,
		Metrics:          metrics,
	})

	err := h.Handle(context.Background(), "sess-A")
	if err != nil {
		t.Fatalf("expected nil error when all steps succeed, got %v", err)
	}

	snap := metrics.Snapshot()
	if snap.HandleCompleted != 1 {
		t.Errorf("HandleCompleted = %d, want 1", snap.HandleCompleted)
	}
	if snap.HandleErrored != 0 {
		t.Errorf("HandleErrored = %d, want 0", snap.HandleErrored)
	}
	if snap.WaveCancelFailed+snap.D4CancelFailed+snap.ProcessCancelFailed != 0 {
		t.Errorf("any cancel failure counter > 0, got %+v", snap)
	}
	if got := sink.events.Load(); got != 1 {
		t.Errorf("sink should receive 1 'stopped' event, got %d", got)
	}
}

// TestHandleInterrupt_NilMetrics verifies that Handle works without
// metrics (backward-compatible with pre-PR-A InterruptOptions{} usage).
func TestHandleInterrupt_NilMetrics(t *testing.T) {
	waveErr := errors.New("wave cancel failed")

	h := NewInterruptHandler(nil, InterruptOptions{
		WaveCanceler: func(string) error { return waveErr },
		// Metrics: nil — backward-compat path
	})

	err := h.Handle(context.Background(), "sess-A")
	if err == nil {
		t.Fatal("expected non-nil error even without metrics")
	}
	if !errors.Is(err, waveErr) {
		t.Errorf("expected waveErr wrapped, got %v", err)
	}
}

// TestHandleInterrupt_NoCancelerWired verifies that nil canceler funcs
// are skipped silently (no error, no metric increment).
func TestHandleInterrupt_NoCancelerWired(t *testing.T) {
	sink := &stubEventSink{}
	metrics := &InterruptMetrics{}
	h := NewInterruptHandler(nil, InterruptOptions{
		Sink:    sink,
		Metrics: metrics,
		// All 3 cancelers nil — typical for FastPath-only session
	})

	err := h.Handle(context.Background(), "sess-A")
	if err != nil {
		t.Fatalf("expected nil error when no cancelers wired, got %v", err)
	}
	if got := sink.events.Load(); got != 1 {
		t.Errorf("expected 1 'stopped' event, got %d", got)
	}
	snap := metrics.Snapshot()
	if snap.HandleCompleted != 1 {
		t.Errorf("HandleCompleted = %d, want 1", snap.HandleCompleted)
	}
}

// TestHandleInterrupt_StoppedEventMetadata verifies the stopped event has
// correct metadata for downstream consumers.
func TestHandleInterrupt_StoppedEventMetadata(t *testing.T) {
	sink := &stubEventSink{}
	h := NewInterruptHandler(nil, InterruptOptions{
		WaveCanceler: func(string) error { return nil },
		Sink:         sink,
	})

	if err := h.Handle(context.Background(), "sess-XYZ"); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	ev := sink.last.Load()
	if ev == nil {
		t.Fatal("expected 'stopped' event")
	}
	if ev.SessionID != "sess-XYZ" {
		t.Errorf("SessionID = %q, want %q", ev.SessionID, "sess-XYZ")
	}
	if ev.Metadata["reason"] != "user_interrupt" {
		t.Errorf("reason = %q, want %q", ev.Metadata["reason"], "user_interrupt")
	}
	// stopped_at should be RFC3339Nano
	if _, err := time.Parse(time.RFC3339Nano, ev.Metadata["stopped_at"]); err != nil {
		t.Errorf("stopped_at = %q is not RFC3339Nano: %v", ev.Metadata["stopped_at"], err)
	}
}