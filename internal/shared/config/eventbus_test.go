package config

import (
	"testing"
	"time"
)

// L5 helper: config defaults invariant tests.

func TestEventBusConfig_Defaults(t *testing.T) {
	c := DefaultEventBusConfig()
	if err := c.Validate(); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}
	if c.LowWatermark >= c.HighWatermark {
		t.Fatalf("low watermark %d must be < high watermark %d",
			c.LowWatermark, c.HighWatermark)
	}
	if c.HighWatermark != 24 {
		t.Fatalf("HighWatermark default = %d, want 24", c.HighWatermark)
	}
	if c.DrainTimeout != 2*time.Second {
		t.Fatalf("DrainTimeout default = %s, want 2s", c.DrainTimeout)
	}
}

func TestEventBusConfig_EnvOverride(t *testing.T) {
	t.Setenv("DEVRIX_EVENTBUS_HIGH_WATERMARK", "100")
	t.Setenv("DEVRIX_EVENTBUS_LOW_WATERMARK", "20")
	t.Setenv("DEVRIX_EVENTBUS_DRAIN_TIMEOUT", "5s")

	c := LoadEventBusConfig()
	if c.HighWatermark != 100 {
		t.Fatalf("HighWatermark = %d, want 100", c.HighWatermark)
	}
	if c.LowWatermark != 20 {
		t.Fatalf("LowWatermark = %d, want 20", c.LowWatermark)
	}
	if c.DrainTimeout != 5*time.Second {
		t.Fatalf("DrainTimeout = %s, want 5s", c.DrainTimeout)
	}
}

func TestEventBusConfig_EnvOverride_ClampsHysteresis(t *testing.T) {
	t.Setenv("DEVRIX_EVENTBUS_HIGH_WATERMARK", "50")
	// Intentionally invalid: Low >= High.
	t.Setenv("DEVRIX_EVENTBUS_LOW_WATERMARK", "100")

	c := LoadEventBusConfig()
	if c.LowWatermark >= c.HighWatermark {
		t.Fatalf("LowWatermark (%d) should be clamped below HighWatermark (%d)",
			c.LowWatermark, c.HighWatermark)
	}
}

func TestEventBusConfig_Validate_RejectsInvalid(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(c *EventBusConfig)
		wantErr bool
	}{
		{"zero high", func(c *EventBusConfig) { c.HighWatermark = 0 }, true},
		{"zero low", func(c *EventBusConfig) { c.LowWatermark = 0 }, true},
		{"low >= high", func(c *EventBusConfig) { c.LowWatermark = c.HighWatermark }, true},
		{"zero buffer", func(c *EventBusConfig) { c.ChannelBuffer = 0 }, true},
		{"zero drain timeout", func(c *EventBusConfig) { c.DrainTimeout = 0 }, true},
		{"valid", func(c *EventBusConfig) {}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := DefaultEventBusConfig()
			tc.mutate(&c)
			err := c.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}
