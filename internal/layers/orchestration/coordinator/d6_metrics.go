package coordinator

import (
	"log/slog"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/observability/metrics"
)

// d6Outcome is the discrete outcome of a D6 advisory validation. It
// drives both the counter increment and the timeout-rate calculation.
type d6Outcome string

const (
	d6OutcomePass    d6Outcome = "pass"
	d6OutcomeFail    d6Outcome = "fail"
	d6OutcomeTimeout d6Outcome = "timeout"
	d6OutcomeError   d6Outcome = "error"
)

const (
	// d6RateWindow is the sliding window for timeout-rate computation.
	// R2 §5 P1 #6 specifies 5min continuous.
	d6RateWindow = 5 * time.Minute

	// d6RateAlertThreshold is the (timeout+error)/total ratio that
	// triggers a WARN log + AlertHook.
	d6RateAlertThreshold = 0.05

	// d6MinSamples is the minimum window size before the alert fires.
	// Cold-start guard: with 1-2 samples a single timeout would trip
	// 50%+ rate and produce false alarms.
	d6MinSamples = 20
)

// AlertHook is the v1.0 in-process alert sink. The default
// implementation logs a WARN; tests inject a recorder.
type AlertHook func(rate float64, samples uint64)

// D6ValidationMetrics owns the four D6 advisory validation counters
// and the sliding-window timeout-rate computation.
//
// Constructed via NewD6ValidationMetrics. A nil receiver is treated
// as no-op (defensive — the orchestrator does not need to nil-check
// before calling Record*).
//
// v1.0 P1 (per R2 §5 P1 #6). v1.1+ AlertManager integration is left
// to a follow-up change.
type D6ValidationMetrics struct {
	pass    metrics.Counter
	fail    metrics.Counter
	timeout metrics.Counter
	error   metrics.Counter

	mu     sync.Mutex
	window []d6Sample // ring buffer (chronological, pruned on insert)
	rate   float64

	onAlert AlertHook
}

// d6Sample is one validation outcome, used for sliding-window rate.
type d6Sample struct {
	at      time.Time
	outcome d6Outcome
}

// MetricsConfig is the input to NewD6ValidationMetrics. The Meter is
// required (counters must be registered); onAlert is optional (nil →
// default WARN log).
type MetricsConfig struct {
	Meter   *metrics.Meter
	OnAlert AlertHook
}

// NewD6ValidationMetrics builds the 4-counter instrument and primes
// the sliding window. If cfg.Meter is nil the function returns nil
// (caller treats as no-op).
func NewD6ValidationMetrics(cfg MetricsConfig) *D6ValidationMetrics {
	if cfg.Meter == nil {
		return nil
	}
	pass, err := cfg.Meter.Int64Counter("orchestration.d6.validation.pass")
	if err != nil {
		slog.Error("d7: failed to register pass counter", "err", err)
		return nil
	}
	fail, err := cfg.Meter.Int64Counter("orchestration.d6.validation.fail")
	if err != nil {
		slog.Error("d7: failed to register fail counter", "err", err)
		return nil
	}
	timeout, err := cfg.Meter.Int64Counter("orchestration.d6.validation.timeout")
	if err != nil {
		slog.Error("d7: failed to register timeout counter", "err", err)
		return nil
	}
	erc, err := cfg.Meter.Int64Counter("orchestration.d6.validation.error")
	if err != nil {
		slog.Error("d7: failed to register error counter", "err", err)
		return nil
	}
	hook := cfg.OnAlert
	if hook == nil {
		hook = defaultAlertHook
	}
	return &D6ValidationMetrics{
		pass:    pass,
		fail:    fail,
		timeout: timeout,
		error:   erc,
		onAlert: hook,
	}
}

// RecordPass increments the pass counter and updates the sliding window.
func (m *D6ValidationMetrics) RecordPass(at time.Time) {
	if m == nil {
		return
	}
	m.pass.Inc()
	m.recordOutcome(d6OutcomePass, at)
}

// RecordFail increments the fail counter and updates the sliding window.
func (m *D6ValidationMetrics) RecordFail(at time.Time) {
	if m == nil {
		return
	}
	m.fail.Inc()
	m.recordOutcome(d6OutcomeFail, at)
}

// RecordTimeout increments the timeout counter and updates the sliding window.
func (m *D6ValidationMetrics) RecordTimeout(at time.Time) {
	if m == nil {
		return
	}
	m.timeout.Inc()
	m.recordOutcome(d6OutcomeTimeout, at)
}

// RecordError increments the error counter and updates the sliding window.
func (m *D6ValidationMetrics) RecordError(at time.Time) {
	if m == nil {
		return
	}
	m.error.Inc()
	m.recordOutcome(d6OutcomeError, at)
}

// recordOutcome is the single insertion point that maintains the
// sliding window, recomputes the rate, and fires the alert hook.
func (m *D6ValidationMetrics) recordOutcome(outcome d6Outcome, at time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := at.Add(-d6RateWindow)
	pruned := m.window[:0]
	for _, s := range m.window {
		if s.at.After(cutoff) {
			pruned = append(pruned, s)
		}
	}
	m.window = append(pruned, d6Sample{at: at, outcome: outcome})

	m.rate = m.computeRateLocked()
	total := uint64(len(m.window))
	if m.rate > d6RateAlertThreshold && total >= d6MinSamples && m.onAlert != nil {
		// Fire the hook under the lock to keep rate atomic w.r.t. further
		// inserts. The hook must be non-blocking; the default WARN-log
		// implementation is.
		m.onAlert(m.rate, total)
	}
}

// computeRateLocked returns (timeout+error) / total within the current
// window. Caller must hold m.mu.
func (m *D6ValidationMetrics) computeRateLocked() float64 {
	var total, bad uint64
	for _, s := range m.window {
		total++
		if s.outcome == d6OutcomeTimeout || s.outcome == d6OutcomeError {
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
func (m *D6ValidationMetrics) TimeoutRate() float64 {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rate
}

// WindowSize returns the current number of samples in the sliding
// window. Useful for tests + cold-start debugging.
func (m *D6ValidationMetrics) WindowSize() int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.window)
}

// Counters returns the four counters for inspection (tests, exporters).
// Nil-safe.
func (m *D6ValidationMetrics) Counters() (pass, fail, timeout, err metrics.Counter) {
	if m == nil {
		return nil, nil, nil, nil
	}
	return m.pass, m.fail, m.timeout, m.error
}

// defaultAlertHook is the v1.0 default: log a structured WARN.
func defaultAlertHook(rate float64, samples uint64) {
	slog.Warn("d7: D6 validation timeout_rate alert",
		"rate", rate,
		"samples", samples,
		"threshold", d6RateAlertThreshold,
		"window", d6RateWindow,
	)
}
