package workmodel

import "testing"

func TestComputeUnifiedUncertainty(t *testing.T) {
	u := ComputeUnifiedUncertainty(UnifiedUncertaintyInput{
		WilsonLower:       0.2,
		ChildStats:        ChildOutcomeStats{Total: 4, Failed: 1},
		VerdictConfidence: 0.7,
		EvidenceCount:     2,
	})
	if u <= 0 || u > 1 {
		t.Fatalf("uncertainty = %v, want (0,1]", u)
	}
	// Higher failure / lower confidence → higher uncertainty
	high := ComputeUnifiedUncertainty(UnifiedUncertaintyInput{
		WilsonLower:       0.1,
		ChildStats:        ChildOutcomeStats{Total: 4, Failed: 3},
		VerdictConfidence: 0.3,
		EvidenceCount:     0,
	})
	if high <= u {
		t.Fatalf("high-risk uncertainty %v should exceed baseline %v", high, u)
	}
}

// T: L5-D7-U-01 — CC-U5 format failure with sufficient evidence must not inflate U like exploration need.
func TestComputeUnifiedUncertainty_formatFailureWithEvidenceDamps(t *testing.T) {
	base := UnifiedUncertaintyInput{
		WilsonLower:       0.2,
		ChildStats:        ChildOutcomeStats{Total: 4, Failed: 1},
		VerdictConfidence: 0.4,
		EvidenceCount:     2,
	}
	plain := ComputeUnifiedUncertainty(base)
	damped := ComputeUnifiedUncertainty(UnifiedUncertaintyInput{
		WilsonLower:               base.WilsonLower,
		ChildStats:                base.ChildStats,
		VerdictConfidence:         base.VerdictConfidence,
		EvidenceCount:             base.EvidenceCount,
		FormatFailureWithEvidence: true,
	})
	if damped >= plain {
		t.Fatalf("damped=%v should be below plain=%v when format failed with evidence", damped, plain)
	}
}

func TestDeliverableIncompleteObsStrength_convergenceDamps(t *testing.T) {
	low := DeliverableIncompleteObsStrength(&WorkItemPipelineRound{ExecuteToolCalls: 0})
	high := DeliverableIncompleteObsStrength(&WorkItemPipelineRound{
		ExecuteToolCalls: 3,
		ScopeInPresent:   true,
	})
	if high >= low {
		t.Fatalf("convergence strength=%v should be below default=%v", high, low)
	}
}
