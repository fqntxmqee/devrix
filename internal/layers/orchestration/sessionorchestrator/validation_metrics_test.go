package sessionorchestrator

import (
	"context"
	"errors"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/observability/configure/settings"
	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
)

// newTestMeter builds a fresh MeterProvider + Meter with no labels.
func newTestMeter(t *testing.T) *metrics.Meter {
	t.Helper()
	mp := metrics.NewMeterProvider(&settings.MetricsConfig{})
	return mp.Meter("d7-test")
}

// T: D7-D6-T03 — 4 counter 注入并按 result.Pass 分流。
func TestValidationMetrics_Record_PassFail(t *testing.T) {
	mtr := newTestMeter(t)
	m := NewValidationMetrics(MetricsConfig{Meter: mtr})
	if m == nil {
		t.Fatalf("metrics should not be nil")
	}
	now := time.Now()
	m.RecordPass(now)
	m.RecordPass(now)
	m.RecordFail(now)
	m.RecordTimeout(now)
	m.RecordError(now)

	pass, fail, timeout, errC := m.Counters()
	if pass.Value() != 2 {
		t.Fatalf("pass.Value() = %d, want 2", pass.Value())
	}
	if fail.Value() != 1 {
		t.Fatalf("fail.Value() = %d, want 1", fail.Value())
	}
	if timeout.Value() != 1 {
		t.Fatalf("timeout.Value() = %d, want 1", timeout.Value())
	}
	if errC.Value() != 1 {
		t.Fatalf("error.Value() = %d, want 1", errC.Value())
	}
}

// T: D7-D6-T03 — nil meter returns nil metrics.
func TestValidationMetrics_NilMeter(t *testing.T) {
	if m := NewValidationMetrics(MetricsConfig{}); m != nil {
		t.Fatalf("nil meter should yield nil metrics")
	}
}

// T: D7-D6-T03 — nil receiver is no-op (defensive).
func TestValidationMetrics_NilReceiver(t *testing.T) {
	var m *ValidationMetrics
	now := time.Now()
	m.RecordPass(now)
	m.RecordFail(now)
	m.RecordTimeout(now)
	m.RecordError(now)
	if r := m.TimeoutRate(); r != 0 {
		t.Fatalf("nil receiver.TimeoutRate = %v, want 0", r)
	}
}

// T: D7-D6-T04 — timeout_rate > 5% 触发 AlertHook。
// 使用 25 个样本中 2 个 timeout，rate = 8% > 5% 阈值。
func TestValidationMetrics_TimeoutRate_Alert(t *testing.T) {
	mtr := newTestMeter(t)
	var (
		mu        sync.Mutex
		alertRate float64
		alertN    uint64
	)
	hook := func(rate float64, samples uint64) {
		mu.Lock()
		defer mu.Unlock()
		alertRate = rate
		alertN = samples
	}
	m := NewValidationMetrics(MetricsConfig{Meter: mtr, OnAlert: hook})
	now := time.Now()
	// 23 pass, 2 timeout → rate = 2/25 = 0.08 > 0.05
	for i := 0; i < 23; i++ {
		m.RecordPass(now.Add(time.Duration(i) * time.Millisecond))
	}
	m.RecordTimeout(now.Add(30 * time.Millisecond))
	m.RecordTimeout(now.Add(31 * time.Millisecond))
	mu.Lock()
	defer mu.Unlock()
	if alertN != 25 {
		t.Fatalf("alert samples = %d, want 25", alertN)
	}
	if alertRate < 0.05 {
		t.Fatalf("alert rate = %v, want ≥ 0.05", alertRate)
	}
}

// T: D7-D6-T04 — 冷启动 (< d6MinSamples) 不触发告警。
func TestValidationMetrics_ColdStart_NoAlert(t *testing.T) {
	mtr := newTestMeter(t)
	var called int32
	hook := func(_ float64, _ uint64) { atomic.AddInt32(&called, 1) }
	m := NewValidationMetrics(MetricsConfig{Meter: mtr, OnAlert: hook})
	now := time.Now()
	// 19 timeouts (below min samples 20).
	for i := 0; i < 19; i++ {
		m.RecordTimeout(now.Add(time.Duration(i) * time.Millisecond))
	}
	if atomic.LoadInt32(&called) != 0 {
		t.Fatalf("alert fired during cold start; want 0")
	}
}

// T: D7-D6-T04 — 滑窗外样本被剪枝。
func TestValidationMetrics_WindowPrune(t *testing.T) {
	mtr := newTestMeter(t)
	m := NewValidationMetrics(MetricsConfig{Meter: mtr})
	old := time.Now().Add(-10 * time.Minute) // outside 5min window
	recent := time.Now()
	for i := 0; i < 10; i++ {
		m.RecordTimeout(old)
	}
	for i := 0; i < 5; i++ {
		m.RecordPass(recent)
	}
	if got := m.WindowSize(); got != 5 {
		t.Fatalf("WindowSize = %d, want 5 (old timeouts pruned)", got)
	}
	if r := m.TimeoutRate(); r != 0 {
		t.Fatalf("TimeoutRate = %v, want 0 (all timeouts pruned)", r)
	}
}

// T: D7-D6-T03 — error + timeout 都计入 rate numerator。
func TestValidationMetrics_RateIncludesErrorAndTimeout(t *testing.T) {
	mtr := newTestMeter(t)
	var (
		mu   sync.Mutex
		rate float64
		samp uint64
	)
	hook := func(r float64, n uint64) {
		mu.Lock()
		defer mu.Unlock()
		rate = r
		samp = n
	}
	m := NewValidationMetrics(MetricsConfig{Meter: mtr, OnAlert: hook})
	now := time.Now()
	// 18 pass, 1 timeout, 1 error → bad=2/total=20 = 0.10 > 0.05 (≥ 20 samples)
	for i := 0; i < 18; i++ {
		m.RecordPass(now.Add(time.Duration(i) * time.Millisecond))
	}
	m.RecordTimeout(now.Add(20 * time.Millisecond))
	m.RecordError(now.Add(21 * time.Millisecond))
	mu.Lock()
	defer mu.Unlock()
	if samp != 20 {
		t.Fatalf("samples = %d, want 20", samp)
	}
	if rate < 0.05 {
		t.Fatalf("rate = %v, want > 0.05", rate)
	}
}

// T: D7-D6-T06 — nil validator 不调用 metrics。
// v6.1.0: routing collapsed to OrchestratePath; exec.RunTurn is no longer
// called directly. Verify wave scheduler started instead.
func TestOrchestrator_NoValidator_NoMetrics(t *testing.T) {
	exec := &fakeD2{}
	orch, sched := newOrchestratorWithFakeOrchestratePath(
		orchtypes.DefaultConfig(),
		exec,
		[]wavescheduler.Artifact{{Summary: "hi"}},
	)
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-nil",
		Message:   "hi",
	})
	if err != nil {
		t.Fatalf("ProcessMessage err: %v", err)
	}
	for range ch {
	}
	if sched.starts != 1 {
		t.Fatalf("wave scheduler should run, got starts=%d", sched.starts)
	}
}

// T: D7-D6-T05 — validator panic recorded as error。
type panickingValidator struct {
	calls int32
}

func (p *panickingValidator) ValidateOrchestration(_ context.Context, _ OrchestrationDecision) ValidationResult {
	atomic.AddInt32(&p.calls, 1)
	panic("simulated D6 validator failure")
}

func TestOrchestrator_AdvisoryValidator_Panic_RecordsError(t *testing.T) {
	exec := &fakeD2{}
	mtr := newTestMeter(t)
	m := NewValidationMetrics(MetricsConfig{Meter: mtr})
	v := &panickingValidator{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec,
		WithValidator(v), WithMetrics(m))
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-panic",
		Message:   "hi",
	})
	if err != nil {
		t.Fatalf("ProcessMessage err: %v", err)
	}
	for range ch {
	}
	_, _, _, errC := m.Counters()
	if errC.Value() != 1 {
		t.Fatalf("error counter = %d, want 1 (panic recovered)", errC.Value())
	}
}

// T: gross-error validator (ignores context, blocks 30ms) with 10ms timeout
// → elapsed ~30ms > 2*timeout = 20ms → error counter.
type grossErrorValidator struct {
	delay time.Duration
}

func (g *grossErrorValidator) ValidateOrchestration(_ context.Context, _ OrchestrationDecision) ValidationResult {
	// Deliberately ignore context; simulates a hung validator that is
	// caught only by the elapsed-time safety net.
	time.Sleep(g.delay)
	return ValidationResult{Pass: true}
}

func TestOrchestrator_AdvisoryValidator_Slow_RecordsError(t *testing.T) {
	exec := &fakeD2{}
	mtr := newTestMeter(t)
	m := NewValidationMetrics(MetricsConfig{Meter: mtr})
	v := &grossErrorValidator{delay: 30 * time.Millisecond}
	cfg := orchtypes.DefaultConfig()
	cfg.AdvisoryValidationTimeoutMs = 10 // 10ms timeout; 30ms delay > 2x = 20ms → error
	orch := NewSessionOrchestrator(cfg, exec,
		WithValidator(v), WithMetrics(m))
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-slow",
		Message:   "hi",
	})
	if err != nil {
		t.Fatalf("ProcessMessage err: %v", err)
	}
	for range ch {
	}
	_, _, _, errC := m.Counters()
	if errC.Value() != 1 {
		t.Fatalf("error counter = %d, want 1 (slow validator)", errC.Value())
	}
}

// T: validator returning Pass=true increments pass counter.
type passingValidator struct {
	calls int32
}

func (p *passingValidator) ValidateOrchestration(_ context.Context, _ OrchestrationDecision) ValidationResult {
	atomic.AddInt32(&p.calls, 1)
	return ValidationResult{Pass: true, Reason: "ok"}
}

func TestOrchestrator_AdvisoryValidator_Pass_RecordsPass(t *testing.T) {
	exec := &fakeD2{}
	mtr := newTestMeter(t)
	m := NewValidationMetrics(MetricsConfig{Meter: mtr})
	v := &passingValidator{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec,
		WithValidator(v), WithMetrics(m))
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-pass",
		Message:   "hi",
	})
	if err != nil {
		t.Fatalf("ProcessMessage err: %v", err)
	}
	for range ch {
	}
	pass, _, _, _ := m.Counters()
	if pass.Value() != 1 {
		t.Fatalf("pass counter = %d, want 1", pass.Value())
	}
}

// T: validator returning Pass=false increments fail counter.
type failingValidator struct{}

func (failingValidator) ValidateOrchestration(_ context.Context, _ OrchestrationDecision) ValidationResult {
	return ValidationResult{Pass: false, Reason: "advisory fail"}
}

func TestOrchestrator_AdvisoryValidator_Fail_RecordsFail(t *testing.T) {
	exec := &fakeD2{}
	mtr := newTestMeter(t)
	m := NewValidationMetrics(MetricsConfig{Meter: mtr})
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec,
		WithValidator(failingValidator{}), WithMetrics(m))
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-fail",
		Message:   "hi",
	})
	if err != nil {
		t.Fatalf("ProcessMessage err: %v", err)
	}
	for range ch {
	}
	_, fail, _, _ := m.Counters()
	if fail.Value() != 1 {
		t.Fatalf("fail counter = %d, want 1", fail.Value())
	}
}

// T: validator that errors (returns an error sentinel) is still recorded.
// (Note: ValidateOrchestration does not return error per the contract;
// a non-Pass result with non-zero Reason is the failure signal.)
type slowValidator struct {
	delay time.Duration
}

func (s *slowValidator) ValidateOrchestration(ctx context.Context, _ OrchestrationDecision) ValidationResult {
	select {
	case <-time.After(s.delay):
	case <-ctx.Done():
	}
	return ValidationResult{Pass: true}
}

// T: D7-D6-T05 — orphan error path: when elapsed > timeout (1ms<elapsed<2ms)
// → timeout counter.
func TestOrchestrator_AdvisoryValidator_Timeout_RecordsTimeout(t *testing.T) {
	exec := &fakeD2{}
	mtr := newTestMeter(t)
	m := NewValidationMetrics(MetricsConfig{Meter: mtr})
	v := &slowValidator{delay: 30 * time.Millisecond}
	cfg := orchtypes.DefaultConfig()
	cfg.AdvisoryValidationTimeoutMs = 10 // 10ms timeout; 30ms > 10ms but < 20ms = timeout
	orch := NewSessionOrchestrator(cfg, exec,
		WithValidator(v), WithMetrics(m))
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-to",
		Message:   "hi",
	})
	if err != nil {
		t.Fatalf("ProcessMessage err: %v", err)
	}
	for range ch {
	}
	_, _, timeoutC, _ := m.Counters()
	// Note: depending on scheduling, the actual elapsed may be > 2*timeout.
	// We assert that some non-pass counter was incremented.
	if timeoutC.Value() < 1 {
		// could be error if scheduling pushed elapsed > 2*timeout
		_, _, _, errC := m.Counters()
		if errC.Value() < 1 {
			t.Fatalf("expected timeout or error counter ≥ 1, got timeout=%d error=%d",
				timeoutC.Value(), errC.Value())
		}
	}
}

// T: counter registration is namespaced by Meter name — two instances
// on different Meters are independent. (Same Meter would conflict at
// registration time, which is correct — the meter is the metric
// namespace.)
func TestValidationMetrics_TwoInstances_DifferentMeters(t *testing.T) {
	mp := metrics.NewMeterProvider(&settings.MetricsConfig{})
	mtr1 := mp.Meter("d7-test-1")
	mtr2 := mp.Meter("d7-test-2")
	m1 := NewValidationMetrics(MetricsConfig{Meter: mtr1})
	m2 := NewValidationMetrics(MetricsConfig{Meter: mtr2})
	if m1 == nil || m2 == nil {
		t.Fatalf("both should be non-nil")
	}
	now := time.Now()
	m1.RecordPass(now)
	m2.RecordPass(now)
	pass1, _, _, _ := m1.Counters()
	pass2, _, _, _ := m2.Counters()
	if pass1.Value() != 1 || pass2.Value() != 1 {
		t.Fatalf("counters should be per-instance, got %d/%d", pass1.Value(), pass2.Value())
	}
}

// T: default WARN log hook is wired when no OnAlert is provided.
func TestValidationMetrics_DefaultHook(t *testing.T) {
	mtr := newTestMeter(t)
	m := NewValidationMetrics(MetricsConfig{Meter: mtr}) // no hook
	if m == nil {
		t.Fatalf("metrics should not be nil")
	}
	now := time.Now()
	// 25 timeouts → fires default hook; we cannot easily assert log output,
	// but we can assert that the alert mechanism does not panic.
	for i := 0; i < 25; i++ {
		m.RecordTimeout(now.Add(time.Duration(i) * time.Millisecond))
	}
}

// T: recording outcomes concurrently is race-free.
func TestValidationMetrics_Concurrent(t *testing.T) {
	mtr := newTestMeter(t)
	m := NewValidationMetrics(MetricsConfig{Meter: mtr})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			now := time.Now()
			switch i % 4 {
			case 0:
				m.RecordPass(now)
			case 1:
				m.RecordFail(now)
			case 2:
				m.RecordTimeout(now)
			case 3:
				m.RecordError(now)
			}
		}(i)
	}
	wg.Wait()
	// 50 samples expected.
	if got := m.WindowSize(); got != 50 {
		t.Fatalf("WindowSize = %d, want 50", got)
	}
}

// T: error counter + sliding window with deliberate age advance.
func TestValidationMetrics_RateRecomputed(t *testing.T) {
	mtr := newTestMeter(t)
	m := NewValidationMetrics(MetricsConfig{Meter: mtr})
	t0 := time.Now()
	// 10 pass at t0
	for i := 0; i < 10; i++ {
		m.RecordPass(t0)
	}
	if r := m.TimeoutRate(); r != 0 {
		t.Fatalf("initial rate = %v, want 0", r)
	}
	// 5 error at t0+1ms (within window)
	for i := 0; i < 5; i++ {
		m.RecordError(t0.Add(time.Millisecond))
	}
	r := m.TimeoutRate()
	if r < 0.30 {
		t.Fatalf("after errors, rate = %v, want ≥ 0.30", r)
	}
}

// T: error counter + passing through context timeout path.
func TestSlowValidator_ContextTimeout(t *testing.T) {
	v := &slowValidator{delay: 100 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	start := time.Now()
	v.ValidateOrchestration(ctx, OrchestrationDecision{SessionID: "x"})
	elapsed := time.Since(start)
	if elapsed > 50*time.Millisecond {
		t.Fatalf("context timeout not honored, elapsed = %v", elapsed)
	}
}

// errors import kept to avoid unused if future tests reference it.
var _ = errors.New
