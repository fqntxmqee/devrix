package coordinator

import (
	"context"
	"log/slog"
	"time"

	"github.com/devrix/devrix/internal/layers/observability/metrics"
)

// LLMIntentClassifier is the abstract LLM interface for intent classification.
//
// d7 does not depend on D3 (LLM Gateway) directly; any implementation
// (gateway adapter, test stub) satisfies this interface. Implementations
// must be safe for concurrent calls.
//
// v1.0 P1: only invoked from ShadowClassifier.tail (the ~20% rule
// falls through to IntentOrchestrate). v1.1 may promote it to a
// fallback decision path.
type LLMIntentClassifier interface {
	ClassifyIntent(ctx context.Context, message string) (IntentClassification, error)
}

// ShadowMetrics groups the 4 counters + 1 histogram that record the
// behavior of the LLM classify shadow. A nil *ShadowMetrics disables
// metric recording while leaving the LLM call path intact.
type ShadowMetrics struct {
	Match    metrics.Counter   // orchestration.intent.classify.shadow.match
	Mismatch metrics.Counter   // orchestration.intent.classify.shadow.mismatch
	Error    metrics.Counter   // orchestration.intent.classify.shadow.error
	Disabled metrics.Counter   // orchestration.intent.classify.shadow.disabled (LLM nil)
	Latency  metrics.Histogram // orchestration.intent.classify.shadow.latency_ms
}

// NewShadowMetrics registers the 5 metrics with the D5 observability
// Meter and returns the bundle. Returns nil if meter is nil or any
// registration fails (caller treats nil as no-op).
func NewShadowMetrics(meter *metrics.Meter) *ShadowMetrics {
	if meter == nil {
		return nil
	}
	match, err := meter.Int64Counter("orchestration.intent.classify.shadow.match")
	if err != nil {
		slog.Error("orchestrator: failed to register shadow match counter", "err", err)
		return nil
	}
	mismatch, err := meter.Int64Counter("orchestration.intent.classify.shadow.mismatch")
	if err != nil {
		slog.Error("orchestrator: failed to register shadow mismatch counter", "err", err)
		return nil
	}
	errC, err := meter.Int64Counter("orchestration.intent.classify.shadow.error")
	if err != nil {
		slog.Error("orchestrator: failed to register shadow error counter", "err", err)
		return nil
	}
	disabled, err := meter.Int64Counter("orchestration.intent.classify.shadow.disabled")
	if err != nil {
		slog.Error("orchestrator: failed to register shadow disabled counter", "err", err)
		return nil
	}
	latency, err := meter.Float64Histogram("orchestration.intent.classify.shadow.latency_ms",
		metrics.WithBounds(metrics.LLMHistogramBounds()))
	if err != nil {
		slog.Error("orchestrator: failed to register shadow latency histogram", "err", err)
		return nil
	}
	return &ShadowMetrics{
		Match:    match,
		Mismatch: mismatch,
		Error:    errC,
		Disabled: disabled,
		Latency:  latency,
	}
}

// ShadowClassifier wraps an IntentClassifier (rule) with an optional
// LLMIntentClassifier (shadow). When the rule returns IntentOrchestrate
// (the ~20% tail) the LLM is invoked asynchronously in a detached
// goroutine; the rule's decision is returned synchronously to the
// caller. The LLM result is recorded as match / mismatch / error
// metrics but never influences the return value.
//
// R2 §5 命题 C: "仅对规则未命中 tail（~20%）异步 LLM classify，结果
// 只写日志/样本库". v1.0 decision path is unchanged.
type ShadowClassifier struct {
	rule    IntentClassifier
	llm     LLMIntentClassifier // may be nil
	metrics *ShadowMetrics      // may be nil
	timeout time.Duration
	log     *slog.Logger
}

// NewShadowClassifier builds a ShadowClassifier. timeoutMs <= 0 →
// 500ms default. nil llm or nil metrics → shadow path is no-op
// (rule result returned as-is).
func NewShadowClassifier(rule IntentClassifier, llm LLMIntentClassifier, m *ShadowMetrics, timeoutMs int) *ShadowClassifier {
	timeout := 500 * time.Millisecond
	if timeoutMs > 0 {
		timeout = time.Duration(timeoutMs) * time.Millisecond
	}
	return &ShadowClassifier{
		rule:    rule,
		llm:     llm,
		metrics: m,
		timeout: timeout,
		log:     slog.Default(),
	}
}

// Classify implements IntentClassifier. It always returns the rule's
// decision synchronously. When the rule falls through to
// IntentOrchestrate AND llm is configured, the LLM is invoked
// asynchronously and the result is recorded in metrics + log.
//
// nil receiver → returns error (defensive; orchestrator must wire
// a non-nil classifier for the shadow to function).
func (s *ShadowClassifier) Classify(ctx context.Context, message string) (IntentClassification, error) {
	if s == nil {
		return IntentClassification{}, errShadowNil
	}
	result, err := s.rule.Classify(ctx, message)
	if err != nil {
		return result, err
	}
	// Tail-only: only run shadow when rule fell through to orchestrate.
	if result.Kind != IntentOrchestrate {
		return result, nil
	}
	if s.llm == nil {
		if s.metrics != nil {
			s.metrics.Disabled.Inc()
		}
		return result, nil
	}
	// Fire-and-forget. Detach from parent ctx so request cancellation
	// does not abort the shadow (R2 §5 命题 C: shadow 仅作 v1.1 准备).
	go s.shadowAsync(context.WithoutCancel(ctx), message, result)
	return result, nil
}

// shadowAsync is the goroutine body. It never panics out: a deferred
// recover upgrades any panic to an error metric.
func (s *ShadowClassifier) shadowAsync(ctx context.Context, message string, ruleResult IntentClassification) {
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("orchestrator: shadow classify panic recovered",
				"panic", r, "rule_kind", ruleResult.Kind)
			if s.metrics != nil {
				s.metrics.Error.Inc()
			}
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	start := time.Now()
	llmResult, err := s.llm.ClassifyIntent(ctx, message)
	elapsed := time.Since(start)
	if s.metrics != nil {
		s.metrics.Latency.Observe(float64(elapsed.Milliseconds()))
	}
	if err != nil {
		if s.metrics != nil {
			s.metrics.Error.Inc()
		}
		s.log.Warn("orchestrator: shadow classify error",
			"err", err,
			"latency_ms", elapsed.Milliseconds(),
			"rule_kind", ruleResult.Kind)
		return
	}
	if llmResult.Kind == ruleResult.Kind {
		if s.metrics != nil {
			s.metrics.Match.Inc()
		}
		return
	}
	if s.metrics != nil {
		s.metrics.Mismatch.Inc()
	}
	s.log.Info("orchestrator: shadow classify mismatch",
		"rule_kind", ruleResult.Kind,
		"rule_confidence", ruleResult.Confidence,
		"llm_kind", llmResult.Kind,
		"llm_confidence", llmResult.Confidence,
		"latency_ms", elapsed.Milliseconds())
}

// errShadowNil is returned when Classify is called on a nil receiver.
// Kept package-private; callers should not see this in normal use.
var errShadowNil = &shadowError{Message: "orchestrator: ShadowClassifier is nil"}

// shadowError is the package-private error type for ShadowClassifier
// internal errors. Distinct from D6 / D7 orchestrator errors so callers
// can identify the source.
type shadowError struct {
	Message string
}

func (e *shadowError) Error() string { return e.Message }
