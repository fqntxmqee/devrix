package coordinator

import (
	"context"
	"regexp"
	"strings"
)

// IntentClassifier produces an IntentClassification from a raw user message.
//
// The v1.0 decision path is rule-only (per R2 OQ-3 resolution B-improved:
// tail shadow only — see ShadowClassifier). LLM-based classification
// surfaces via SetLLMClassifier (rule-and-LLM merge) when wired.
//
// The classifier must:
//   - Be safe for the FastPath hot path: no allocations beyond the result;
//     sub-millisecond on the rule set.
//   - Be deterministic: same input → same output (rule order is the source
//     of priority).
//   - Honor CommandFirst: recognized commands short-circuit.
type IntentClassifier interface {
	Classify(ctx context.Context, message string) (IntentClassification, error)
}

// RuleClassifier is the rule-only implementation. It is concurrency-safe
// (no internal state mutated during Classify). For LLM-augmented paths,
// see classifier_fallback.go.
type RuleClassifier struct {
	cfg            *Config
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
func NewRuleClassifier(cfg *Config) *RuleClassifier {
	if cfg == nil {
		cfg = DefaultConfig()
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
func (c *RuleClassifier) Classify(_ context.Context, message string) (IntentClassification, error) {
	if c.emptyPattern.MatchString(message) {
		return IntentClassification{
			Kind:       IntentSkip,
			Confidence: 100,
			Reason:     "empty message",
		}, nil
	}
	trimmed := strings.TrimSpace(message)
	// Command-first (per R2 routing matrix): if a recognized command is at
	// the start, route to IntentCommand regardless of LLM-style content
	// later in the message.
	if c.cfg.CommandFirst {
		for _, re := range c.commandRegexes {
			if re.MatchString(trimmed) {
				// Extract the command token for downstream handler.
				cmd := trimmed
				if idx := strings.IndexAny(trimmed, " \t\n"); idx >= 0 {
					cmd = trimmed[:idx]
				}
				return IntentClassification{
					Kind:       IntentCommand,
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
			return IntentClassification{
				Kind:       IntentFast,
				Confidence: 95,
				Reason:     r.reason,
			}, nil
		}
	}
	// Loop-first: all non-command messages enter the Turn loop; complexity
	// is decided inside the loop via tool calls (Clawcode-aligned harness).
	if c.cfg != nil && c.cfg.IsLoopFirst() {
		return IntentClassification{
			Kind:       IntentFast,
			Confidence: 100,
			Reason:     "loop_first_default",
		}, nil
	}
	// Short single-token messages default to FastPath with lower confidence
	// so they still go through the engine but get classified as fast.
	if len(trimmed) <= 32 && !strings.ContainsAny(trimmed, "\n;") {
		return IntentClassification{
			Kind:       IntentFast,
			Confidence: 70,
			Reason:     "short single-line message",
		}, nil
	}
	return IntentClassification{
		Kind:       IntentOrchestrate,
		Confidence: 60,
		Reason:     "no fast pattern matched",
	}, nil
}

// ClassifyLegacyTail applies rule_orchestrate tail logic (skip loop_first default).
// Used by ShadowClassifier to decide loop_first shadow samples without affecting routing.
func (c *RuleClassifier) ClassifyLegacyTail(_ context.Context, message string) (IntentClassification, error) {
	if c.emptyPattern.MatchString(message) {
		return IntentClassification{Kind: IntentSkip, Confidence: 100, Reason: "empty message"}, nil
	}
	trimmed := strings.TrimSpace(message)
	if c.cfg != nil && c.cfg.CommandFirst {
		for _, re := range c.commandRegexes {
			if re.MatchString(trimmed) {
				cmd := trimmed
				if idx := strings.IndexAny(trimmed, " \t\n"); idx >= 0 {
					cmd = trimmed[:idx]
				}
				return IntentClassification{
					Kind: IntentCommand, Confidence: 100, Reason: "command whitelist match", Command: cmd,
				}, nil
			}
		}
	}
	for _, r := range c.fastPatterns {
		if r.pattern.MatchString(trimmed) {
			return IntentClassification{Kind: IntentFast, Confidence: 95, Reason: r.reason}, nil
		}
	}
	if len(trimmed) <= 32 && !strings.ContainsAny(trimmed, "\n;") {
		return IntentClassification{
			Kind: IntentFast, Confidence: 70, Reason: "short single-line message",
		}, nil
	}
	return IntentClassification{
		Kind: IntentOrchestrate, Confidence: 60, Reason: "no fast pattern matched",
	}, nil
}
