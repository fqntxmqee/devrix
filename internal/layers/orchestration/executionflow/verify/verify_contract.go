// Package verify contains the D7 Verify node's 4-tuple input contract
// that strengthens the existing 1-dimensional deliverable check into a
// proper multi-dimensional audit (demand.md RC-4 + H5 + P0-AC-4).
//
// DSAFT: D7-S10-A50-T01..T04.
// Change: devrix-mups-tool-classification-and-channel-autonomy (DM-20260701-007) Phase C.
package verify

import (
	"errors"
	"fmt"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// VerdictKind is the 4-state outcome of the Verify audit.
// Distinct from orchtypes.VerdictKind (which is a 4-state enum) —
// this struct is the verify-package specific type, kept separate to
// avoid cross-package churn.
type VerdictKind int

const (
	// VerdictPass — all 4-tuple checks pass.
	VerdictPass VerdictKind = iota

	// VerdictFail — at least one mandatory check failed (deliverable
	// missing, evidence insufficient, etc.).
	VerdictFail

	// VerdictPartial — soft check failed (calibrated_confidence below
	// threshold) but the deliverable is present and acceptable.
	VerdictPartial

	// VerdictIndeterminate — verifier could not decide (e.g. unable to
	// parse the LLM output).
	VerdictIndeterminate
)

// String returns the symbolic name.
func (k VerdictKind) String() string {
	switch k {
	case VerdictPass:
		return "Pass"
	case VerdictFail:
		return "Fail"
	case VerdictPartial:
		return "Partial"
	case VerdictIndeterminate:
		return "Indeterminate"
	}
	return "Unknown"
}

// VerifyContract is the 4-tuple input contract that the Verify node
// MUST validate before issuing a Verdict (demand §3.3 + specs/verify-contract.md
// §D7-VERIFY-CONTRACT-1).
//
// The 4 tuples are:
//  1. expected_class  — what EmissionClass did the channel route to?
//  2. deliverable_text — final text from the LLM (mandatory if DeliverableRequired)
//  3. evidence        — tool call IDs / evidence items (mandatory if EvidenceRequired)
//  4. source_uncertainty — calibrated_confidence computation
//
// The "burdens of proof" are allocated by EmissionClass per the
// burden-of-proof table (P1-AC-3).
type VerifyContract struct {
	// ExpectedClass is the EmissionClass the channel was supposed to
	// route to. Drives the burden-of-proof allocation.
	ExpectedClass contracts.EmissionClass

	// DeliverableRequired indicates whether the deliverable text is
	// mandatory. Set true for review/edit/test tasks; false for
	// pure-explore observe tasks.
	DeliverableRequired bool

	// DeliverableMinChars is the minimum deliverable text length.
	// Task-kind-dependent (review=20, edit=10, test=30, observe=10).
	DeliverableMinChars int

	// EvidenceRequired indicates whether the call must include
	// tool call evidence. Set true for Action and Probe; false for Fact.
	EvidenceRequired bool

	// MinEvidenceCount is the minimum number of evidence items.
	// Defaults to 1 for Action/Probe; 0 for Fact.
	MinEvidenceCount int

	// MinSourceQuality is the calibrated_confidence floor.
	// Below this → VerdictPartial. Default 0.5.
	MinSourceQuality float64

	// TaskKind is the inferred user task kind (review/edit/test/observe/refactor).
	TaskKind string
}

// NewVerifyContract constructs a VerifyContract with task-kind defaults
// per specs/verify-contract.md §D7-VERIFY-CONTRACT-1.
//
// The function takes a taskKind string and the expected EmissionClass,
// and fills in the burden-of-proof table:
//
//   Fact       → DeliverableRequired=true,  EvidenceRequired=false
//   Action     → DeliverableRequired=true,  EvidenceRequired=true,  MinEvidenceCount=1
//   Probe      → DeliverableRequired=true,  EvidenceRequired=true,  MinEvidenceCount=1
//   Experiment → DeliverableRequired=true,  EvidenceRequired=true,  MinEvidenceCount=1
//
// DSAFT: D7-S10-A50-T01. (Info #4 fix: explicit constructor to prevent
// the Go zero-value trap where MinSourceQuality=0 silently passes.)
func NewVerifyContract(taskKind string, expected contracts.EmissionClass) VerifyContract {
	return VerifyContract{
		ExpectedClass:       expected,
		DeliverableRequired: true,
		DeliverableMinChars: defaultMinCharsForTaskKind(taskKind),
		EvidenceRequired:    expected != contracts.EC_Fact,
		MinEvidenceCount:    1,
		MinSourceQuality:    0.5,
		TaskKind:            taskKind,
	}
}

// defaultMinCharsForTaskKind returns the minimum deliverable char count
// for a given task kind.
func defaultMinCharsForTaskKind(kind string) int {
	switch kind {
	case "review":
		return 20
	case "edit":
		return 10
	case "test":
		return 30
	case "refactor":
		return 40
	case "observe":
		return 10
	default:
		return 20 // safe default
	}
}

// Verdict is the output of the Verify audit.
type Verdict struct {
	Kind     VerdictKind
	Reason   string // canonical reason code (e.g. "deliverable_missing", "evidence_insufficient")
	ReasonZh string // human-readable Chinese explanation
	Meta     map[string]string
}

// Common reason codes (stable for span attributes and meta["verify_exit_reason"]).
const (
	ReasonDeliverableMissing  = "deliverable_missing"
	ReasonDeliverableTooShort = "deliverable_too_short"
	ReasonEvidenceInsufficient = "evidence_insufficient"
	ReasonSourceUncertaintyHigh = "source_uncertainty_high"
	ReasonStateChangeFailed    = "state_change_failed"
	ReasonExperimentInconclusive = "experiment_inconclusive"
	ReasonOK                   = "ok"
)

// VerifyInput is the runtime input to Verify.
type VerifyInput struct {
	// DeliverableText is the LLM's final text.
	DeliverableText string
	// Evidence is the list of tool call IDs (or evidence items).
	Evidence []string
	// SourceQualities is the per-tool SourceUncertainty.Value for each
	// tool call in Evidence. Used for calibrated_confidence.
	SourceQualities []float64
	// EmissionClassWeights is the per-call emission class weight
	// (EC_Fact=0.50, EC_Action=0.35, EC_Probe=0.20, EC_Experiment=0.10).
	EmissionClassWeights []float64
}

// CalibratedConfidence computes the calibrated confidence per the
// Codex Critical #6 fixed formula:
//
//	calibrated_confidence = Σ(source_uncertainty × emission_class_weight)
//	                        / Σ(emission_class_weight)
//
// Returns 0 if there are no samples.
func (v *VerifyInput) CalibratedConfidence() float64 {
	if len(v.SourceQualities) == 0 || len(v.SourceQualities) != len(v.EmissionClassWeights) {
		return 0
	}
	var num, denom float64
	for i, su := range v.SourceQualities {
		w := v.EmissionClassWeights[i]
		num += su * w
		denom += w
	}
	if denom == 0 {
		return 0
	}
	return num / denom
}

// ErrContractMismatch is returned by Verify when the input does not
// match the contract.
var ErrContractMismatch = errors.New("verify: contract mismatch")

// Verify audits the input against the contract and returns a Verdict.
// Returns the verdict (always non-nil) and an error only if the input
// is structurally invalid (e.g. mismatched slice lengths).
//
// DSAFT: D7-S10-A50-T01..T03 (4-tuple + burden of proof + reason透传).
func (c VerifyContract) Verify(input VerifyInput) (*Verdict, error) {
	// Structural check
	if len(input.SourceQualities) != len(input.EmissionClassWeights) {
		return nil, fmt.Errorf("%w: source_qualities/emission_class_weights length mismatch (%d vs %d)",
			ErrContractMismatch, len(input.SourceQualities), len(input.EmissionClassWeights))
	}

	meta := make(map[string]string)
	meta["task_kind"] = c.TaskKind
	meta["expected_class"] = c.ExpectedClass.String()

	// Check 1: deliverable_text presence + min chars
	if c.DeliverableRequired {
		if input.DeliverableText == "" {
			return &Verdict{
				Kind:     VerdictFail,
				Reason:   ReasonDeliverableMissing,
				ReasonZh: "deliverable text is empty (LLM did not produce output)",
				Meta:     meta,
			}, nil
		}
		if len([]rune(input.DeliverableText)) < c.DeliverableMinChars {
			return &Verdict{
				Kind:     VerdictFail,
				Reason:   ReasonDeliverableTooShort,
				ReasonZh: fmt.Sprintf("deliverable too short (%d < %d chars)",
					len([]rune(input.DeliverableText)), c.DeliverableMinChars),
				Meta: meta,
			}, nil
		}
	}

	// Check 2: evidence
	if c.EvidenceRequired {
		if len(input.Evidence) < c.MinEvidenceCount {
			return &Verdict{
				Kind:     VerdictFail,
				Reason:   ReasonEvidenceInsufficient,
				ReasonZh: fmt.Sprintf("evidence insufficient (%d < %d items)",
					len(input.Evidence), c.MinEvidenceCount),
				Meta: meta,
			}, nil
		}
	}

	// Check 3: calibrated_confidence
	cc := input.CalibratedConfidence()
	meta["calibrated_confidence"] = fmt.Sprintf("%.3f", cc)
	if cc < c.MinSourceQuality {
		return &Verdict{
			Kind:     VerdictPartial,
			Reason:   ReasonSourceUncertaintyHigh,
			ReasonZh: fmt.Sprintf("source uncertainty high (CC=%.3f < %.3f)", cc, c.MinSourceQuality),
			Meta:     meta,
		}, nil
	}

	// All checks pass
	meta["calibrated_confidence"] = fmt.Sprintf("%.3f", cc)
	return &Verdict{
		Kind:     VerdictPass,
		Reason:   ReasonOK,
		ReasonZh: "all 4-tuple checks pass",
		Meta:     meta,
	}, nil
}

// BurdenOfProofForClass returns the burden-of-proof rules for the
// given EmissionClass. Used by the ChannelRouter to allocate举证
// (P1-AC-3, demand §3.3).
//
// Returns the FAIL reason that applies for the class's primary check.
func BurdenOfProofForClass(ec contracts.EmissionClass) string {
	switch ec {
	case contracts.EC_Fact:
		return ReasonDeliverableMissing // deliverable text self-attests
	case contracts.EC_Action:
		return ReasonStateChangeFailed // state change evidence required
	case contracts.EC_Probe:
		return ReasonSourceUncertaintyHigh // source_quality required
	case contracts.EC_Experiment:
		return ReasonExperimentInconclusive // reproducibility required
	}
	return ReasonDeliverableMissing
}
