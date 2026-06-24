package decisionplanning

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
)

// IntentClassifier produces an orchtypes.IntentClassification from a raw user message.
//
// The v1.1 decision path is rule-only (per R2 OQ-3 resolution B-improved).
// Tests can swap in a custom IntentClassifier via SessionOrchestrator.WithClassifier.
//
// The classifier must:
//   - Be safe for the FastPath hot path: no allocations beyond the result;
//     sub-millisecond on the rule set.
//   - Be deterministic: same input → same output (rule order is the source
//     of priority).
//   - Honor CommandFirst: recognized commands short-circuit.
//
// Phase 6 PR-F2 (D7-S12-A42-T05) adds ClassifyWithPrior to the
// interface so SessionOrchestrator can inject AdaptivePrior uniformly
// across all classifier implementations.
type IntentClassifier interface {
	Classify(ctx context.Context, message string) (orchtypes.IntentClassification, error)
	ClassifyWithPrior(ctx context.Context, message string, prior *learn.AdaptivePrior) (orchtypes.IntentClassification, error)
}

// RuleClassifier is the rule-only implementation. It is concurrency-safe
// (no internal state mutated during Classify).
type RuleClassifier struct {
	cfg            *orchtypes.Config
	commandRegexes []*regexp.Regexp
	fastPatterns   []fastRule
	emptyPattern   *regexp.Regexp
}

// fastRule is a compiled rule for the FastPath. Lower-case matched; the
// regex must be anchored for full-message patterns.
type fastRule struct {
	pattern *regexp.Regexp
	reason  string
}

// NewRuleClassifier builds the v1.0 rule set from config.
//
// The rule order is:
//  1. Empty/whitespace → skip
//  2. Command whitelist → command
//  3. Fast patterns (greetings, short factual lookups) → fast
//  4. Otherwise → orchestrate
func NewRuleClassifier(cfg *orchtypes.Config) *RuleClassifier {
	if cfg == nil {
		cfg = orchtypes.DefaultConfig()
	}
	rc := &RuleClassifier{cfg: cfg}
	for _, c := range cfg.CommandWhitelist {
		// Match "/" + name at start of trimmed message.
		rc.commandRegexes = append(rc.commandRegexes,
			regexp.MustCompile(`^`+regexp.QuoteMeta(c)+`(?:\s|$)`))
	}
	rc.fastPatterns = []fastRule{
		{regexp.MustCompile(`^(hi|hello|hey|你好|嗨)(?:$|[\s,.!?])`), "greeting"},
		{regexp.MustCompile(`^(thanks|thank you|thx|谢谢)(?:$|[\s,.!?])`), "thanks"},
		{regexp.MustCompile(`^(bye|goodbye|再见)(?:$|[\s,.!?])`), "goodbye"},
		{regexp.MustCompile(`^/help\b`), "help command"},
		{regexp.MustCompile(`^/(status|version|ping)\b`), "status command"},
	}
	rc.emptyPattern = regexp.MustCompile(`^\s*$`)
	return rc
}

// Classify applies the rule set. It never errors; the error return is for
// the v1.1 LLM-fallback path.
func (c *RuleClassifier) Classify(_ context.Context, message string) (orchtypes.IntentClassification, error) {
	if c.emptyPattern.MatchString(message) {
		return orchtypes.IntentClassification{
			Kind:       orchtypes.IntentSkip,
			Confidence: 100,
			Reason:     "empty message",
		}, nil
	}
	trimmed := strings.TrimSpace(message)
	// Command-first (per R2 routing matrix): if a recognized command is at
	// the start, route to orchtypes.IntentCommand regardless of LLM-style content
	// later in the message.
	if c.cfg.CommandFirst {
		for _, re := range c.commandRegexes {
			if re.MatchString(trimmed) {
				// Extract the command token for downstream handler.
				cmd := trimmed
				if idx := strings.IndexAny(trimmed, " \t\n"); idx >= 0 {
					cmd = trimmed[:idx]
				}
				return orchtypes.IntentClassification{
					Kind:       orchtypes.IntentCommand,
					Confidence: 100,
					Reason:     "command whitelist match",
					Command:    cmd,
				}, nil
			}
		}
	}
	// Fast-path patterns.
	for _, r := range c.fastPatterns {
		if r.pattern.MatchString(trimmed) {
			return orchtypes.IntentClassification{
				Kind:       orchtypes.IntentFast,
				Confidence: 95,
				Reason:     r.reason,
			}, nil
		}
	}
	// Loop-first: all non-command messages enter the Turn loop; complexity
	// is decided inside the loop via tool calls (Clawcode-aligned harness).
	if c.cfg != nil && c.cfg.IsLoopFirst() {
		return orchtypes.IntentClassification{
			Kind:       orchtypes.IntentFast,
			Confidence: 100,
			Reason:     "loop_first_default",
		}, nil
	}
	// Short single-token messages default to FastPath with lower confidence
	// so they still go through the engine but get classified as fast.
	if len(trimmed) <= 32 && !strings.ContainsAny(trimmed, "\n;") {
		return orchtypes.IntentClassification{
			Kind:       orchtypes.IntentFast,
			Confidence: 70,
			Reason:     "short single-line message",
		}, nil
	}
	return orchtypes.IntentClassification{
		Kind:       orchtypes.IntentOrchestrate,
		Confidence: 60,
		Reason:     "no fast pattern matched",
	}, nil
}

// ClassifyWithPrior applies AdaptivePrior.PriorBeta.Mean() as a confidence
// multiplier on top of the baseline Classify output.
//
// Phase 6 PR-F1 (D7-S12-A41-T03 + A41 ClassifyWithPrior part). Used by
// SessionOrchestrator.ProcessMessage to inject the user's cross-session
// reputation into intent classification.
//
// Behavior:
//   - prior == nil → returns Classify (no adjustment).
//   - prior.PriorBeta.Alpha+Beta == 0 (cold start) → Mean == 0 → no adjustment.
//   - prior.PriorBeta.Mean() > 0 → multiplies baseline confidence, clamped to [0, 100].
//
// Immutable: prior is not mutated. The returned IntentClassification is a
// new value (Reason is updated to include the prior mean for observability).
func (c *RuleClassifier) ClassifyWithPrior(ctx context.Context, message string, prior *learn.AdaptivePrior) (orchtypes.IntentClassification, error) {
	result, err := c.Classify(ctx, message)
	if err != nil {
		return result, err
	}
	if prior == nil {
		return result, nil
	}
	mean := prior.PriorBeta.Mean()
	if mean == 0 {
		return result, nil
	}
	adjusted := int(float64(result.Confidence) * mean)
	if adjusted > 100 {
		adjusted = 100
	}
	if adjusted < 0 {
		adjusted = 0
	}
	result.Confidence = adjusted
	result.Reason = fmt.Sprintf("%s [prior.Mean=%.3f]", result.Reason, mean)
	return result, nil
}
