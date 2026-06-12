package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// EventBusConfig holds backpressure event-bus configuration.
//
// DM-20260611-003 (devrix-event-channel): controls the Drain/Compact/Reconnect
// state machine thresholds for the BackpressureEventBus.
type EventBusConfig struct {
	// HighWatermark is the backlog threshold that triggers a Draining transition.
	HighWatermark int
	// LowWatermark is the hysteresis exit threshold (Draining → Compacting).
	LowWatermark int
	// DrainTimeout caps how long a Drain call can block waiting for backlog to drop.
	DrainTimeout time.Duration
	// CompactMaxBatch is the maximum number of events compacted into a single
	// aggregated event in one pass.
	CompactMaxBatch int
	// ReconnectTimeout caps the channel rebuild step.
	ReconnectTimeout time.Duration
	// ChannelBuffer is the initial normal-event channel capacity.
	ChannelBuffer int
	// SubscribeBuffer is the per-subscriber buffered channel size.
	SubscribeBuffer int
}

// DefaultEventBusConfig returns safe production defaults.
func DefaultEventBusConfig() EventBusConfig {
	return EventBusConfig{
		HighWatermark:    24,
		LowWatermark:     8,
		DrainTimeout:     2 * time.Second,
		CompactMaxBatch:  16,
		ReconnectTimeout: 1 * time.Second,
		ChannelBuffer:    32,
		SubscribeBuffer:  64,
	}
}

// LoadEventBusConfig returns a config with env-var overrides applied.
// Recognized env vars:
//
//	DEVRIX_EVENTBUS_HIGH_WATERMARK  (int)
//	DEVRIX_EVENTBUS_LOW_WATERMARK   (int)
//	DEVRIX_EVENTBUS_DRAIN_TIMEOUT   (Go duration, e.g. "3s")
//	DEVRIX_EVENTBUS_CHANNEL_BUFFER  (int)
func LoadEventBusConfig() EventBusConfig {
	cfg := DefaultEventBusConfig()
	if v := os.Getenv("DEVRIX_EVENTBUS_HIGH_WATERMARK"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.HighWatermark = n
		}
	}
	if v := os.Getenv("DEVRIX_EVENTBUS_LOW_WATERMARK"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.LowWatermark = n
		}
	}
	if v := os.Getenv("DEVRIX_EVENTBUS_DRAIN_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.DrainTimeout = d
		}
	}
	if v := os.Getenv("DEVRIX_EVENTBUS_CHANNEL_BUFFER"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.ChannelBuffer = n
		}
	}
	if cfg.LowWatermark >= cfg.HighWatermark {
		// Hysteresis invariant — keep Low strictly below High.
		cfg.LowWatermark = cfg.HighWatermark / 3
		if cfg.LowWatermark < 1 {
			cfg.LowWatermark = 1
		}
	}
	return cfg
}

// Validate ensures the config is internally consistent.
func (c EventBusConfig) Validate() error {
	if c.HighWatermark <= 0 {
		return fmt.Errorf("eventbus: HighWatermark must be > 0, got %d", c.HighWatermark)
	}
	if c.LowWatermark <= 0 {
		return fmt.Errorf("eventbus: LowWatermark must be > 0, got %d", c.LowWatermark)
	}
	if c.LowWatermark >= c.HighWatermark {
		return fmt.Errorf("eventbus: LowWatermark (%d) must be < HighWatermark (%d)",
			c.LowWatermark, c.HighWatermark)
	}
	if c.ChannelBuffer <= 0 {
		return fmt.Errorf("eventbus: ChannelBuffer must be > 0, got %d", c.ChannelBuffer)
	}
	if c.DrainTimeout <= 0 {
		return fmt.Errorf("eventbus: DrainTimeout must be > 0, got %s", c.DrainTimeout)
	}
	return nil
}
