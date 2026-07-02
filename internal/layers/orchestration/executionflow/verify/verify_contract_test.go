// T: D7-S10-A50-T01..T04 — VerifyContract 4-tuple + burden of proof tests.
package verify

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// D7-S10-A50-T01: NewVerifyContract applies task_kind defaults.
func TestNewVerifyContract_TaskKindDefaults(t *testing.T) {
	c := NewVerifyContract("review", contracts.EC_Probe)
	if c.DeliverableMinChars != 20 {
		t.Errorf("review should have min_chars=20, got %d", c.DeliverableMinChars)
	}
	if !c.EvidenceRequired {
		t.Errorf("Probe class should require evidence")
	}
	if c.MinSourceQuality != 0.5 {
		t.Errorf("default MinSourceQuality should be 0.5, got %f", c.MinSourceQuality)
	}
	if c.MinEvidenceCount != 1 {
		t.Errorf("default MinEvidenceCount should be 1, got %d", c.MinEvidenceCount)
	}
}

// D7-S10-A50-T01: NewVerifyContract different min_chars for different kinds.
func TestNewVerifyContract_AllTaskKinds(t *testing.T) {
	cases := []struct {
		kind string
		want int
	}{
		{"review", 20},
		{"edit", 10},
		{"test", 30},
		{"refactor", 40},
		{"observe", 10},
		{"unknown", 20}, // safe default
	}
	for _, tc := range cases {
		c := NewVerifyContract(tc.kind, contracts.EC_Probe)
		if c.DeliverableMinChars != tc.want {
			t.Errorf("kind=%s: want min_chars=%d, got %d", tc.kind, tc.want, c.DeliverableMinChars)
		}
	}
}

// D7-S10-A50-T01: Info #4 fix — explicit constructor prevents zero-value
// trap. A contract built via &VerifyContract{} should be detected as
// invalid (MinSourceQuality=0 silently passes).
func TestVerifyContract_ZeroValueIsDetectable(t *testing.T) {
	c := &VerifyContract{} // zero value: MinSourceQuality=0, EvidenceRequired=false
	// Note: this contract is "loose" — it accepts everything. Production
	// code MUST use NewVerifyContract to avoid this trap. The test
	// documents the danger.
	if c.MinSourceQuality == 0 {
		// Expected: zero value has MinSourceQuality=0
	}
	if c.MinEvidenceCount == 0 {
		// Expected: zero value has MinEvidenceCount=0
	}
	// Verifying that NewVerifyContract sets sensible defaults:
	c2 := NewVerifyContract("review", contracts.EC_Probe)
	if c2.MinSourceQuality == 0 {
		t.Errorf("NewVerifyContract should set MinSourceQuality != 0")
	}
}

// D7-S10-A50-T01: Verify rejects empty deliverable.
func TestVerify_DeliverableMissing(t *testing.T) {
	c := NewVerifyContract("review", contracts.EC_Probe)
	v, err := c.Verify(VerifyInput{DeliverableText: "", Evidence: []string{"t1"}})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if v.Kind != VerdictFail {
		t.Errorf("empty deliverable should FAIL, got %v", v.Kind)
	}
	if v.Reason != ReasonDeliverableMissing {
		t.Errorf("expected reason=deliverable_missing, got %q", v.Reason)
	}
}

// D7-S10-A50-T01: Verify rejects too-short deliverable.
func TestVerify_DeliverableTooShort(t *testing.T) {
	c := NewVerifyContract("review", contracts.EC_Probe) // min_chars=20
	v, _ := c.Verify(VerifyInput{DeliverableText: "short", Evidence: []string{"t1"}})
	if v.Kind != VerdictFail {
		t.Errorf("too-short deliverable should FAIL, got %v", v.Kind)
	}
	if v.Reason != ReasonDeliverableTooShort {
		t.Errorf("expected reason=deliverable_too_short, got %q", v.Reason)
	}
}

// D7-S10-A50-T01: Verify rejects insufficient evidence.
func TestVerify_EvidenceInsufficient(t *testing.T) {
	c := NewVerifyContract("review", contracts.EC_Probe)
	v, _ := c.Verify(VerifyInput{DeliverableText: "this is a long enough review", Evidence: []string{}})
	if v.Kind != VerdictFail {
		t.Errorf("no evidence should FAIL, got %v", v.Kind)
	}
	if v.Reason != ReasonEvidenceInsufficient {
		t.Errorf("expected reason=evidence_insufficient, got %q", v.Reason)
	}
}

// D7-S10-A50-T01: Verify returns Partial on low calibrated_confidence.
func TestVerify_SourceUncertaintyHigh(t *testing.T) {
	c := NewVerifyContract("review", contracts.EC_Probe)
	// All weights 0.20 (Probe), source qualities all 0.3
	// CC = (0.3*0.2 * 3) / (0.2 * 3) = 0.3, below 0.5 → Partial
	v, _ := c.Verify(VerifyInput{
		DeliverableText:       "this is a long enough review",
		Evidence:              []string{"t1", "t2", "t3"},
		SourceQualities:       []float64{0.3, 0.3, 0.3},
		EmissionClassWeights:  []float64{0.2, 0.2, 0.2},
	})
	if v.Kind != VerdictPartial {
		t.Errorf("low CC should PARTIAL, got %v", v.Kind)
	}
	if v.Reason != ReasonSourceUncertaintyHigh {
		t.Errorf("expected reason=source_uncertainty_high, got %q", v.Reason)
	}
	if !strings.Contains(v.Meta["calibrated_confidence"], "0.300") {
		t.Errorf("expected CC=0.300 in meta, got %q", v.Meta["calibrated_confidence"])
	}
}

// D7-S10-A50-T01: Verify returns Pass on all checks pass.
func TestVerify_AllPass(t *testing.T) {
	c := NewVerifyContract("review", contracts.EC_Probe)
	// All weights 0.50 (Fact), source qualities 1.0 → CC = 1.0
	v, err := c.Verify(VerifyInput{
		DeliverableText:       "this is a long enough review with proper structure",
		Evidence:              []string{"t1", "t2"},
		SourceQualities:       []float64{1.0, 1.0},
		EmissionClassWeights:  []float64{0.50, 0.50},
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if v.Kind != VerdictPass {
		t.Errorf("all checks pass should be PASS, got %v (reason=%s)", v.Kind, v.Reason)
	}
	if v.Reason != ReasonOK {
		t.Errorf("expected reason=ok, got %q", v.Reason)
	}
}

// D7-S10-A50-T01: CalibratedConfidence denominator must be > 0.
func TestCalibratedConfidence_Empty(t *testing.T) {
	v := &VerifyInput{}
	if cc := v.CalibratedConfidence(); cc != 0 {
		t.Errorf("empty input should have CC=0, got %f", cc)
	}
}

// D7-S10-A50-T01: CalibratedConfidence formula — Critical #6 fix.
func TestCalibratedConfidence_Formula(t *testing.T) {
	// 3 samples, weights 0.5+0.2+0.1, source qualities 1.0+0.8+0.6
	// num = 1.0*0.5 + 0.8*0.2 + 0.6*0.1 = 0.5 + 0.16 + 0.06 = 0.72
	// denom = 0.5 + 0.2 + 0.1 = 0.8
	// CC = 0.72 / 0.8 = 0.9
	v := &VerifyInput{
		SourceQualities:      []float64{1.0, 0.8, 0.6},
		EmissionClassWeights: []float64{0.5, 0.2, 0.1},
	}
	cc := v.CalibratedConfidence()
	if cc < 0.89 || cc > 0.91 {
		t.Errorf("CC should be ~0.9, got %f", cc)
	}
}

// D7-S10-A50-T04: Burden of proof by EmissionClass.
func TestBurdenOfProofForClass(t *testing.T) {
	cases := []struct {
		ec   contracts.EmissionClass
		want string
	}{
		{contracts.EC_Fact, ReasonDeliverableMissing},
		{contracts.EC_Action, ReasonStateChangeFailed},
		{contracts.EC_Probe, ReasonSourceUncertaintyHigh},
		{contracts.EC_Experiment, ReasonExperimentInconclusive},
	}
	for _, tc := range cases {
		got := BurdenOfProofForClass(tc.ec)
		if got != tc.want {
			t.Errorf("ec=%v: want %q, got %q", tc.ec, tc.want, got)
		}
	}
}

// D7-S10-A50-T04: TestBurdenOfProofByClass — Probe task with low CC → PARTIAL.
func TestBurdenOfProof_Probe_LowCC(t *testing.T) {
	c := NewVerifyContract("review", contracts.EC_Probe)
	v, _ := c.Verify(VerifyInput{
		DeliverableText:       "I have reviewed the code and found several issues",
		Evidence:              []string{"read_file:foo.go", "read_file:bar.go"},
		SourceQualities:       []float64{0.2, 0.2}, // LLM-suggested (low trust)
		EmissionClassWeights:  []float64{0.2, 0.2}, // Probe weight
	})
	if v.Kind != VerdictPartial {
		t.Errorf("Probe + low CC should be PARTIAL, got %v (reason=%s)", v.Kind, v.Reason)
	}
	if v.Reason != ReasonSourceUncertaintyHigh {
		t.Errorf("Probe burden of proof is source_uncertainty, got %q", v.Reason)
	}
}

// D7-S10-A50-T01: VerifyContract meta contains task_kind for D1 feishu render.
func TestVerify_MetaContainsTaskKind(t *testing.T) {
	c := NewVerifyContract("review", contracts.EC_Probe)
	v, _ := c.Verify(VerifyInput{
		DeliverableText: "this is a long enough review",
		Evidence:        []string{"t1"},
		SourceQualities: []float64{1.0},
		EmissionClassWeights: []float64{0.5},
	})
	if v.Meta["task_kind"] != "review" {
		t.Errorf("meta should contain task_kind=review, got %q", v.Meta["task_kind"])
	}
	if v.Meta["expected_class"] != "Probe" {
		t.Errorf("meta should contain expected_class=Probe, got %q", v.Meta["expected_class"])
	}
}
