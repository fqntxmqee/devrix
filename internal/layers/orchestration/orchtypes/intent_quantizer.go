package orchtypes

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
)

// IntentClass is the 4-class quantized intent produced by IntentQuantizer.
// It is a finer-grained classification than IntentKind (Fast/Command/
// Orchestrate/Skip) — use IntentQuantizer for analytics, IntentKind for
// routing.
//
// The 4 classes correspond to doc 35 §三.1 "intent quantization" 4×4 quadrant:
//
//	Fact         — pure information query (e.g. "what is X?")
//	Command      — explicit imperative (e.g. "/status", "/plan")
//	Orchestrate  — multi-step task that needs planning
//	Skip         — empty or non-actionable
type IntentClass string

const (
	IntentClassFact        IntentClass = "fact"
	IntentClassCommand     IntentClass = "command"
	IntentClassOrchestrate IntentClass = "orchestrate"
	IntentClassSkip        IntentClass = "skip"
)

// IntentPayload is the result of IntentQuantizer.Quantize.
type IntentPayload struct {
	Class      IntentClass
	Confidence int
	Reason     string
	SubClass   string // optional sub-class (e.g. for fact: "weather", "definition")
}

// IntentQuantizer is the Phase 6 PR-F1 IntentQuantizer submodule (D7-S12-A41-T02).
//
// It quantizes raw user messages into 4 IntentClass values. The Quantize
// method is the baseline (no prior); QuantizeWithPrior applies
// AdaptivePrior.PriorBeta.Mean() as a confidence multiplier on top of
// the baseline.
//
// Concurrency-safe: no internal state mutated during Quantize /
// QuantizeWithPrior.
type IntentQuantizer struct {
	fastPatterns   []quantizeRule
	commandRegexes []*regexp.Regexp
	emptyPattern   *regexp.Regexp
}

type quantizeRule struct {
	pattern *regexp.Regexp
	class   IntentClass
	reason  string
}

// NewIntentQuantizer builds the v1.0 4-class quantizer from config.
//
// Rule order:
//  1. Empty / whitespace → Skip
//  2. Command whitelist (config.CommandWhitelist) → Command
//  3. Fast patterns (greetings, short factual lookups) → Fact
//  4. Otherwise → Orchestrate
func NewIntentQuantizer(cfg *Config) *IntentQuantizer {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	q := &IntentQuantizer{}
	for _, c := range cfg.CommandWhitelist {
		q.commandRegexes = append(q.commandRegexes,
			regexp.MustCompile(`^`+regexp.QuoteMeta(c)+`(?:\s|$)`))
	}
	q.fastPatterns = []quantizeRule{
		{regexp.MustCompile(`^(hi|hello|hey|你好|嗨)(?:$|[\s,.!?])`), IntentClassFact, "greeting"},
		{regexp.MustCompile(`^(thanks|thank you|thx|谢谢)(?:$|[\s,.!?])`), IntentClassFact, "thanks"},
		{regexp.MustCompile(`^(bye|goodbye|再见)(?:$|[\s,.!?])`), IntentClassFact, "goodbye"},
		{regexp.MustCompile(`^(what|when|where|who|why|怎么|什么|哪)(?:\s|$)`), IntentClassFact, "question"},
		{regexp.MustCompile(`^/help\b`), IntentClassCommand, "help command"},
		{regexp.MustCompile(`^/(status|version|ping)\b`), IntentClassCommand, "status command"},
	}
	q.emptyPattern = regexp.MustCompile(`^\s*$`)
	return q
}

// Quantize is the baseline (no prior) 4-class intent quantization.
// It mirrors the rule order of RuleClassifier.Classify but produces an
// IntentPayload (4 IntentClass) instead of an IntentClassification
// (4 IntentKind routing).
func (q *IntentQuantizer) Quantize(_ context.Context, message string) (IntentPayload, error) {
	if q.emptyPattern.MatchString(message) {
		return IntentPayload{
			Class:      IntentClassSkip,
			Confidence: 100,
			Reason:     "empty message",
		}, nil
	}
	trimmed := strings.TrimSpace(message)
	// Command-first
	for _, re := range q.commandRegexes {
		if re.MatchString(trimmed) {
			return IntentPayload{
				Class:      IntentClassCommand,
				Confidence: 100,
				Reason:     "command whitelist match",
				SubClass:   trimmed,
			}, nil
		}
	}
	// Fast patterns
	for _, r := range q.fastPatterns {
		if r.pattern.MatchString(trimmed) {
			return IntentPayload{
				Class:      r.class,
				Confidence: 95,
				Reason:     r.reason,
			}, nil
		}
	}
	// Short single-token messages default to Fact (informational).
	if len(trimmed) <= 32 && !strings.ContainsAny(trimmed, "\n;") {
		return IntentPayload{
			Class:      IntentClassFact,
			Confidence: 70,
			Reason:     "short single-line message",
		}, nil
	}
	// Otherwise → Orchestrate
	return IntentPayload{
		Class:      IntentClassOrchestrate,
		Confidence: 60,
		Reason:     "no fast pattern matched",
	}, nil
}

// QuantizeWithPrior applies AdaptivePrior.PriorBeta.Mean() as a
// confidence multiplier on top of the baseline Quantize output.
//
// prior == nil → returns baseline (no adjustment).
// prior.PriorBeta.Alpha+Beta == 0 (cold start) → Mean == 0 → no adjustment.
// prior.PriorBeta.Mean() > 0 → multiplies baseline confidence, clamped to [0, 100].
//
// Immutable: prior is not mutated. The returned IntentPayload is a new
// value (Reason is updated to include the prior mean for observability).
func (q *IntentQuantizer) QuantizeWithPrior(ctx context.Context, message string, prior *learn.AdaptivePrior) (IntentPayload, error) {
	payload, err := q.Quantize(ctx, message)
	if err != nil {
		return payload, err
	}
	if prior == nil {
		return payload, nil
	}
	mean := prior.PriorBeta.Mean()
	if mean == 0 {
		return payload, nil
	}
	adjusted := int(float64(payload.Confidence) * mean)
	if adjusted > 100 {
		adjusted = 100
	}
	if adjusted < 0 {
		adjusted = 0
	}
	payload.Confidence = adjusted
	payload.Reason = fmt.Sprintf("%s [prior.Mean=%.3f]", payload.Reason, mean)
	return payload, nil
}
