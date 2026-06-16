package coordinator

import (
	"context"
	"time"
)

// LLMFallbackClassifier wraps a rule-based IntentClassifier and falls back
// to an LLM classifier when confidence is below the threshold.
//
// Deprecated: LLM fallback classification is deferred to v1.1 (LLMFallback
// config defaults to false, no production call site creates this classifier).
// Code and tests are kept for future reference.
// v1.0 behavior (ShadowClassifier): LLM is shadow-only, rule result always returned.
// v1.1 behavior (this classifier): LLM is called when confidence < minConfidence,
// and its result is returned if it has higher confidence than the rule.
type LLMFallbackClassifier struct {
	rule          IntentClassifier
	llm           LLMIntentClassifier
	minConfidence int // threshold below which LLM is invoked
	timeout       time.Duration
}

// NewLLMFallbackClassifier creates a classifier that uses LLM fallback.
// minConfidence is the minimum confidence required from the rule-based
// classifier before resorting to LLM classification. Common values:
//   - 80: high bar, only very confident rule matches are accepted
//   - 70: medium bar (recommended for production)
//   - 60: low bar, only explicit commands/fast patterns avoid LLM
//
// If minConfidence <= 0, defaults to 70.
// If llm is nil, falls back to direct rule evaluation (no LLM).
func NewLLMFallbackClassifier(rule IntentClassifier, llm LLMIntentClassifier, minConfidence int) *LLMFallbackClassifier {
	if minConfidence <= 0 {
		minConfidence = 70
	}
	return &LLMFallbackClassifier{
		rule:          rule,
		llm:           llm,
		minConfidence: minConfidence,
		timeout:       500 * time.Millisecond,
	}
}

// Classify implements IntentClassifier.
// If rule confidence >= minConfidence, returns rule result.
// Otherwise, if LLM is configured, calls LLM and returns its result if higher confidence.
// Falls back to rule result if LLM fails or returns lower confidence.
func (c *LLMFallbackClassifier) Classify(ctx context.Context, message string) (IntentClassification, error) {
	// First, get rule-based classification
	ruleResult, err := c.rule.Classify(ctx, message)
	if err != nil {
		return ruleResult, err
	}

	// If rule confidence is high enough, use it
	if ruleResult.Confidence >= c.minConfidence {
		return ruleResult, nil
	}

	// Rule confidence is low - try LLM if available
	if c.llm == nil {
		return ruleResult, nil
	}

	// Create timeout context for LLM call
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	llmResult, err := c.llm.ClassifyIntent(ctx, message)
	if err != nil {
		// LLM failed - fall back to rule result
		return ruleResult, nil
	}

	// Use LLM result if it has higher confidence
	if llmResult.Confidence > ruleResult.Confidence {
		llmResult.Reason = "llm_fallback: " + llmResult.Reason
		return llmResult, nil
	}

	// Rule confidence was higher or equal - use rule result
	return ruleResult, nil
}
