package coordinator

import (
	"log/slog"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/observability/metrics"
)

// outcome is the discrete outcome of a validation. It
// drives both the counter increment and the timeout-rate calculation.
type outcome string

const (
	outcomePass    outcome = "pass"
	outcomeFail    outcome = "fail"
	outcomeTimeout outcome = "timeout"
	outcomeError   outcome = "error"
)

const (
	// rateWindow is the sliding window for timeout-rate computation.
	// R2 §5 P1 #6 specifies 5min continuous.
	rateWindow = 5 * time.Minute

	// rateAlertThreshold is the (timeout+error)/total ratio that
	// triggers a WARN log + AlertHook.
	rateAlertThreshold = 0.05

	// minSamples is the minimum window size before the alert fires.
	// Cold-start guard: with 1-2 samples a single timeout would trip
	// 50%+ rate and produce false alarms.
	minSamples = 20
)

// AlertHook is the v1.0 in-process alert sink. The default
// implementation logs a WARN; tests inject a recorder.
type AlertHook func(rate float64, samples uint64)

// ValidationMetrics owns the four validation counters
// and the sliding-window timeout-rate computation.
//
// Constructed via NewValidationMetrics. A nil receiver is treated
// as no-op (defensive — the orchestrator does not need to nil-check
// before calling Record*).
//
// v1.0 P1 (per R2 §5 P1 #6). v1.1+ AlertManager integration is left
// to a follow-up change.
type ValidationMetrics struct {
	pass    metrics.Counter
	fail    metrics.Counter
	timeout metrics.Counter
	error   metrics.Counter

	mu     sync.Mutex
	window []sample // ring buffer (chronological, pruned on insert)
	rate   float64

	onAlert AlertHook
}

// sample is one validation outcome, used for sliding-window rate.
type sample struct {
	at      time.Time
	outcome outcome
}

// MetricsConfig is the input to NewValidationMetrics. The Meter is
// required (counters must be registered); onAlert is optional (nil →
// default WARN log).
type MetricsConfig struct {
	Meter   *metrics.Meter
	OnAlert AlertHook
}

// NewValidationMetrics builds the 4-counter instrument and primes
// the sliding window. If cfg.Meter is nil the function returns nil
// (caller treats as no-op).
func NewValidationMetrics(cfg MetricsConfig) *ValidationMetrics {
	if cfg.Meter == nil {
		return nil
	}
	pass, err := cfg.Meter.Int64Counter("orchestration.d6.validation.pass")
	if err != nil {
		slog.Error("orchestrator: failed to register pass counter", "err", err)
		return nil
	}
	fail, err := cfg.Meter.Int64Counter("orchestration.d6.validation.fail")
	if err != nil {
		slog.Error("orchestrator: failed to register fail counter", "err", err)
		return nil
	}
	timeout, err := cfg.Meter.Int64Counter("orchestration.d6.validation.timeout")
	if err != nil {
		slog.Error("orchestrator: failed to register timeout counter", "err", err)
		return nil
	}
	erc, err := cfg.Meter.Int64Counter("orchestration.d6.validation.error")
	if err != nil {
		slog.Error("orchestrator: failed to register error counter", "err", err)
		return nil
	}
	hook := cfg.OnAlert
	if hook == nil {
		hook = defaultAlertHook
	}
	return &ValidationMetrics{
		pass:    pass,
		fail:    fail,
		timeout: timeout,
		error:   erc,
		onAlert: hook,
	}
}

// RecordPass increments the pass counter and updates the sliding window.
func (m *ValidationMetrics) RecordPass(at time.Time) {
	if m == nil {
		return
	}
	m.pass.Inc()
	m.recordOutcome(outcomePass, at)
}

// RecordFail increments the fail counter and updates the sliding window.
func (m *ValidationMetrics) RecordFail(at time.Time) {
	if m == nil {
		return
	}
	m.fail.Inc()
	m.recordOutcome(outcomeFail, at)
}

// RecordTimeout increments the timeout counter and updates the sliding window.
func (m *ValidationMetrics) RecordTimeout(at time.Time) {
	if m == nil {
		return
	}
	m.timeout.Inc()
	m.recordOutcome(outcomeTimeout, at)
}

// RecordError increments the error counter and updates the sliding window.
func (m *ValidationMetrics) RecordError(at time.Time) {
	if m == nil {
		return
	}
	m.error.Inc()
	m.recordOutcome(outcomeError, at)
}

// recordOutcome is the single insertion point that maintains the
// sliding window, recomputes the rate, and fires the alert hook.
func (m *ValidationMetrics) recordOutcome(outcome outcome, at time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := at.Add(-rateWindow)
	pruned := m.window[:0]
	for _, s := range m.window {
		if s.at.After(cutoff) {
			pruned = append(pruned, s)
		}
	}
	m.window = append(pruned, sample{at: at, outcome: outcome})

	m.rate = m.computeRateLocked()
	total := uint64(len(m.window))
	if m.rate > rateAlertThreshold && total >= minSamples && m.onAlert != nil {
		// Fire the hook under the lock to keep rate atomic w.r.t. further
		// inserts. The hook must be non-blocking; the default WARN-log
		// implementation is.
		m.onAlert(m.rate, total)
	}
}

// computeRateLocked returns (timeout+error) / total within the current
// window. Caller must hold m.mu.
func (m *ValidationMetrics) computeRateLocked() float64 {
	var total, bad uint64
	for _, s := range m.window {
		total++
		if s.outcome == outcomeTimeout || s.outcome == outcomeError {
			bad++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(bad) / float64(total)
}

// TimeoutRate returns the most recently computed timeout_rate. Safe
// for concurrent reads.
func (m *ValidationMetrics) TimeoutRate() float64 {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rate
}

// WindowSize returns the current number of samples in the sliding
// window. Useful for tests + cold-start debugging.
func (m *ValidationMetrics) WindowSize() int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.window)
}

// Counters returns the four counters for inspection (tests, exporters).
// Nil-safe.
func (m *ValidationMetrics) Counters() (pass, fail, timeout, err metrics.Counter) {
	if m == nil {
		return nil, nil, nil, nil
	}
	return m.pass, m.fail, m.timeout, m.error
}

// defaultAlertHook is the v1.0 default: log a structured WARN.
func defaultAlertHook(rate float64, samples uint64) {
	slog.Warn("orchestrator: D6 validation timeout_rate alert",
		"rate", rate,
		"samples", samples,
		"threshold", rateAlertThreshold,
		"window", rateWindow,
	)
}
