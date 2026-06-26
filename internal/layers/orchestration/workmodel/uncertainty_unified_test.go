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
