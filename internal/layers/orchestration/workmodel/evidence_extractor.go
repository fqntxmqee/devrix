package workmodel

import (
	"context"
	"encoding/json"
	"fmt"
)

// EvidenceExtractor extracts structured Evidence from a Verifier LLM
// output. The interface is intentionally minimal (2 methods) so the
// Phase 5 Learn node and Phase 4 PR-D3 callers can swap implementations
// (LLM-based, regex-based, stub-based) without breaking contracts.
//
// Phase 4 PR-D3 introduces this so the Verify node can produce
// machine-readable Evidence that Phase 5 Learn's LearningAsset generators
// consume directly.
type EvidenceExtractor interface {
	// Extract parses a VerifierOutput and returns the list of Evidence
	// records contained within. Returns an error if the output is
	// malformed or contains no parseable evidence.
	Extract(ctx context.Context, verifierOutput VerifierOutput) ([]Evidence, error)

	// Validate verifies that the Evidence list satisfies invariants
	// (non-empty, each Evidence validates). Returns an error on the
	// first violation.
	Validate(evidence []Evidence) error
}

// LLMEvidenceExtractor extracts Evidence from JSON-formatted Verifier LLM
// output. The expected JSON shape is:
//
//	{
//	  "evidences": [
//	    {"reason": "...", "confidence": 0.9, "counterexample": "..."},
//	    ...
//	  ]
//	}
//
// Or alternatively, a single Evidence object:
//
//	{"reason": "...", "confidence": 0.9}
//
// This mirrors doc 17 §4 L2 verifier prompt conventions.
type LLMEvidenceExtractor struct{}

// NewLLMEvidenceExtractor returns a default LLMEvidenceExtractor.
func NewLLMEvidenceExtractor() *LLMEvidenceExtractor {
	return &LLMEvidenceExtractor{}
}

// Extract parses the raw verifier output and returns the Evidence list.
func (l *LLMEvidenceExtractor) Extract(ctx context.Context, v VerifierOutput) ([]Evidence, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("evidence_extractor: ctx cancelled: %w", err)
	}
	if v.Raw == "" {
		return nil, nil // empty raw → empty evidence (not an error)
	}

	// Try list form first.
	var listForm struct {
		Evidences []struct {
			Reason         string  `json:"reason"`
			Confidence     float64 `json:"confidence"`
			Counterexample string  `json:"counterexample"`
		} `json:"evidences"`
	}
	if err := json.Unmarshal([]byte(v.Raw), &listForm); err == nil && len(listForm.Evidences) > 0 {
		return l.evidenceFromList(listForm.Evidences, v)
	}

	// Fall back to single object form.
	var singleForm struct {
		Reason         string  `json:"reason"`
		Confidence     float64 `json:"confidence"`
		Counterexample string  `json:"counterexample"`
	}
	if err := json.Unmarshal([]byte(v.Raw), &singleForm); err == nil && singleForm.Reason != "" {
		sourceRef := v.SourceID
		if sourceRef == "" {
			sourceRef = "verifier_output"
		}
		ev, err := NewEvidence(singleForm.Reason, singleForm.Confidence, sourceRef)
		if err != nil {
			return nil, err
		}
		if singleForm.Counterexample != "" {
			ev = ev.WithCounterexample(singleForm.Counterexample)
		}
		return []Evidence{ev}, nil
	}

	return nil, fmt.Errorf("evidence_extractor: malformed LLM output (no evidences list or single reason field): %q", truncate(v.Raw, 80))
}

func (l *LLMEvidenceExtractor) evidenceFromList(items []struct {
	Reason         string  `json:"reason"`
	Confidence     float64 `json:"confidence"`
	Counterexample string  `json:"counterexample"`
}, v VerifierOutput,
) ([]Evidence, error) {
	out := make([]Evidence, 0, len(items))
	for i, item := range items {
		sourceRef := v.SourceID
		if sourceRef == "" {
			sourceRef = fmt.Sprintf("verifier_output[%d]", i)
		}
		ev, err := NewEvidence(item.Reason, item.Confidence, sourceRef)
		if err != nil {
			return nil, fmt.Errorf("evidence_extractor: evidences[%d]: %w", i, err)
		}
		if item.Counterexample != "" {
			ev = ev.WithCounterexample(item.Counterexample)
		}
		out = append(out, ev)
	}
	return out, nil
}

// Validate verifies the Evidence list invariants.
func (l *LLMEvidenceExtractor) Validate(evidence []Evidence) error {
	if len(evidence) == 0 {
		return fmt.Errorf("evidence_extractor: empty evidence list")
	}
	for i, e := range evidence {
		if err := e.Validate(); err != nil {
			return fmt.Errorf("evidence_extractor: evidence[%d]: %w", i, err)
		}
	}
	return nil
}

// StubEvidenceExtractor returns a fixed Evidence list, useful for tests
// that need a deterministic extractor without an LLM.
type StubEvidenceExtractor struct {
	Evidence []Evidence
	Err      error
}

// NewStubEvidenceExtractor returns a StubEvidenceExtractor that yields
// the given evidence list (and error if non-nil).
func NewStubEvidenceExtractor(ev []Evidence, err error) *StubEvidenceExtractor {
	return &StubEvidenceExtractor{Evidence: ev, Err: err}
}

// Extract returns the stubbed evidence (and error).
func (s *StubEvidenceExtractor) Extract(ctx context.Context, v VerifierOutput) ([]Evidence, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.Err != nil {
		return nil, s.Err
	}
	return s.Evidence, nil
}

// Validate delegates to the same logic as LLMEvidenceExtractor.
func (s *StubEvidenceExtractor) Validate(evidence []Evidence) error {
	return (&LLMEvidenceExtractor{}).Validate(evidence)
}

// truncate shortens s to at most n runes for safe error messages.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}