package coordinator

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLLMFallbackClassifier_Classify(t *testing.T) {
	rule := NewRuleClassifier(RuleOrchestrateConfig())

	tests := []struct {
		name           string
		message        string
		minConfidence  int
		llmConfidence  int
		llmErr        error
		wantKind       IntentKind
		wantConfidence int
	}{
		{
			name:           "high confidence rule - no LLM call",
			message:       "/help",
			minConfidence: 70,
			llmConfidence: 50,
			wantKind:       IntentCommand,
			wantConfidence: 100,
		},
		{
			name:           "low confidence rule - LLM used when higher confidence",
			message:       "implement the new feature for user authentication system",
			minConfidence: 70,
			llmConfidence: 85,
			wantKind:       IntentOrchestrate,
			wantConfidence: 85,
		},
		{
			name:           "low confidence rule - LLM returns lower confidence",
			message:       "implement the new feature for user authentication system",
			minConfidence: 70,
			llmConfidence: 60,
			wantKind:       IntentOrchestrate,
			wantConfidence: 60,
		},
		{
			name:           "low confidence rule - LLM unavailable",
			message:       "implement the new feature for user authentication system",
			minConfidence: 70,
			llmErr:        errors.New("LLM unavailable"),
			wantKind:       IntentOrchestrate,
			wantConfidence: 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			llm := &mockLLMClassifier{
				confidence: tt.llmConfidence,
				err:        tt.llmErr,
			}

			classifier := NewLLMFallbackClassifier(rule, llm, tt.minConfidence)
			result, err := classifier.Classify(context.Background(), tt.message)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Kind != tt.wantKind {
				t.Errorf("Kind = %v, want %v", result.Kind, tt.wantKind)
			}

			if result.Confidence != tt.wantConfidence {
				t.Errorf("Confidence = %d, want %d", result.Confidence, tt.wantConfidence)
			}
		})
	}
}

func TestLLMFallbackClassifier_DefaultMinConfidence(t *testing.T) {
	rule := NewRuleClassifier(DefaultConfig())

	// Create classifier with zero minConfidence (should default to 70)
	classifier := NewLLMFallbackClassifier(rule, nil, 0)

	// High confidence message (/help) should not trigger LLM
	result, err := classifier.Classify(context.Background(), "/help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Confidence != 100 {
		t.Errorf("Confidence = %d, want 100", result.Confidence)
	}
}

type mockLLMClassifier struct {
	confidence int
	err       error
	onCall    func()
}

func (m *mockLLMClassifier) ClassifyIntent(ctx context.Context, message string) (IntentClassification, error) {
	if m.onCall != nil {
		m.onCall()
	}
	if m.err != nil {
		return IntentClassification{}, m.err
	}
	return IntentClassification{
		Kind:       IntentOrchestrate,
		Confidence: m.confidence,
		Reason:     "mock LLM",
	}, nil
}

func TestLLMFallbackClassifier_Timeout(t *testing.T) {
	rule := NewRuleClassifier(RuleOrchestrateConfig())

	slowLLM := &slowLLMClassifier{delay: 100 * time.Millisecond}
	classifier := NewLLMFallbackClassifier(rule, slowLLM, 70)
	classifier.timeout = 50 * time.Millisecond // 50ms timeout

	// This message is long enough to get orchestrate (low confidence)
	start := time.Now()
	result, err := classifier.Classify(context.Background(), "implement the new feature for user authentication system")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have fallen back to rule result due to timeout
	// Rule returns orchestrate with 60 confidence for long messages
	if result.Confidence != 60 {
		t.Errorf("Confidence = %d, want 60 (rule result)", result.Confidence)
	}

	// Should not have waited for slow LLM
	if elapsed > 100*time.Millisecond {
		t.Errorf("elapsed = %v, should not wait for slow LLM", elapsed)
	}
}

type slowLLMClassifier struct {
	delay time.Duration
}

func (s *slowLLMClassifier) ClassifyIntent(ctx context.Context, message string) (IntentClassification, error) {
	select {
	case <-ctx.Done():
		return IntentClassification{}, ctx.Err()
	case <-time.After(s.delay):
		return IntentClassification{
			Kind:       IntentOrchestrate,
			Confidence: 80,
			Reason:     "slow LLM",
		}, nil
	}
}
