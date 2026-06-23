package learn

import (
	"testing"
	"time"
)

func TestSOPAssetContent_Validate_NameRequired(t *testing.T) {
	c := &SOPAssetContent{Name: "", Steps: []string{"s1"}}
	if err := c.Validate(); err == nil {
		t.Error("empty Name should fail Validate()")
	}
}

func TestSOPAssetContent_Validate_StepsRequired(t *testing.T) {
	c := &SOPAssetContent{Name: "name", Steps: []string{}}
	if err := c.Validate(); err == nil {
		t.Error("empty Steps should fail Validate()")
	}
}

func TestSOPAssetContent_Validate_PassesValid(t *testing.T) {
	c := &SOPAssetContent{Name: "name", Steps: []string{"s1", "s2"}}
	if err := c.Validate(); err != nil {
		t.Errorf("valid SOP should pass; got %v", err)
	}
}

func TestProtocolAssetContent_Validate_TriggerRequired(t *testing.T) {
	c := &ProtocolAssetContent{Name: "p", Trigger: ""}
	if err := c.Validate(); err == nil {
		t.Error("empty Trigger should fail Validate()")
	}
}

func TestProtocolAssetContent_Validate_PassesValid(t *testing.T) {
	c := &ProtocolAssetContent{
		Name:    "p",
		Trigger: "p95 > 500ms",
		SLA:     SLAConfig{TargetMs: 100, MaxRetries: 3},
	}
	if err := c.Validate(); err != nil {
		t.Errorf("valid Protocol should pass; got %v", err)
	}
}

func TestKnowledgeAssetContent_Validate_TopicRequired(t *testing.T) {
	c := &KnowledgeAssetContent{Topic: "", Hypothesis: "h"}
	if err := c.Validate(); err == nil {
		t.Error("empty Topic should fail Validate()")
	}
}

func TestKnowledgeAssetContent_Validate_HypothesisRequired(t *testing.T) {
	c := &KnowledgeAssetContent{Topic: "t", Hypothesis: ""}
	if err := c.Validate(); err == nil {
		t.Error("empty Hypothesis should fail Validate()")
	}
}

func TestKnowledgeAssetContent_Validate_ConfidenceRange(t *testing.T) {
	cases := []struct {
		confidence float64
		want       bool
	}{
		{0.0, true},
		{0.5, true},
		{1.0, true},
		{-0.1, false},
		{1.1, false},
	}
	for _, tc := range cases {
		c := &KnowledgeAssetContent{Topic: "t", Hypothesis: "h", Confidence: tc.confidence}
		err := c.Validate()
		got := err == nil
		if got != tc.want {
			t.Errorf("Confidence=%v: got valid=%v, want %v (err=%v)", tc.confidence, got, tc.want, err)
		}
	}
}

func TestConclusionAssetContent_Validate_StatementRequired(t *testing.T) {
	c := &ConclusionAssetContent{Statement: ""}
	if err := c.Validate(); err == nil {
		t.Error("empty Statement should fail Validate()")
	}
}

func TestConclusionAssetContent_Validate_PassesValid(t *testing.T) {
	c := &ConclusionAssetContent{
		Statement:          "p<0.05 hypothesis confirmed",
		PValue:             0.03,
		ConfidenceInterval: [2]float64{0.6, 0.9},
		SampleSize:         100,
	}
	if err := c.Validate(); err != nil {
		t.Errorf("valid Conclusion should pass; got %v", err)
	}
}

func TestPendingAssetContent_Validate_IndeterminateReasonRequired(t *testing.T) {
	c := &PendingAssetContent{OriginalArtifactID: "art_1"}
	if err := c.Validate(); err == nil {
		t.Error("empty IndeterminateReason should fail Validate()")
	}
}

func TestPendingAssetContent_Validate_OriginalArtifactIDRequired(t *testing.T) {
	c := &PendingAssetContent{IndeterminateReason: "env_limited"}
	if err := c.Validate(); err == nil {
		t.Error("empty OriginalArtifactID should fail Validate()")
	}
}

func TestPendingAssetContent_Validate_RetryAttemptsRange(t *testing.T) {
	cases := []struct {
		attempts int
		want     bool
	}{
		{0, true},
		{1, true},
		{2, true},
		{3, true},
		{-1, false},
		{4, false},
	}
	for _, tc := range cases {
		c := &PendingAssetContent{
			IndeterminateReason: "env_limited",
			OriginalArtifactID:  "art_1",
			RetryAttempts:       tc.attempts,
		}
		err := c.Validate()
		got := err == nil
		if got != tc.want {
			t.Errorf("RetryAttempts=%d: got valid=%v, want %v (err=%v)", tc.attempts, got, tc.want, err)
		}
	}
}

func TestPendingAssetContent_Validate_MVEStateQuestionRequired(t *testing.T) {
	c := &PendingAssetContent{
		IndeterminateReason: "user_decision_pending",
		OriginalArtifactID:  "art_1",
		RetryAttempts:       0,
		MaxRetries:          3,
		NextRetryAt:         time.Now().Add(time.Hour),
		MVEState: &PendingMVEState{
			Round:            1,
			Mode:             "interactive",
			StrategyDecision: "ask_now",
		},
		// Question is empty → should fail
	}
	if err := c.Validate(); err == nil {
		t.Error("MVEState non-nil with empty Question should fail Validate()")
	}

	c.Question = "Which approach do you prefer?"
	if err := c.Validate(); err != nil {
		t.Errorf("MVEState with Question should pass; got %v", err)
	}
}

func TestPendingAssetContent_Validate_NoMVEStateIsOK(t *testing.T) {
	c := &PendingAssetContent{
		IndeterminateReason: "verifier_parse_failure",
		OriginalArtifactID:  "art_1",
		RetryAttempts:       0,
		MaxRetries:          3,
		NextRetryAt:         time.Now().Add(time.Hour),
		// No MVEState → Question not required
	}
	if err := c.Validate(); err != nil {
		t.Errorf("Pending without MVEState should pass; got %v", err)
	}
}

func TestAssetContent_SchemaVersion_Current(t *testing.T) {
	cases := []AssetContent{
		&SOPAssetContent{Name: "x", Steps: []string{"a"}},
		&ProtocolAssetContent{Trigger: "t"},
		&KnowledgeAssetContent{Topic: "t", Hypothesis: "h"},
		&ConclusionAssetContent{Statement: "s"},
		&PendingAssetContent{IndeterminateReason: "r", OriginalArtifactID: "a"},
	}
	for _, c := range cases {
		if got := c.SchemaVersion(); got != CurrentAssetSchemaVersion {
			t.Errorf("SchemaVersion() = %q, want %q", got, CurrentAssetSchemaVersion)
		}
	}
}

func TestAssetContent_ByteSize_Positive(t *testing.T) {
	cases := []AssetContent{
		&SOPAssetContent{Name: "x", Steps: []string{"a"}},
		&ProtocolAssetContent{Trigger: "t"},
		&KnowledgeAssetContent{Topic: "t", Hypothesis: "h"},
		&ConclusionAssetContent{Statement: "s"},
		&PendingAssetContent{IndeterminateReason: "r", OriginalArtifactID: "a"},
	}
	for _, c := range cases {
		if got := c.ByteSize(); got <= 0 {
			t.Errorf("ByteSize() = %d, want > 0", got)
		}
	}
}

func TestAssetContent_InterfaceCompliance(t *testing.T) {
	// Compile-time check: all 5 types implement AssetContent.
	var _ AssetContent = &SOPAssetContent{}
	var _ AssetContent = &ProtocolAssetContent{}
	var _ AssetContent = &KnowledgeAssetContent{}
	var _ AssetContent = &ConclusionAssetContent{}
	var _ AssetContent = &PendingAssetContent{}
}