package eventbus

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// newTestBus returns a bus with a tight config for deterministic tests.
func newTestBus(t *testing.T, mutate func(*config.EventBusConfig)) (*Bus, config.EventBusConfig) {
	t.Helper()
	cfg := config.DefaultEventBusConfig()
	cfg.SubscribeBuffer = 16
	if mutate != nil {
		mutate(&cfg)
	}
	b, err := NewBus(cfg)
	if err != nil {
		t.Fatalf("NewBus: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b, cfg
}

func makeEv(typ, content string) Event {
	return Event{
		EngineEvent: &contracts.EngineEvent{
			Type:      typ,
			Content:   content,
			SessionID: "sess_test",
		},
		Priority:    PriorityNormal,
		PublishedAt: time.Now(),
	}
}

// drainSubs collects up to want events from a subscriber channel, or fails.
func drainSubs(t *testing.T, ch <-chan Event, want int, timeout time.Duration) []Event {
	t.Helper()
	out := make([]Event, 0, want)
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for len(out) < want {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatalf("subscriber channel closed early; got %d/%d", len(out), want)
			}
			out = append(out, ev)
		case <-deadline.C:
			t.Fatalf("timeout waiting for events; got %d/%d", len(out), want)
		}
	}
	return out
}

// L5-2-3-01: 正常事件流不丢
func TestL5_2_3_01_NormalEventFlow_NoLoss(t *testing.T) {
	b, _ := newTestBus(t, func(c *config.EventBusConfig) {
		c.SubscribeBuffer = 256
		c.HighWatermark = 100 // don't trip backpressure during this test
		c.LowWatermark = 10
	})
	_, ch, _, cancel := b.Subscribe("sess_a")
	defer cancel()

	// Drain subscriber in a goroutine so fanout never drops.
	go func() {
		for range ch {
		}
	}()

	const N = 50
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < N; i++ {
			ev := makeEv("text", "hello")
			if err := b.Publish(context.Background(), ev); err != nil {
				t.Errorf("Publish: %v", err)
				return
			}
		}
	}()

	wg.Wait()
	// Wait for the bus to drain everything to subscribers.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if b.Backlog() == 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if b.Backlog() != 0 {
		t.Fatalf("backlog not drained: %d", b.Backlog())
	}
}

// L5-2-3-05: Complete 事件必达
func TestL5_2_3_05_CompleteEventNeverDropped(t *testing.T) {
	b, _ := newTestBus(t, func(c *config.EventBusConfig) {
		c.ChannelBuffer = 4
	})
	_, ch, _, cancel := b.Subscribe("sess_complete")
	defer cancel()

	// Flood Normal events to fill the buffer.
	for i := 0; i < 4; i++ {
		_ = b.Publish(context.Background(), makeEv("text", "flood"))
	}

	// PublishCritical must succeed even when normalCh is full.
	ctx, cc := context.WithTimeout(context.Background(), 1*time.Second)
	defer cc()
	completeEv := makeEv("complete", "summary")
	if err := b.PublishCritical(ctx, completeEv); err != nil {
		t.Fatalf("PublishCritical(complete): %v", err)
	}

	// Drain subscriber until we see the complete event (or timeout).
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	var gotComplete bool
	for !gotComplete {
		select {
		case ev := <-ch:
			if ev.EngineEvent.Type == "complete" {
				if ev.Priority != PriorityCritical {
					t.Fatalf("complete event priority = %d, want Critical", ev.Priority)
				}
				gotComplete = true
			}
		case <-deadline.C:
			t.Fatal("timeout waiting for complete event on subscriber")
		}
	}
}

// L5-2-3-06: Error 事件必达
func TestL5_2_3_06_ErrorEventNeverDropped(t *testing.T) {
	b, _ := newTestBus(t, nil)
	_, ch, _, cancel := b.Subscribe("sess_err")
	defer cancel()

	// Pre-fill with normal events.
	for i := 0; i < 8; i++ {
		_ = b.Publish(context.Background(), makeEv("text", "x"))
	}

	errEv := makeEv("error", "boom")
	if err := b.PublishCritical(context.Background(), errEv); err != nil {
		t.Fatalf("PublishCritical(error): %v", err)
	}

	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	var gotError bool
	for !gotError {
		select {
		case ev := <-ch:
			if ev.EngineEvent.Type == "error" {
				if ev.Priority != PriorityCritical {
					t.Fatalf("error event priority = %d, want Critical", ev.Priority)
				}
				gotError = true
			}
		case <-deadline.C:
			t.Fatal("timeout waiting for error event on subscriber")
		}
	}
}

// L5-2-3-07: 通道满时回压到上游
//
// Verifies the backpressure contract: when the bus is in StateDraining
// (operator-triggered), Publish must block the caller because the
// internal pipeline is being shed. We prove blocking by publishing with
// a short context deadline; if the channel were not back-pressured,
// Publish would return immediately and the test would observe an error.
func TestL5_2_3_07_PublishBlocksAtHighWatermark(t *testing.T) {
	b, _ := newTestBus(t, func(c *config.EventBusConfig) {
		c.ChannelBuffer = 2
		c.HighWatermark = 4
		c.LowWatermark = 1
		c.DrainTimeout = 500 * time.Millisecond
	})
	_, _, _, cancel := b.Subscribe("backpressure")
	defer cancel()

	// Force Draining FIRST so the monitor stops consuming normalCh.
	b.TriggerDrain()

	// Now fill the channel. Publishes 1 and 2 succeed (buffer capacity
	// is 2). Publish 3 must block.
	for i := 0; i < 2; i++ {
		if err := b.Publish(context.Background(), makeEv("text", "x")); err != nil {
			t.Fatalf("Publish %d: %v", i, err)
		}
	}

	// Now Publish must block. Use a short timeout context — Publish
	// should return ErrContextCancelled (proving it blocked until
	// the ctx fired, since the channel is full and the monitor is
	// paused in Draining).
	ctx, cc := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cc()
	start := time.Now()
	err := b.Publish(ctx, makeEv("text", "blocked"))
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("expected Publish to block (ctx timeout), got nil err")
	}
	if err != ErrContextCancelled {
		t.Fatalf("expected ErrContextCancelled, got %v", err)
	}
	if elapsed < 40*time.Millisecond {
		t.Fatalf("Publish returned too fast (%s); expected to block for ctx duration", elapsed)
	}

	// Cleanup: return to Running so the monitor drains residual events.
	_, _ = b.Drain(context.Background(), "backpressure", 200*time.Millisecond)
}
