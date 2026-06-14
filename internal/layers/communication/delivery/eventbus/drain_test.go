package eventbus

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/config"
)

// D1-S9-A02-T02: 背压触发 Drain
func TestL5_2_3_02_BackpressureTriggersDrain(t *testing.T) {
	b, cfg := newTestBus(t, func(c *config.EventBusConfig) {
		c.ChannelBuffer = 256
		c.HighWatermark = 32
		c.LowWatermark = 2
		c.DrainTimeout = 2 * time.Second
		c.SubscribeBuffer = 1
	})
	_, _, _, cancel := b.Subscribe("drain")
	defer cancel()

	// Pause the monitor FIRST so it does not race the publisher and
	// drain the channel before we observe backlog. With monitor
	// paused, every Publish accumulates in normalCh.
	b.TriggerDrain()

	// Now publish enough events to leave a meaningful backlog. We
	// publish 50, which is well above LowWatermark=2 but small
	// enough to fit comfortably in ChannelBuffer=256.
	for i := 0; i < 50; i++ {
		if err := b.Publish(context.Background(), makeEv("milestone_progress", "step")); err != nil {
			t.Fatalf("Publish %d: %v", i, err)
		}
	}

	// Call Drain. It owns the channel and sheds Normal events down to
	// LowWatermark.
	report, err := b.Drain(context.Background(), "drain", cfg.DrainTimeout)
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if report.Drained == 0 {
		t.Fatalf("Drain report Drained=0; expected some events shed (state=%s, backlog=%d)",
			b.State(), b.Backlog())
	}
	if report.EndBacklog > cfg.LowWatermark+1 { // tiny tolerance for monitor race
		t.Fatalf("EndBacklog=%d > LowWatermark=%d", report.EndBacklog, cfg.LowWatermark)
	}
}

// Critical events MUST never be drained.
func TestDrainPreservesCritical(t *testing.T) {
	b, cfg := newTestBus(t, func(c *config.EventBusConfig) {
		c.ChannelBuffer = 4
		c.HighWatermark = 4
		c.LowWatermark = 1
		c.DrainTimeout = 500 * time.Millisecond
	})
	_, ch, _, cancel := b.Subscribe("drain-crit")
	defer cancel()

	var seen []string
	var mu sync.Mutex
	go func() {
		for ev := range ch {
			mu.Lock()
			seen = append(seen, ev.EngineEvent.Type)
			mu.Unlock()
		}
	}()

	// Force into Draining.
	b.TriggerDrain()

	// Publish one Critical — must reach subscriber regardless.
	if err := b.PublishCritical(context.Background(), makeEv("complete", "ok")); err != nil {
		t.Fatalf("PublishCritical: %v", err)
	}
	// And one Normal which may be drained.
	_ = b.Publish(context.Background(), makeEv("text", "low"))

	// Wait for the critical event to land.
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		has := false
		for _, s := range seen {
			if s == "complete" {
				has = true
			}
		}
		mu.Unlock()
		if has {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	found := false
	for _, s := range seen {
		if s == "complete" {
			found = true
		}
	}
	if !found {
		t.Fatalf("complete event lost during Drain: seen=%v", seen)
	}
	_ = cfg
}
