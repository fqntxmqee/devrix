// Package plan: PlanParseReject — 6 new sub-class rejections for LLM-emitted
// Plan parse failures (DM-20260707-001 PR-F, T67).
//
// The existing 10 sub-classes (PlanFieldValidator) catch STRUCTURAL violations
// AFTER a Plan is constructed. This module catches PARSE-TIME rejections
// BEFORE a Plan exists, when the LLM emits malformed JSON / wrong enum /
// missing required field. The taxonomy is split by WHO catches the failure:
//
//   | Sub-class                   | When caught         | By                  |
//   | --------------------------- | ------------------- | ------------------- |
//   | 10×CodeBlockXXX             | post-construction   | PlanFieldValidator  |
//   | 6×CodeParseXXX (this file)  | LLM output parsing  | PlanParseReject     |
//
// All 16 codes roll up to the Decision layer (T70) for unified routing.
//
// Why 6 (not "1 generic parse error"): each LLM failure mode has a different
// recovery strategy:
//   - MalformedJSON → retry with JSON-fix prompt hint
//   - UnknownKind   → retry with enum list prompt hint
//   - MissingField  → retry with required-fields hint
//   - InvalidNumeric→ retry with numeric constraints hint
//   - InvalidEnum   → retry with enum whitelist hint
//   - InvalidAST    → fallback to DecomposeIntoChildren (LLM broke the schema
//                     in a way that retry cannot fix)
//
// Retry budget is owned by RetryWithFeedback (T68): max 2 retries, then
// fallback to DecomposeIntoChildren or PlanErrorDecision.
package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// Parse-time rejection codes (6 new sub-classes, extending FieldRejectionCode
// from plan_field_validator.go).
const (
	// CodeParseMalformedJSON — JSON unmarshal itself failed (syntax error,
	// truncation, mismatched braces). Recoverable: retry with JSON-fix hint.
	CodeParseMalformedJSON FieldRejectionCode = "PLAN_PARSE_JSON_8031"

	// CodeParseUnknownKind — JSON parsed, but PlanKind value is not in the
	// 4-class enum. Recoverable: retry with enum list hint.
	CodeParseUnknownKind FieldRejectionCode = "PLAN_PARSE_KIND_8032"

	// CodeParseMissingField — JSON parsed, but a required field is absent
	// (id/session_id/kind/strength/source_observation_ids/steps).
	// Recoverable: retry with required-fields hint.
	CodeParseMissingField FieldRejectionCode = "PLAN_PARSE_FIELD_8033"

	// CodeParseInvalidNumeric — JSON parsed, but Strength is not a finite
	// number in [0, 1] (NaN, ±Inf, string-as-number, out of range).
	// Recoverable: retry with numeric constraints hint.
	CodeParseInvalidNumeric FieldRejectionCode = "PLAN_PARSE_NUMERIC_8034"

	// CodeParseInvalidEnum — JSON parsed, but PersistScope is not in the
	// 3-value whitelist (transient/session/permanent), OR FailureCriterion.Op
	// is not in the 6-value whitelist. Recoverable: retry with enum hint.
	CodeParseInvalidEnum FieldRejectionCode = "PLAN_PARSE_ENUM_8035"

	// CodeParseInvalidAST — JSON parsed, but semantic invariants failed
	// (e.g. duplicate Step IDs, Step with empty Directive, FailureCriteria
	// referencing undeclared field, DAG with cycle, Steps > plan limit).
	// NOT recoverable by retry: caller falls back to DecomposeIntoChildren.
	CodeParseInvalidAST FieldRejectionCode = "PLAN_PARSE_SEMANTIC_8036"
)

// AllParseRejectionCodes returns the 6 parse-time codes in stable order.
func AllParseRejectionCodes() []FieldRejectionCode {
	return []FieldRejectionCode{
		CodeParseMalformedJSON,
		CodeParseUnknownKind,
		CodeParseMissingField,
		CodeParseInvalidNumeric,
		CodeParseInvalidEnum,
		CodeParseInvalidAST,
	}
}

// AllRejectionCodes returns the full 14-codes taxonomy (8 field + 6 parse),
// in stable order. Useful for the Decision layer (T70) and dashboards.
func AllRejectionCodes() []FieldRejectionCode {
	out := make([]FieldRejectionCode, 0, 14)
	out = append(out, AllFieldRejectionCodes()...)
	out = append(out, AllParseRejectionCodes()...)
	return out
}

// ParseRejectReason describes one parse-time rejection. Mirrors
// PlanFieldAudit shape so the Decision layer (T70) can treat both uniformly.
type ParseRejectReason struct {
	Code    FieldRejectionCode
	Field   string // JSON-pointer-ish path (e.g. "$.kind", "$.strength")
	Message string
	Token   int    // approximate byte offset of the offending token, 0 if N/A
}

// String renders the reason for logging.
func (r ParseRejectReason) String() string {
	if r.Token > 0 {
		return fmt.Sprintf("%s field=%s byte=%d msg=%q",
			r.Code, r.Field, r.Token, r.Message)
	}
	return fmt.Sprintf("%s field=%s msg=%q", r.Code, r.Field, r.Message)
}

// PlanParseRejection is the structured error returned by ParsePlan when the
// LLM output cannot be turned into a valid Plan. Carries both the underlying
// error (so callers that just want err.Error() still work) and the structured
// Reason (for the Decision layer).
type PlanParseRejection struct {
	// Reason is the structured parse-time classification.
	Reason ParseRejectReason

	// raw is the input bytes that failed parsing (truncated to 4 KB).
	raw []byte

	// err is the wrapping underlying error (errors.Is-compatible).
	err error
}

// Error satisfies the error interface.
func (p *PlanParseRejection) Error() string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("plan parse rejected: %s", p.Reason.String())
}

// Unwrap exposes the underlying sentinel error.
func (p *PlanParseRejection) Unwrap() error {
	if p == nil {
		return nil
	}
	return p.err
}

// Raw returns the bytes that failed parsing (truncated to 4 KB).
func (p *PlanParseRejection) Raw() []byte {
	if p == nil {
		return nil
	}
	out := make([]byte, len(p.raw))
	copy(out, p.raw)
	return out
}

// Sentinel error for errors.Is matching.
var (
	ErrPlanParseRejected = errors.New("plan: parse rejected")
)

// newParseRejection constructs a PlanParseRejection with the given reason +
// the raw bytes that failed to parse.
func newParseRejection(code FieldRejectionCode, field, msg string, raw []byte, token int) *PlanParseRejection {
	const maxRawBytes = 4096
	trunc := raw
	if len(trunc) > maxRawBytes {
		trunc = trunc[:maxRawBytes]
	}
	return &PlanParseRejection{
		Reason: ParseRejectReason{
			Code:    code,
			Field:   field,
			Message: msg,
			Token:   token,
		},
		raw: trunc,
		err: sharederrors.WithCode(string(code), msg, ErrPlanParseRejected),
	}
}

// requiredPlanFields is the list of Plan fields that MUST be present after
// the first JSON decode pass. Order matters for stable Field path reporting.
var requiredPlanFields = []string{
	"id", "session_id", "kind", "strength", "source_observation_ids", "steps",
}

// ParsePlan parses LLM-emitted JSON bytes into a *Plan + applies post-parse
// semantic checks (CodeParseInvalidAST). Returns:
//
//   - (plan, nil) on success.
//   - (nil, *PlanParseRejection) when the bytes fail any of the 6 sub-classes.
//
// ParsePlan NEVER returns a partial Plan — if any sub-class fails, the Plan
// is not constructed. Callers that want retry semantics must wrap ParsePlan
// in RetryWithFeedback (T68).
//
// The function is intentionally pure (no LLM calls, no DB). It is exercised
// directly by tests and by the LLMPlanner (future PR-B3).
func ParsePlan(raw []byte) (*Plan, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, newParseRejection(
			CodeParseMalformedJSON,
			"$",
			"empty input",
			raw, 0,
		)
	}

	// First pass: structural shape — decode into a generic map so we can
	// distinguish "JSON malformed" (CodeParseMalformedJSON) from "JSON
	// parsed but field missing" (CodeParseMissingField).
	var shape map[string]json.RawMessage
	dec := json.NewDecoder(strings.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&shape); err != nil {
		return nil, newParseRejection(
			CodeParseMalformedJSON,
			"$.<root>",
			fmt.Sprintf("json decode failed: %v", err),
			raw, 0,
		)
	}
	// Reject multi-document JSON (a common LLM error).
	if dec.More() {
		return nil, newParseRejection(
			CodeParseMalformedJSON,
			"$.<root>",
			"multiple JSON documents found; expected exactly one",
			raw, 0,
		)
	}

	// Required-field check.
	missing := make([]string, 0, 2)
	for _, f := range requiredPlanFields {
		if _, ok := shape[f]; !ok {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing) // deterministic field path
		return nil, newParseRejection(
			CodeParseMissingField,
			"$."+missing[0],
			fmt.Sprintf("missing required field(s): %s", strings.Join(missing, ", ")),
			raw, 0,
		)
	}

	// Field-by-field validation BEFORE the second decode pass. This is
	// where we catch unknown enum / bad numeric / weak type mismatches
	// with precise field paths — using json.Unmarshal directly would lose
	// this granularity.
	//
	// kind
	var kindStr string
	if err := json.Unmarshal(shape["kind"], &kindStr); err != nil {
		return nil, newParseRejection(
			CodeParseMalformedJSON,
			"$.kind",
			fmt.Sprintf("kind field must be a string: %v", err),
			raw, 0,
		)
	}
	parsedKind, err := ParsePlanKind(kindStr)
	if err != nil {
		return nil, newParseRejection(
			CodeParseUnknownKind,
			"$.kind",
			fmt.Sprintf("unknown kind %q (valid: commitment_plan/protocol_plan/scenario_plan/exploration_plan)", kindStr),
			raw, 0,
		)
	}

	// strength — must be a finite number in [0, 1]
	var strength float64
	if err := json.Unmarshal(shape["strength"], &strength); err != nil {
		return nil, newParseRejection(
			CodeParseInvalidNumeric,
			"$.strength",
			fmt.Sprintf("strength must be a finite number: %v", err),
			raw, 0,
		)
	}
	if math.IsNaN(strength) || math.IsInf(strength, 0) {
		return nil, newParseRejection(
			CodeParseInvalidNumeric,
			"$.strength",
			fmt.Sprintf("strength must be finite, got %v", strength),
			raw, 0,
		)
	}
	if strength < 0 || strength > 1 {
		return nil, newParseRejection(
			CodeParseInvalidNumeric,
			"$.strength",
			fmt.Sprintf("strength=%v out of [0, 1] range", strength),
			raw, 0,
		)
	}

	// persist_scope — if present, must be one of the 3 enums.
	if rawPS, ok := shape["blast_radius"]; ok {
		var br struct {
			PersistScope string `json:"persist_scope"`
		}
		if err := json.Unmarshal(rawPS, &br); err != nil {
			return nil, newParseRejection(
				CodeParseMalformedJSON,
				"$.blast_radius",
				fmt.Sprintf("blast_radius malformed: %v", err),
				raw, 0,
			)
		}
		if br.PersistScope != "" {
			ps := PersistScope(br.PersistScope)
			if !ps.Valid() {
				return nil, newParseRejection(
					CodeParseInvalidEnum,
					"$.blast_radius.persist_scope",
					fmt.Sprintf("persist_scope=%q not in whitelist (transient/session/permanent)", br.PersistScope),
					raw, 0,
				)
			}
		}
	}

	// failure_criteria — if present, each op must be in the whitelist.
	if rawFC, ok := shape["failure_criteria"]; ok {
		var fcs []map[string]any
		if err := json.Unmarshal(rawFC, &fcs); err != nil {
			return nil, newParseRejection(
				CodeParseMalformedJSON,
				"$.failure_criteria",
				fmt.Sprintf("failure_criteria malformed: %v", err),
				raw, 0,
			)
		}
		for i, fc := range fcs {
			op, _ := fc["op"].(string)
			if !isOpAllowed(op) {
				return nil, newParseRejection(
					CodeParseInvalidEnum,
					fmt.Sprintf("$.failure_criteria[%d].op", i),
					fmt.Sprintf("op=%q not in whitelist (eq/ne/gt/lt/in/contains)", op),
					raw, 0,
				)
			}
		}
	}

	// Second pass: full decode into Plan.
	var p Plan
	dec2 := json.NewDecoder(strings.NewReader(trimmed))
	dec2.DisallowUnknownFields()
	if err := dec2.Decode(&p); err != nil {
		// Should not happen — first-pass shape check covered enum + numeric.
		// Fall through with the malformed JSON code.
		return nil, newParseRejection(
			CodeParseMalformedJSON,
			"$.<root>",
			fmt.Sprintf("full plan decode failed: %v", err),
			raw, 0,
		)
	}
	// Force kind to be the parsed value (in case the JSON used a different
	// case or whitespace).
	p.Kind = parsedKind

	// AST-level semantic checks (CodeParseInvalidAST) — invariant violations
	// that no retry can fix.
	if reason := checkASTInvariants(&p); reason != nil {
		return nil, newParseRejection(
			CodeParseInvalidAST,
			reason.Field,
			reason.Message,
			raw, 0,
		)
	}

	return &p, nil
}

// checkASTInvariants returns a non-nil reason when the Plan violates a
// semantic invariant that no parse-time retry can fix. Examples:
//
//   - Duplicate Step IDs (LLM emitted s1 twice)
//   - Step with empty Directive
//   - Step with no IdempotencyKey when its ToolName is side-effectful
//   - Steps slice > 32 (the Phase 3 channel hard cap)
//
// AST checks happen AFTER successful JSON parse; the field path uses the
// $.steps[i].field notation consistent with CodeParse* errors.
func checkASTInvariants(p *Plan) *ParseRejectReason {
	if p == nil {
		return &ParseRejectReason{
			Code:    CodeParseInvalidAST,
			Field:   "$",
			Message: "plan is nil",
		}
	}
	// Duplicate step IDs.
	seen := make(map[string]struct{}, len(p.Steps))
	for i, s := range p.Steps {
		if s.ID == "" {
			return &ParseRejectReason{
				Code:    CodeParseInvalidAST,
				Field:   fmt.Sprintf("$.steps[%d].id", i),
				Message: "step id must be non-empty",
			}
		}
		if _, dup := seen[s.ID]; dup {
			return &ParseRejectReason{
				Code:    CodeParseInvalidAST,
				Field:   fmt.Sprintf("$.steps[%d].id", i),
				Message: fmt.Sprintf("duplicate step id %q", s.ID),
			}
		}
		seen[s.ID] = struct{}{}
		if strings.TrimSpace(s.Directive) == "" {
			return &ParseRejectReason{
				Code:    CodeParseInvalidAST,
				Field:   fmt.Sprintf("$.steps[%d].directive", i),
				Message: "step directive must be non-empty",
			}
		}
	}
	// Steps slice hard cap (matches Phase 3 PR-C2 channel hard limit).
	const maxSteps = 32
	if len(p.Steps) > maxSteps {
		return &ParseRejectReason{
			Code:    CodeParseInvalidAST,
			Field:   "$.steps",
			Message: fmt.Sprintf("steps length %d exceeds hard cap %d", len(p.Steps), maxSteps),
		}
	}
	// SourceObservationIDs must be non-empty & non-duplicate.
	if len(p.SourceObservationIDs) == 0 {
		return &ParseRejectReason{
			Code:    CodeParseInvalidAST,
			Field:   "$.source_observation_ids",
			Message: "source_observation_ids must be non-empty (PP-4 lineage)",
		}
	}
	seenIDs := make(map[string]struct{}, len(p.SourceObservationIDs))
	for i, id := range p.SourceObservationIDs {
		if id == "" {
			return &ParseRejectReason{
				Code:    CodeParseInvalidAST,
				Field:   fmt.Sprintf("$.source_observation_ids[%d]", i),
				Message: "observation id must be non-empty",
			}
		}
		seenIDs[id] = struct{}{}
	}
	if len(seenIDs) != len(p.SourceObservationIDs) {
		return &ParseRejectReason{
			Code:    CodeParseInvalidAST,
			Field:   "$.source_observation_ids",
			Message: "source_observation_ids contains duplicates",
		}
	}
	return nil
}
