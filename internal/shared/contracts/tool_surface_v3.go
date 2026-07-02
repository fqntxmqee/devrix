package contracts

// ToolSpec v3 extension — 6 new control plane fields (D2-S15-A02-T06..T07)
//
// DSAFT: D2-S15-A02-T06 (ToolSpec v3 struct EXTEND) + D2-S15-A02-T07 (4 new type definitions)
// Change: devrix-mups-tool-classification-and-channel-autonomy (DM-20260701-007) Phase A
//
// v2 ToolSpec has 9 fields; v3 adds 6 control plane fields at the END
// of the struct to guarantee position-struct-literal backward compat.
// All existing v2 callers using named-field literals continue to work.
//
// R3 cycle 0 zero-value defaults (designed for the "no metadata" fallback
// path; T14 gate forbids this fallback for any registered tool):
//   EmissionClass       = EC_Action  (zero value = 0)
//   ConvergenceContract = {Kind: 0/None}
//   IterationBound      = {Kind: 0/OpenEnded}
//   SourceUncertainty   = {Source: 0/Deterministic, Value: 0.0}
//   MaxResultSizeChars  = 0  (0 means "no enforced cap")
//   TruncateMarkerText  = "" (caller MUST set DefaultTruncateMarkerText)
//
// Per-tool overrides are applied by DefaultV3MetadataFor in
// internal/layers/contextengine/enforce/tools/surface/orthogonal_flags.go
// (D2-S15-A02-T08..T11).

// EmissionClass classifies a tool's runtime emission pattern. The 4
// orthogonal classes are the runtime projection of the MUPS 4-class
// decomposition (Observe / Execute / Verify / Learn). They drive the
// per-tool ToolChannel routing in Phase B (D7-S9-A50) and the
// PerEmissionClassFilter in Phase D (D2-S15-A02-T02).
//
// EC_Action is intentionally the zero value (iota index 0) so that
// a freshly-constructed ToolSpec without per-tool metadata is the
// conservative "could change state" default — T14 (gate) ensures no
// registered surface ever returns this default.
type EmissionClass int

const (
	// EC_Action (0): state-changing — write_file, edit_file, bash,
	// verify_plan_execution, ask_user_question, task_*. Side effects
	// observable via PostSnapshot != PreSnapshot (L7-ACTION-POSTSNAPSHOT).
	EC_Action EmissionClass = iota

	// EC_Fact (1): deterministic read-only — read_file, grep, glob,
	// query_diagnostics, lsp_goto_definition, lsp_hover, lsp_references,
	// tool_search. Either returns content or errors; no LLM judgement
	// involved.
	EC_Fact

	// EC_Probe (2): exploratory — call_agent, delegate_*, MCP,
	// lsp_workspace_symbol, lsp_code_action, AND (per H12 consensus)
	// read_file, grep, glob when the LLM is in a self-loop recovery
	// cycle. Subject to Bounded(n) hard stop (L4-BOUNDED-ITERATIONS).
	EC_Probe

	// EC_Experiment (3): free_fork, sub-process, worktree. Subject to
	// L7-EXPERIMENT-CONCLUDED-BEFORE-DEADLINE.
	EC_Experiment
)

// String returns the symbolic name (for logs, metrics, JSON tags).
func (e EmissionClass) String() string {
	switch e {
	case EC_Action:
		return "Action"
	case EC_Fact:
		return "Fact"
	case EC_Probe:
		return "Probe"
	case EC_Experiment:
		return "Experiment"
	}
	return "Unknown"
}

// ConvergenceKind is the enumerated form of ConvergenceContract.Kind.
// Drive of the per-class burden of proof (D7-S10-A50 VerifyContract).
type ConvergenceKind int

const (
	// CC_None (0): no convergence required (most reads, default).
	CC_None ConvergenceKind = iota

	// CC_StateChangeRequired (1): PostSnapshot != PreSnapshot.
	// VerifyContract accepts the result only if state changed (or the
	// tool was a no-op by design).
	CC_StateChangeRequired

	// CC_EvidenceRequired (2): at least MinEvidence tool calls or
	// evidence items must precede the deliverable. Bounded(n) +
	// MinEvidence together = "no premature synthesize".
	CC_EvidenceRequired

	// CC_QuotientThreshold (3): metric-based convergence. Used by
	// free_fork (80% of child outputs must agree).
	CC_QuotientThreshold
)

// String returns the symbolic name.
func (k ConvergenceKind) String() string {
	switch k {
	case CC_None:
		return "None"
	case CC_StateChangeRequired:
		return "StateChangeRequired"
	case CC_EvidenceRequired:
		return "EvidenceRequired"
	case CC_QuotientThreshold:
		return "QuotientThreshold"
	}
	return "Unknown"
}

// IterationBoundKind is the enumerated form of IterationBound.Kind.
// ProbeToolChannel (D7-S9-A50-T03) and PerTaskKindFilter (D2-S15-A02-T02)
// consume this to enforce per-tool iteration caps.
type IterationBoundKind int

const (
	// IB_OpenEnded (0): no bound (R3 default; Phase D Filter may add
	// task-kind-level bounds on top).
	IB_OpenEnded IterationBoundKind = iota

	// IB_Bounded (1): hard cap at MaxN; iter > MaxN → InjectSynthesize,
	// iter > MaxN+1 → reject. Read_file/grep/glob Bounded(15) in Phase A.
	IB_Bounded

	// IB_Quotient (2): soft bound on a metric (free_fork 0.8).
	IB_Quotient
)

// String returns the symbolic name.
func (k IterationBoundKind) String() string {
	switch k {
	case IB_OpenEnded:
		return "OpenEnded"
	case IB_Bounded:
		return "Bounded"
	case IB_Quotient:
		return "Quotient"
	}
	return "Unknown"
}

// SourceKind is the enumerated form of SourceUncertainty.Source.
// VerifyContract computes calibrated_confidence from this with the
// emission-class weight (EC_Fact=0.50, EC_Action=0.35, EC_Probe=0.20,
// EC_Experiment=0.10 per Codex Critical #6).
type SourceKind int

const (
	// SK_Deterministic (0): read_file on disk, lsp_*. Value=1.0.
	SK_Deterministic SourceKind = iota

	// SK_LLM (1): LLM-suggested (workspace_symbol, code action).
	// Value=0.4 default (overridden per-tool).
	SK_LLM

	// SK_User (2): bash with explicit user input. Value=0.85.
	SK_User

	// SK_Memory (3): FeedbackMemory recall. Value=0.3.
	SK_Memory
)

// String returns the symbolic name.
func (k SourceKind) String() string {
	switch k {
	case SK_Deterministic:
		return "Deterministic"
	case SK_LLM:
		return "LLM"
	case SK_User:
		return "User"
	case SK_Memory:
		return "Memory"
	}
	return "Unknown"
}

// ConvergenceContract is the per-tool "must converge to what" contract.
// VerifyContract (D7-S10-A50-T01) consumes this to allocate burden of
// proof by class.
type ConvergenceContract struct {
	// Kind is the convergence requirement.
	Kind ConvergenceKind `json:"kind"`

	// Threshold is the QuotientThreshold value (only when Kind=CC_QuotientThreshold).
	// For free_fork, 0.8 means 80% of child outputs must agree.
	Threshold float64 `json:"threshold,omitempty"`

	// MinEvidence is the minimum number of tool calls or evidence
	// items required (only when Kind=CC_EvidenceRequired). Default 1.
	MinEvidence int `json:"min_evidence,omitempty"`
}

// IterationBound is the per-tool "iteration count" ceiling that
// ProbeToolChannel (Phase B) and PerTaskKindFilter (Phase D) consume.
type IterationBound struct {
	// Kind is the bound mode.
	Kind IterationBoundKind `json:"kind"`

	// MaxN is the Bounded(n) ceiling (only when Kind=IB_Bounded).
	// Default 0 = unset.
	MaxN int `json:"max_n,omitempty"`

	// Quotient is the metric threshold (only when Kind=IB_Quotient).
	// Default 0.0 = unset.
	Quotient float64 `json:"quotient,omitempty"`
}

// SourceUncertainty declares the tool's information source trust level.
// VerifyContract computes calibrated_confidence from this value with
// the emission-class weight (EC_Fact=0.50, EC_Action=0.35, EC_Probe=0.20,
// EC_Experiment=0.10 per Codex Critical #6).
type SourceUncertainty struct {
	// Source is the source kind.
	Source SourceKind `json:"source"`

	// Value is the calibrated source quality in [0, 1].
	// 1.0 = fully trusted, 0.0 = no trust.
	Value float64 `json:"value"`
}

// DefaultTruncateMarkerText is the marker template appended when
// D2 TruncateWithMarker (D2-S15-A02-T13) cuts the tool output. The
// %d placeholders are substituted (chars kept, total chars). The
// marker MUST be visible to the LLM (no silent default) so it can
// request a REREAD or refile search on a smaller scope.
const DefaultTruncateMarkerText = "[TRUNCATED at %d/%d chars, complete=false, REREAD may help]"
