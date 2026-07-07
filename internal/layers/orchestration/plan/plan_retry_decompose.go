// Package plan: RetryWithFeedback + DecomposeIntoChildren + PlanErrorDecision
// (DM-20260707-001 PR-F, T68+T69).
//
// When the LLM emits a malformed Plan, the caller has 3 recovery options:
//
//	1. Retry       — feed the rejection hint back to the LLM (≤2 attempts)
//	2. Decompose   — split the original intent into child sub-intents
//	3. Abort       — surface the rejection to Learn as a plan_error (no further retry)
//
// The Decision layer (T70) selects which path based on the rejection's
// FieldRejectionCode. RetryWithFeedback encapsulates the retry-then-decompose
// sequence; PlanErrorDecision encapsulates the abort path that skips Learn.
//
// Why ≤2 retries (not more): each retry is another LLM round-trip (~3-5s +
// $0.02 in typical models). Three retries in a row often indicates a model
// being asked out-of-distribution; doing one more is cheaper than letting the
// failure cascade into degraded execution.
package plan

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
)

// DefaultMaxRetries is the hard cap on LLM retry attempts after a parse
// rejection. After this many failures, RetryWithFeedback falls back to
// DecomposeIntoChildren, which has its own 1-attempt budget.
const DefaultMaxRetries = 2

// FeedbackHint is the structured prompt-hint returned by FeedbackForRejection.
// Callers feed this back to the LLM as part of the retry request so the model
// knows exactly what to fix.
type FeedbackHint struct {
	// Code mirrors ParseRejectReason.Code so callers can switch on it.
	Code FieldRejectionCode

	// FieldPath is the JSON-pointer-ish path to the offending field
	// (e.g. "$.kind" / "$.strength" / "$.failure_criteria[0].op").
	FieldPath string

	// Message is the human-readable hint (e.g. "kind must be one of
	// commitment_plan/protocol_plan/scenario_plan/exploration_plan").
	Message string

	// Retryable reports whether this rejection is recoverable by retry.
	// CodeParseInvalidAST is the canonical non-retryable case.
	Retryable bool

	// Suggestion is an LLM-facing hint phrased as "next time, ...". Empty
	// when Retryable=false (the LLM cannot fix the issue).
	Suggestion string
}

// String renders the feedback hint for logging.
func (f FeedbackHint) String() string {
	if f.Suggestion == "" {
		return fmt.Sprintf("%s retryable=%v field=%s msg=%q",
			f.Code, f.Retryable, f.FieldPath, f.Message)
	}
	return fmt.Sprintf("%s retryable=%v field=%s msg=%q suggestion=%q",
		f.Code, f.Retryable, f.FieldPath, f.Message, f.Suggestion)
}

// FeedbackForRejection converts a PlanParseRejection into a FeedbackHint.
// Used by the retry loop + by the Decision layer to drive the routing table.
//
// Retry classification:
//
//	JSON / missing-field / unknown-kind / numeric / enum → RETRYABLE
//	AST invariants (duplicate IDs, >32 steps, etc.)    → NOT RETRYABLE
func FeedbackForRejection(rej *PlanParseRejection) FeedbackHint {
	if rej == nil {
		return FeedbackHint{
			Code:    CodeParseMalformedJSON,
			Message: "nil rejection",
		}
	}
	switch rej.Reason.Code {
	case CodeParseMalformedJSON:
		return FeedbackHint{
			Code:       rej.Reason.Code,
			FieldPath:  rej.Reason.Field,
			Message:    rej.Reason.Message,
			Retryable:  true,
			Suggestion: "emit a single well-formed JSON object (no trailing commas, balanced braces, no extra whitespace-only lines)",
		}
	case CodeParseUnknownKind:
		return FeedbackHint{
			Code:       rej.Reason.Code,
			FieldPath:  rej.Reason.Field,
			Message:    rej.Reason.Message,
			Retryable:  true,
			Suggestion: "next time, set kind to one of: commitment_plan | protocol_plan | scenario_plan | exploration_plan",
		}
	case CodeParseMissingField:
		return FeedbackHint{
			Code:       rej.Reason.Code,
			FieldPath:  rej.Reason.Field,
			Message:    rej.Reason.Message,
			Retryable:  true,
			Suggestion: "next time, include every required field: id, session_id, kind, strength, source_observation_ids, steps",
		}
	case CodeParseInvalidNumeric:
		return FeedbackHint{
			Code:       rej.Reason.Code,
			FieldPath:  rej.Reason.Field,
			Message:    rej.Reason.Message,
			Retryable:  true,
			Suggestion: "next time, set strength to a finite number in the closed range [0, 1] (e.g. 0.85)",
		}
	case CodeParseInvalidEnum:
		return FeedbackHint{
			Code:       rej.Reason.Code,
			FieldPath:  rej.Reason.Field,
			Message:    rej.Reason.Message,
			Retryable:  true,
			Suggestion: "next time, use only the whitelisted enum values (persist_scope: transient | session | permanent; op: eq | ne | gt | lt | in | contains)",
		}
	case CodeParseInvalidAST:
		return FeedbackHint{
			Code:       rej.Reason.Code,
			FieldPath:  rej.Reason.Field,
			Message:    rej.Reason.Message,
			Retryable:  false,
			Suggestion: "",
		}
	default:
		return FeedbackHint{
			Code:       rej.Reason.Code,
			FieldPath:  rej.Reason.Field,
			Message:    rej.Reason.Message,
			Retryable:  true,
			Suggestion: "review the JSON structure and constraints",
		}
	}
}

// Regenerator is the caller-supplied callback that produces a new LLM attempt
// after a parse rejection. The hint carries the rejection classification +
// suggested fix; the generator returns the new JSON bytes (or an error if
// the LLM call itself failed — that's NOT a parse rejection).
//
// Retries counter is 0-indexed: attempt 0 is the first regeneration after the
// initial failure.
type Regenerator func(hint FeedbackHint, attempt int) ([]byte, error)

// DecomposeHook is the optional fallback called when retry exhaustion is
// reached on a non-retryable or consistently failing rejection. The contract
// is "given the original prompt + a structured parse-rejection history,
// produce a different shaped Plan (typically a parent-rollup plan wrapping
// child sub-plans) OR return ErrDecomposeFailed to signal the next tier".
//
// DecomposeHook is optional. When nil, the retry-exhaustion path returns
// PlanErrorDecision directly.
type DecomposeHook func(rejections []*PlanParseRejection) (*Plan, error)

// ErrDecomposeFailed signals DecomposeHook could not produce a fallback Plan.
var ErrDecomposeFailed = errors.New("plan: decompose fallback failed")

// RetryResult is what RetryWithFeedback returns. TotalAttempts counts
// EVERY parse attempt (1 = first attempt succeeded, ≥2 = at least one retry).
type RetryResult struct {
	// Plan is the dispatch-ready result. Nil only when Err != nil and
	// the caller chose to escalate to abort instead of decompose.
	Plan *Plan

	// TotalAttempts counts how many times ParsePlan was called.
	TotalAttempts int

	// AttemptsDetail is a per-attempt trace for audit/log.
	AttemptsDetail []AttemptRecord

	// FinalRejection is the last rejection (nil on success).
	FinalRejection *PlanParseRejection

	// Escalated reports whether the function escalated to DecomposeHook
	// (true) vs returning a Plan (false).
	Escalated bool

	// Err is set only when both retry AND decompose failed AND the caller
	// asked for an error return rather than PlanErrorDecision.
	Err error
}

// AttemptRecord captures one retry attempt for audit/log.
type AttemptRecord struct {
	Attempt    int
	Hint       FeedbackHint
	Rejected   bool
	Rejection  *PlanParseRejection
	BytesIn    int
	BytesOut   int
	ElapsedMS  int64 // populated when wrapper measures; else 0
}

// RetryWithFeedback runs ParsePlan → on rejection, ask Regenerator for a new
// attempt, retry up to maxRetries times. After maxRetries is exhausted:
//
//   - If DecomposeHook is non-nil, invoke it once.
//   - Otherwise, return a RetryResult with Plan=nil + FinalRejection set +
//     Escalated=true (caller decides next action).
//
// maxRetries must be ≥ 1; 0 is treated as "no retry, return the initial result".
// Negative values are normalized to 0.
func RetryWithFeedback(
	initial []byte,
	maxRetries int,
	regenerator Regenerator,
	decompose DecomposeHook,
) RetryResult {
	if maxRetries < 0 {
		maxRetries = 0
	}

	result := RetryResult{}

	// Attempt 0 — initial parse.
	result.TotalAttempts++
	att := AttemptRecord{Attempt: 0, BytesIn: len(initial)}
	plan, parseErr := ParsePlan(initial)
	rej := toParseRejection(parseErr)
	if rej != nil {
		att.Rejected = true
		att.Rejection = rej
		att.BytesOut = 0
		att.Hint = FeedbackForRejection(rej)
		result.AttemptsDetail = append(result.AttemptsDetail, att)
		result.FinalRejection = rej

		// Try up to maxRetries regenerations.
		for attempt := 1; attempt <= maxRetries; attempt++ {
			if regenerator == nil {
				break
			}
			hint := FeedbackForRejection(rej)
			if !hint.Retryable {
				break
			}
			attBytes, regErr := regenerator(hint, attempt)
			attRec := AttemptRecord{
				Attempt: attempt, Hint: hint, BytesIn: len(initial),
			}
			if regErr != nil {
				// Regenerator itself failed (e.g. LLM 5xx). Treat as a
				// non-retry termination and fall back.
				attRec.BytesOut = 0
				result.AttemptsDetail = append(result.AttemptsDetail, attRec)
				result.FinalRejection = &PlanParseRejection{
					Reason: ParseRejectReason{
						Code:    CodeParseMalformedJSON,
						Field:   "$.<root>",
						Message: fmt.Sprintf("regenerator failed at attempt %d: %v", attempt, regErr),
					},
				}
				break
			}
			attRec.BytesOut = len(attBytes)
			plan, parseErr2 := ParsePlan(attBytes)
			rej = toParseRejection(parseErr2)
			result.TotalAttempts++
			if rej == nil {
				attRec.Rejected = false
				result.AttemptsDetail = append(result.AttemptsDetail, attRec)
				result.Plan = plan
				return result
			}
			attRec.Rejected = true
			attRec.Rejection = rej
			result.AttemptsDetail = append(result.AttemptsDetail, attRec)
			result.FinalRejection = rej
		}
	} else {
		// Success on attempt 0.
		att.Rejected = false
		att.BytesOut = len(initial)
		result.AttemptsDetail = append(result.AttemptsDetail, att)
		result.Plan = plan
		return result
	}

	// Retry exhaustion or non-retryable hint — try decompose hook.
	if decompose != nil {
		// Collect all distinct rejections for the decompose context.
		rejections := make([]*PlanParseRejection, 0, len(result.AttemptsDetail))
		for _, ar := range result.AttemptsDetail {
			if ar.Rejection != nil {
				rejections = append(rejections, ar.Rejection)
			}
		}
		fallback, decErr := decompose(rejections)
		if decErr == nil && fallback != nil {
			result.Plan = fallback
			result.Escalated = true
			return result
		}
		result.Err = fmt.Errorf("retry exhausted and decompose failed: %w", decErr)
	} else {
		result.Escalated = true
	}
	return result
}

// DecomposeIntoChildren is the canonical DecomposeHook for Phase 4 Verify-
// promoted users (e.g. when retry cannot produce a Plan but the intent is
// recoverable by splitting into smaller sub-intents).
//
// Placeholder contract: when invoked, emit a parent-rollup Plan whose Steps
// are the *PlanParseRejection.Reason.Message strings treated as directives.
// The actual implementation in Phase 6+ uses the LLM Decomposer; this stub
// captures the determinism requirement for testing.
//
// Returns ErrDecomposeFailed when the input rejection list is empty (no
// signal to decompose from).
func DecomposeIntoChildren(rejections []*PlanParseRejection) (*Plan, error) {
	if len(rejections) == 0 {
		return nil, ErrDecomposeFailed
	}
	seen := make(map[FieldRejectionCode]struct{}, len(rejections))
	steps := make([]Step, 0, len(rejections))
	for i, r := range rejections {
		if r == nil {
			continue
		}
		if _, dup := seen[r.Reason.Code]; dup {
			continue
		}
		seen[r.Reason.Code] = struct{}{}
		steps = append(steps, Step{
			ID:        fmt.Sprintf("decomp_%d_%s", i, codeSlug(r.Reason.Code)),
			Directive: "decompose: " + strings.TrimSpace(r.Reason.Message),
		})
	}
	if len(steps) == 0 {
		return nil, ErrDecomposeFailed
	}
	// Sort for determinism.
	sort.SliceStable(steps, func(i, j int) bool { return steps[i].ID < steps[j].ID })
	return &Plan{
		ID:                   "plan_decompose_" + uniqueSuffix(),
		Kind:                 ScenarioPlan, // scenario = read-only probe, safest fallback
		Strength:             0.5,          // decompose inherits parent's weakest signal
		Steps:                steps,
		FailureCriteria:      []FailureCriterion{{Field: "exit_code", Op: "eq", Value: 0}},
		BlastRadius:          BlastRadius{FileCount: 0, APICallCount: 0, TokenCost: 0, PersistScope: PersistTransient},
		SourceObservationIDs: []string{"decompose_root"},
	}, nil
}

// codeSlug converts a FieldRejectionCode into a filesystem-safe string for
// use as a Step ID suffix.
func codeSlug(c FieldRejectionCode) string {
	s := string(c)
	s = strings.ReplaceAll(s, "_", "-")
	return s
}

// uniqueSuffix returns a monotonic counter so each Plan ID is unique even
// within the same test run. Uses atomic to be safe under -race.
var decomposeCounter atomic.Uint64

func uniqueSuffix() string {
	return fmt.Sprintf("%d", decomposeCounter.Add(1))
}

// PlanErrorDecision is the structured abort signal emitted when retry +
// decompose both fail. Carries the parsed rejection so the Learn layer (T59)
// can attribute the failure to a specific Phase 2 step (Plan) without
// double-firing BayesianUpdate.
//
// The Decision layer (T70) emits this when the 3-tier retry budget is
// exhausted; Learn MUST skip BayesianUpdate when IsPlanError() returns true.
type PlanErrorDecision struct {
	// SessionID identifies the originating orchestration session.
	SessionID string

	// Rejections captures the rejection history (≥1 element).
	Rejections []*PlanParseRejection

	// StepID is the parent StepID in the WaveScheduler (for fallback
	// audit attribution).
	StepID string

	// EmitAt is when the decision was emitted (for trace correlation).
	EmitAt int64
}

// IsPlanError is the duck-type marker the Learn layer uses to bypass
// BayesianUpdate. Returns true for any PlanErrorDecision value.
func (d *PlanErrorDecision) IsPlanError() bool { return d != nil }

// LastRejection returns the most recent rejection (or nil when empty).
func (d *PlanErrorDecision) LastRejection() *PlanParseRejection {
	if d == nil || len(d.Rejections) == 0 {
		return nil
	}
	return d.Rejections[len(d.Rejections)-1]
}

// NewPlanErrorDecision constructs a PlanErrorDecision from a RetryResult.
// Returns nil when the RetryResult has no rejection (caller error — abort
// should never be constructed on success).
func NewPlanErrorDecision(sessionID string, retry RetryResult) *PlanErrorDecision {
	if retry.FinalRejection == nil {
		return nil
	}
	return &PlanErrorDecision{
		SessionID: sessionID,
		StepID:     "", // populated by orchestrator with the Wave stepID
		Rejections: collectAllRejections(retry),
	}
}

// collectAllRejections returns a deduplicated list of rejections preserving
// insertion order. Used by NewPlanErrorDecision to build audit-friendly
// context for the Learn layer.
func collectAllRejections(retry RetryResult) []*PlanParseRejection {
	seen := make(map[string]struct{}, len(retry.AttemptsDetail))
	out := make([]*PlanParseRejection, 0, len(retry.AttemptsDetail))
	for _, ar := range retry.AttemptsDetail {
		if ar.Rejection == nil {
			continue
		}
		key := string(ar.Rejection.Reason.Code)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, ar.Rejection)
	}
	return out
}

// toParseRejection adapts ParsePlan's error return (which may be nil or any
// error matching errors.As(*PlanParseRejection)) into the typed pointer the
// retry loop needs. Returns nil when the error is nil or not a
// PlanParseRejection (defensive — ParsePlan always returns PlanParseRejection
// today, but this helper makes the loop robust to future additions).
func toParseRejection(err error) *PlanParseRejection {
	if err == nil {
		return nil
	}
	var rej *PlanParseRejection
	if errors.As(err, &rej) {
		return rej
	}
	return nil
}
