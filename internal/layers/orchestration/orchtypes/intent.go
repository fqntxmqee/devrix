package orchtypes

// IntentKind is the routing decision produced by ClassifyIntent.
type IntentKind string

const (
	IntentFast        IntentKind = "fast"
	IntentCommand     IntentKind = "command"
	IntentOrchestrate IntentKind = "orchestrate"
	IntentSkip        IntentKind = "skip"
)

// ClassifierSource identifies which path produced the classification.
// DM-20260630-011 (devrix-session-conclusion-completeness) — replaces
// the prior hardcoded `learn.classifier_source="rule"` span attribute
// (orchestrator.go:392). Rule-based and LLM-based classifiers carry
// different uncertainty signals downstream; observability distinguishes
// them so dashboards / D6 Evolution can filter by classifier path.
type ClassifierSource string

const (
	SourceRule   ClassifierSource = "rule"   // RuleClassifier.Classify / ClassifyWithPrior / ClassifyWithReport default
	SourceLLM    ClassifierSource = "llm"    // reserved for future LLM-driven classifier (devrix-d7-llm-classifier-promotion)
	SourceHybrid ClassifierSource = "hybrid" // LLM with rule prior (Phase 6 PR-F2 forward-compatible)
)

// IntentClassification is the result of ClassifyIntent.
type IntentClassification struct {
	Kind       IntentKind
	Confidence int
	Reason     string
	Command    string
	// Source identifies the classifier path that produced this result.
	// DM-20260630-011: default zero value is SourceRule (back-compat for
	// callers that pre-date the field).
	Source ClassifierSource
}

// WithSource returns a copy of the classification with the source set.
// Immutable: receiver is not mutated.
func (c IntentClassification) WithSource(s ClassifierSource) IntentClassification {
	c.Source = s
	return c
}
