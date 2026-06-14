package adapters

import (
	"time"
)

type streamThrottleConfig struct {
	Enabled       bool
	Interval      time.Duration
	MinDeltaRunes int
}

func defaultStreamThrottleConfig() streamThrottleConfig {
	return streamThrottleConfig{
		Enabled:       true,
		Interval:      400 * time.Millisecond,
		MinDeltaRunes: 24,
	}
}

func (c streamThrottleConfig) shouldFlush(lastAt time.Time, lastRunes, newRunes int, force bool) bool {
	if force {
		return true
	}
	if !c.Enabled {
		return true
	}
	if lastAt.IsZero() {
		return true
	}
	delta := newRunes - lastRunes
	if delta <= 0 {
		return false
	}
	elapsed := time.Since(lastAt)
	if delta >= c.MinDeltaRunes && elapsed >= c.Interval {
		return true
	}
	return elapsed >= c.Interval*2
}
