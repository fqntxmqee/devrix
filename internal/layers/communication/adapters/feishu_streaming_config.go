package adapters

import "time"

func (c FeishuStreamingConfig) throttleConfig() streamThrottleConfig {
	cfg := defaultStreamThrottleConfig()
	cfg.Enabled = c.Enabled
	if c.IntervalMs > 0 {
		cfg.Interval = time.Duration(c.IntervalMs) * time.Millisecond
	}
	if c.MinDeltaChars > 0 {
		cfg.MinDeltaRunes = c.MinDeltaChars
	}
	return cfg
}
