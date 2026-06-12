package eventbus

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/config"
)

// L5-2-3-04: Reconnect 恢复
func TestL5_2_3_04_ReconnectRecovery(t *testing.T) {
	b, _ := newTestBus(t, func(c *config.EventBusConfig) {
		c.ChannelBuffer = 32
		c.HighWatermark = 8
		c.LowWatermark = 2
		c.DrainTimeout = 500 * time.Millisecond
		c.CompactMaxBatch = 16
		c.SubscribeBuffer = 32
	})
	_, ch, _, cancel := b.Subscribe("reconnect")
	defer cancel()

	// Drain subscriber in a separate goroutine.
	var seenCount atomic.Int64
	go func() {
		for ev := range ch {
			seenCount.Add(1)
			_ = ev
		}
	}()

	// Fill backlog.
	for i := 0; i < 16; i++ {
		_ = b.Publish(context.Background(), makeEv("text", "x"))
	}

	// Wait for monitor to detect backpressure.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if b.State() == StateDraining {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	rep, err := b.Reconnect(context.Background(), "reconnect")
	if err != nil {
		t.Fatalf("Reconnect: %v", err)
	}
	if b.State() != StateRunning {
		t.Fatalf("state after Reconnect = %s, want running", b.State())
	}
	if rep.DrainReport.Drained == 0 && rep.CompactReport.Compacted == 0 {
		t.Logf("Reconnect report: drained=%d, compacted=%d, flushed=%d",
			rep.DrainReport.Drained, rep.CompactReport.Compacted, rep.PendingFlushed)
	}

	// After Reconnect, the bus should accept new events.
	if err := b.Publish(context.Background(), makeEv("text", "after")); err != nil {
		t.Fatalf("Publish after Reconnect: %v", err)
	}
	if b.Backlog() < 0 {
		t.Fatalf("backlog negative: %d", b.Backlog())
	}
}

// Critical event survives the full Drain→Compact→Reconnect cycle.
func TestReconnectPreservesCritical(t *testing.T) {
	b, _ := newTestBus(t, func(c *config.EventBusConfig) {
		c.ChannelBuffer = 8
		c.HighWatermark = 4
		c.LowWatermark = 1
		c.DrainTimeout = 300 * time.Millisecond
	})
	_, ch, _, cancel := b.Subscribe("reconnect-crit")
	defer cancel()

	saw := make(chan string, 1)
	go func() {
		for ev := range ch {
			if ev.EngineEvent.Type == "complete" {
				saw <- ev.EngineEvent.Type
			}
		}
	}()

	// Flood with Normal events.
	for i := 0; i < 8; i++ {
		_ = b.Publish(context.Background(), makeEv("text", "x"))
	}

	// Critical event must land.
	if err := b.PublishCritical(context.Background(), makeEv("complete", "ok")); err != nil {
		t.Fatalf("PublishCritical: %v", err)
	}

	// Run Reconnect.
	_, err := b.Reconnect(context.Background(), "reconnect-crit")
	if err != nil {
		t.Fatalf("Reconnect: %v", err)
	}

	select {
	case typ := <-saw:
		if typ != "complete" {
			t.Fatalf("got event type %q, want complete", typ)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: complete event did not reach subscriber through Reconnect cycle")
	}
}
