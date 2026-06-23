package workmodel

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Evidence captures structured judgement behind a Verifier Verdict. It is
// the per-rule "why" of a PASS / PARTIAL / FAIL / INDETERMINATE outcome,
// designed to be machine-readable by Phase 5 Learn's LearningAsset
// generators (SOPAsset / ProtocolAsset / KnowledgeAsset / ConclusionAsset).
//
// Phase 4 PR-D3 (DM-20260623-002) introduces this struct so the Learn
// node can consume evidence without re-parsing raw verifier LLM output.
//
// Fields:
//
//	Reason         — natural language explanation of the verdict
//	Confidence     — confidence in [0,1] (clamped)
//	Counterexample — optional counterexample demonstrating the verdict
//	SourceRef      — source identifier (Plan ID / Observation ID / Verifier ID)
//	ExtractedAt    — extraction timestamp
type Evidence struct {
	Reason         string    `json:"reason"`
	Confidence     float64   `json:"confidence,omitempty"`
	Counterexample string    `json:"counterexample,omitempty"`
	SourceRef      string    `json:"source_ref,omitempty"`
	ExtractedAt    time.Time `json:"extracted_at"`
}

// NewEvidence constructs a new Evidence. Reason and SourceRef are
// required; Confidence is clamped to [0,1]. Returns an error if any
// required field is empty.
//
// This is the canonical constructor — direct struct literals should be
// avoided so the validation invariant is centralised.
func NewEvidence(reason string, confidence float64, sourceRef string) (Evidence, error) {
	if reason == "" {
		return Evidence{}, errors.New("workmodel: Evidence.Reason is required")
	}
	if sourceRef == "" {
		return Evidence{}, errors.New("workmodel: Evidence.SourceRef is required")
	}
	return Evidence{
		Reason:      reason,
		Confidence:  clamp01OrFallback(confidence, 0.5),
		SourceRef:   sourceRef,
		ExtractedAt: time.Now(),
	}, nil
}

// Validate verifies the Evidence invariant (Reason + SourceRef non-empty,
// Confidence ∈ [0,1]). Counterexample is optional.
func (e Evidence) Validate() error {
	if e.Reason == "" {
		return errors.New("workmodel: Evidence.Reason is required")
	}
	if e.Confidence < 0 || e.Confidence > 1 {
		return fmt.Errorf("workmodel: Evidence.Confidence %.3f out of [0,1]", e.Confidence)
	}
	if e.SourceRef == "" {
		return errors.New("workmodel: Evidence.SourceRef is required")
	}
	return nil
}

// WithCounterexample returns a copy with the Counterexample set.
func (e Evidence) WithCounterexample(ce string) Evidence {
	e.Counterexample = ce
	return e
}

// WithConfidence returns a copy with the new Confidence (clamped).
func (e Evidence) WithConfidence(c float64) Evidence {
	e.Confidence = clamp01OrFallback(c, 0.5)
	return e
}

// MarshalJSON encodes Evidence with all fields (Reason + ExtractedAt
// required; Confidence/Counterexample/SourceRef omitempty).
func (e Evidence) MarshalJSON() ([]byte, error) {
	type alias Evidence
	return json.Marshal(alias(e))
}

// UnmarshalJSON is the symmetric counterpart.
func (e *Evidence) UnmarshalJSON(data []byte) error {
	type alias Evidence
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*e = Evidence(a)
	if e.ExtractedAt.IsZero() {
		e.ExtractedAt = time.Now()
	}
	return nil
}