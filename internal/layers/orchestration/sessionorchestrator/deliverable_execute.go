package sessionorchestrator

import (
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// uncertaintyQuestionMaxLen caps the per-question string length embedded in
// observation_summary. Without this, an LLM that emits a 1kB uncertainty
// question would balloon the Plan LLM request frame. 120 chars matches the
// default OpenSpec convention for inline question previews.
const uncertaintyQuestionMaxLen = 120

// AppendDeliverableContractExecuteHint adds contract tag + machine acceptance criteria.
func AppendDeliverableContractExecuteHint(directive string, contract workmodel.DeliverableContract) string {
	tag := workmodel.DeliverableContractTag(contract)
	if tag == "" {
		return directive
	}
	if strings.Contains(directive, tag) {
		return directive
	}
	crit := workmodel.AcceptanceCriteriaForContract(contract)
	finalHint := workmodel.DeliverableFinalAnswerHint(contract)
	var hint strings.Builder
	hint.WriteString(tag)
	if crit != "" {
		hint.WriteString("\n")
		hint.WriteString(crit)
	}
	if finalHint != "" {
		hint.WriteString("\n")
		hint.WriteString(finalHint)
	}
	return strings.TrimSpace(directive) + "\n\n" + hint.String()
}

// SynthesisTurnExecuteHint instructs the model on the tool-free final iteration.
func SynthesisTurnExecuteHint(contract workmodel.DeliverableContract) string {
	if contract.Normalized().Structure != workmodel.DeliverableStructureFindingsJSON {
		return ""
	}
	return "Final iteration: tools are disabled. Use evidence from prior tool results only. " +
		"Respond with ONLY the findings_json deliverable; do not explore new paths or OpenSpec docs."
}

// RollupSynthesisTurnExecuteHint instructs the model on rollup synthesis rounds
// (ModeRollupSynth / NeedsRollup) where tools are disabled but the model may
// still emit phantom tool-call XML on the text channel.
func RollupSynthesisTurnExecuteHint() string {
	return "Rollup synthesis: tools are disabled. Write a prose summary (≥500 runes) " +
		"with P0 and P1 findings citing file:line from evidence already gathered. " +
		"Do not emit tool_call XML, invoke tags, or start new reads."
}

// PriorDeliverableRetryHint builds machine retry context for inline rounds.
// Omits spawn rationale and full artifact prose to avoid scope drift (DM-20260703-001).
func PriorDeliverableRetryHint(item *workmodel.WorkItem, contract workmodel.DeliverableContract) string {
	if item == nil || item.LastRound == nil {
		return ""
	}
	lr := item.LastRound
	if lr.DeliverableStatus != workmodel.DeliverableStatusIncomplete {
		return ""
	}
	var parts []string
	if sc := item.ScopeContract; sc != nil && len(sc.InScope) > 0 {
		parts = append(parts, "ScopeIn: "+strings.Join(sc.InScope, ", "))
		parts = append(parts, "Keep review target unchanged; do not switch to openspec/ or other directories.")
	}
	summary := strings.TrimSpace(lr.ArtifactSummary)
	if summary != "" && contract.ContractApplicable() {
		got := workmodel.VerifyDeliverableContract(contract, summary, "max_iters")
		if got.Reason != "" {
			parts = append(parts, "PriorDeliverableFailure: "+got.Reason)
		}
	}
	if workmodel.RequiresSynthesisTurn(contract) {
		parts = append(parts, "Stop exploring; synthesize findings_json from files already read.")
	}
	return strings.Join(parts, "\n")
}

// machineSpawnFeedback extracts non-prose spawn lines safe for Execute retry
// (strategic rejection, scope gate). Full SpawnRationale is omitted to avoid
// scope drift from stale path lists.
func machineSpawnFeedback(item *workmodel.WorkItem) string {
	if item == nil || item.LastRound == nil {
		return ""
	}
	var parts []string
	for _, line := range strings.Split(item.LastRound.SpawnRationale, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "strategic_plan_rejected") || strings.HasPrefix(line, "scope:") {
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, "\n")
}

// AppendDeliverableExecuteHint adds deliverable contract tag for legacy schema callers.
func AppendDeliverableExecuteHint(directive string, schema workmodel.DeliverableSchema) string {
	return AppendDeliverableContractExecuteHint(directive, workmodel.ExpandLegacySchemaToContract(schema))
}

// uncertaintyReportSummary builds the Plan-side observation_summary from the
// Observe-phase UncertaintyReport.
//
// History:
//   - DM-20260706-007: serialized ObsUncertainty/ObsDeviation inline so Plan
//     could see high-strength questions and deviations instead of guessing.
//   - DM-20260706-009: added ObsFact branch. Previously ObsFact.statement was
//     dropped, which meant a high-confidence fact (e.g. "1+1=2") would only
//     show up as `intent=fast` in the Plan frame. Plan LLM had no way to know
//     the question was already answered, so it would decompose into 2 child
//     specs and route through Execute+bash tool, blowing up trivial Q&A.
//
// We scan `report.Observations` (NOT report.Anomalies) because LLM-driven
// ObsUncertainty entries are CatBusiness and live in BusinessObservations —
// not in Anomalies. Filtering by Kind + a minimum strength floor avoids
// flooding the Plan frame with weak signals.
//
// Format (semicolon-joined, identical to the prior contract so the Plan
// prompt parser stays unchanged):
//   intent=<kind>; fact=<truncated statement>; q=<truncated question>; dev=<statement>; ...
//
// intent is always emitted first when present. Empty input → empty string
// (preserves the prior "all blank" fast path).
func uncertaintyReportSummary(report orchtypes.UncertaintyReport, intentKind string) string {
	var parts []string
	if intentKind != "" {
		parts = append(parts, "intent="+intentKind)
	}
	for _, o := range report.Observations {
		switch o.Kind {
		case orchtypes.ObsFact:
			// DM-20260706-009: high-strength facts MUST reach the Plan LLM
			// verbatim so it knows the question is already answered (no need
			// to decompose + execute + bash echo). We only surface CatBusiness
			// facts; system-level facts (e.g. "D7 bootstrap active") are
			// routing metadata the Plan shouldn't branch on.
			if o.Category != orchtypes.CatBusiness {
				continue
			}
			if s := extractObservationStatement(o); s != "" {
				parts = append(parts, "fact="+truncateForSummary(s, uncertaintyQuestionMaxLen))
			}
		case orchtypes.ObsUncertainty:
			// Skip low-strength uncertainty noise; only surface questions
			// that meaningfully change the Plan decision (matches the
			// obsUncertaintyAnomalyThreshold used by Verify-side anomaly
			// detection).
			if o.Strength < 0.7 {
				continue
			}
			if q := extractObservationQuestion(o); q != "" {
				parts = append(parts, "q="+truncateForSummary(q, uncertaintyQuestionMaxLen))
			}
		case orchtypes.ObsDeviation:
			if s := extractObservationStatement(o); s != "" {
				parts = append(parts, "dev="+truncateForSummary(s, uncertaintyQuestionMaxLen))
			}
		}
	}
	return strings.Join(parts, "; ")
}

// hasHighStrengthFact reports whether the report carries a CatBusiness
// ObsFact with strength ≥ threshold. Used by the Plan single-mode gate to
// detect "question already answered" cases (DM-20260706-009) so trivial Q&A
// like "1+1=几?" doesn't get force-decomposed.
func hasHighStrengthFact(report orchtypes.UncertaintyReport, threshold float64) bool {
	for _, o := range report.Observations {
		if o.Kind == orchtypes.ObsFact && o.Category == orchtypes.CatBusiness && o.Strength >= threshold {
			return true
		}
	}
	return false
}

// extractObservationQuestion reads UncertaintyPayload.Question regardless
// of the concrete Payload type. Kept package-private because only the
// summary helper needs it.
func extractObservationQuestion(o orchtypes.Observation) string {
	p, ok := o.Payload.(orchtypes.UncertaintyPayload)
	if !ok {
		return ""
	}
	return strings.TrimSpace(p.Question)
}

// extractObservationStatement reads FactPayload.Statement / SignalPayload.Name
// for ObsDeviation rows. Deviation rows are typically SignalPayload (a
// metric-delta tag) so we fall back to the signal name when statement is
// unavailable.
func extractObservationStatement(o orchtypes.Observation) string {
	switch p := o.Payload.(type) {
	case orchtypes.FactPayload:
		return strings.TrimSpace(p.Statement)
	case orchtypes.SignalPayload:
		if p.Name == "" {
			return ""
		}
		return p.Name
	}
	return ""
}

// truncateForSummary clips a string and appends an ellipsis when over the
// cap. Trailing whitespace is preserved only inside the prefix (we don't
// trim mid-string to keep the user's original wording intact up to the
// cut point).
func truncateForSummary(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
