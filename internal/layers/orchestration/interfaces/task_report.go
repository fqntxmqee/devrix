package interfaces

import (
	"strings"
	"time"
)

// ResultKind enumerates the 4 outcomes the pipeline may produce. The values
// match the v4.4 VerdictKind enum used by Verifier so a TaskReport.Result
// can be assigned to a Verdict without translation.
type ResultKind int

const (
	// ResultKindPending — execution hasn't reached a verdict yet. This is
	// the NewTaskReport default; every freshly constructed report starts
	// here until a Worker or Channel writes the final result.
	ResultKindPending ResultKind = iota
	// ResultKindPass — work succeeded and Verifier accepted the artifact.
	ResultKindPass
	// ResultKindPartial — work produced an artifact but Verifier flagged
	// soft concerns. SkillMemory.SOP records this with a downgrade note.
	ResultKindPartial
	// ResultKindIndeterminate — work couldn't reach a verdict (timeout,
	// divergence, plan exhausted). Triggers Dissent capture.
	ResultKindIndeterminate
	// ResultKindFailed — work failed terminally. Result.Message carries
	// the human-readable reason.
	ResultKindFailed
)

// String returns the canonical ResultKind name. Used by spans, logs and
// the LP-1 Bayesian reputation lookup (the keys are case-sensitive).
func (k ResultKind) String() string {
	switch k {
	case ResultKindPending:
		return "pending"
	case ResultKindPass:
		return "pass"
	case ResultKindPartial:
		return "partial"
	case ResultKindIndeterminate:
		return "indeterminate"
	case ResultKindFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Result is the verdict produced by Channel.Execute (after Verifier). It is
// embedded in TaskReport and intentionally minimal — anything richer (e.g.
// exit reason, error chain) lives in Blockage / Evidence.
type Result struct {
	Kind       ResultKind
	Confidence float64       // [0,1]; advisory, Bayesian reputation uses this
	Message    string        // human-readable summary; sanitized by SanitizeForUser before display
	At         time.Time     // when the verdict was reached
}

// Evidence is the structured backing for Result. At least one of the three
// optional fields SHOULD be set when Result.Kind = Pass; the v7.0 HardEvidence
// rule (AC15, defined in PR-B) escalates the lack of any field to Partial.
type Evidence struct {
	TestResult       string // human-readable test outcome (e.g. "5/5 pass, coverage 87%")
	LogExcerpt       string // capped log lines (≤ 4 KiB; sanitized)
	ArtifactHash     string // hex SHA-256 of the produced artifact
}

// BlockageKind classifies why a plan could not complete. The 3 values map
// directly to the 3 remediation paths the planner considers when re-
// planning: feed more info, pick a different path, ask the user for input.
type BlockageKind int

const (
	// BlockageMissing — the planner needs more information (missing input,
	// unspecified parameter). Re-planning should query the user or wait for
	// upstream context to fill in.
	BlockageMissing BlockageKind = iota
	// BlockageInfeasible — the chosen path is known to fail (e.g. tool not
	// available in this environment). Re-planning must pick a different path.
	BlockageInfeasible
	// BlockageRequiredExternal — completion requires an external action
	// (human approval, manual test, third-party API). Re-planning should
	// park the WorkItem and emit a "blocked" EngineEvent to D1.
	BlockageRequiredExternal
)

// String returns the canonical BlockageKind name. The string values are
// stable for span attributes and downstream filtering.
func (k BlockageKind) String() string {
	switch k {
	case BlockageMissing:
		return "missing"
	case BlockageInfeasible:
		return "infeasible"
	case BlockageRequiredExternal:
		return "required_external"
	default:
		return "unknown"
	}
}

// Blockage is one structured "why this couldn't finish" record. The 5 层 CB
// reads Blockage as the escalation signal: BlockageInfeasible promotes to a
// forced retry, BlockageRequiredExternal emits a "blocked" EngineEvent and
// BlockageMissing asks the planner for more context.
type Blockage struct {
	Kind        BlockageKind
	Description string    // human-readable explanation; sanitized
	Source      string    // component that emitted the blockage (verifier id, tool name)
	Traceback   string    // call chain leading to the blockage; capped to 1 KiB
	At          time.Time
}

// Resource is the per-Plan execution cost. All four numeric fields are
// non-negative — WithResource enforces this and returns ErrResourceInvalid
// otherwise. Producers should pull these numbers from ContextBudget Phase B
// (see decisionplanning.Decompose.resourceFromBudget, defined in PR-B).
type Resource struct {
	TokensUsed       int
	TokensBudget     int
	TimeElapsed      time.Duration
	StepCount        int
	ToolInvocations  int
}

// TaskReport is the unified up-link contract. Every Channel.Execute exit
// point and every Learn-node entry point MUST construct / accept a
// TaskReport so that the feedback path has a single, type-safe shape.
//
// TaskReport is immutable. With* methods and AppendDissent return shallow
// copies; the receiver is never modified. This lets callers share a base
// report across goroutines while each appends their own Dissent/Blockage.
type TaskReport struct {
	// TraceID ties this report back to its TaskSpec. NewTaskReport requires
	// a non-empty traceID.
	TraceID string

	// Result is the verdict produced by Verifier. Result.Kind = ResultKindPending
	// on a freshly constructed report.
	Result Result

	// Evidence backs Result. Optional on construction; required (>= 1
	// field) for Result.Kind = Pass under AC15 (enforced in PR-B).
	Evidence Evidence

	// Dissent holds minority plans captured by ExplorationChannel. Top-N
	// truncation via AppendDissent (default N = 3). Empty slice when no
	// dissent applies.
	Dissent []DissentEntry

	// Blockage holds structured "why this couldn't finish" reasons. The 5
	// 层 CB and the planner read this to decide on retry vs re-plan vs
	// park. Empty slice on a successful report.
	Blockage []Blockage

	// Resource records the per-Plan cost. Set by WithResource after the
	// Channel has finished and ContextBudget has been queried.
	Resource Resource

	// FallbackUsed records whether FallbackPolicy fired. PR-A leaves this
	// as false (no FallbackPolicy consumer exists yet); PR-B flips it when
	// the pessimistic / rule-based fallbacks land.
	FallbackUsed bool

	// MVPArtifact — reserved for PR-B (AC11 Pessimistic Commit). Always
	// nil in PR-A. Held as a pointer so the zero value remains cheap.
	MVPArtifact *MVPArtifact

	// At records when the report was constructed.
	At time.Time
}

// MVPArtifact is the shape PR-B will populate when FallbackPessimistic
// fires. The struct is defined here (as a placeholder) so the TaskReport
// field signature is stable across PR-A and PR-B. Producers in PR-A
// always leave this nil.
type MVPArtifact struct {
	Output       string
	RiskWarnings []string
	Trigger      string
	ChainHash    string
}

// NewTaskReport constructs a TaskReport with the supplied TraceID. Result.Kind
// defaults to ResultKindPending; the rest is zero-value. The Dissent and
// Blockage slices are non-nil empty slices so callers can append without a
// nil-check. Returns ErrTaskReportTraceIDEmpty when traceID is empty.
func NewTaskReport(traceID string) (*TaskReport, error) {
	if strings.TrimSpace(traceID) == "" {
		return nil, NewTaskReportTraceIDEmptyError()
	}
	return &TaskReport{
		TraceID:  traceID,
		Result:   Result{Kind: ResultKindPending, At: time.Now()},
		Evidence: Evidence{},
		Dissent:  []DissentEntry{},
		Blockage: []Blockage{},
		Resource: Resource{},
		At:       time.Now(),
	}, nil
}

// WithResult returns a shallow copy with the supplied Result.
func (r *TaskReport) WithResult(res Result) *TaskReport {
	c := *r
	c.Result = res
	return &c
}

// WithEvidence returns a shallow copy with the supplied Evidence.
func (r *TaskReport) WithEvidence(ev Evidence) *TaskReport {
	c := *r
	c.Evidence = ev
	return &c
}

// WithBlockage returns a shallow copy with one Blockage appended. Unlike
// AppendDissent, blockage has no top-N cap — multiple blockages of different
// kinds can coexist and the 5 层 CB consults each one.
func (r *TaskReport) WithBlockage(b Blockage) *TaskReport {
	c := *r
	bs := make([]Blockage, len(r.Blockage)+1)
	copy(bs, r.Blockage)
	bs[len(r.Blockage)] = b
	c.Blockage = bs
	return &c
}

// AppendDissent returns a shallow copy with one DissentEntry appended, or
// the receiver unchanged if the entry's Reason is empty (returns
// ErrDissentRejection) or if the report already holds DissentMaxEntries
// entries (returns the receiver unchanged, no error — top-N truncation is
// silent). The Learn node dedups by entry.Summary hash downstream.
func (r *TaskReport) AppendDissent(entry DissentEntry) (*TaskReport, error) {
	if strings.TrimSpace(entry.Reason) == "" {
		return r, NewDissentRejectionError()
	}
	if len(r.Dissent) >= DissentMaxEntries {
		// Silent truncation — top-N is the contract, callers shouldn't
		// have to handle "would have appended but we dropped it".
		return r, nil
	}
	c := *r
	ds := make([]DissentEntry, len(r.Dissent)+1)
	copy(ds, r.Dissent)
	ds[len(r.Dissent)] = entry
	c.Dissent = ds
	return &c, nil
}

// WithResource returns a shallow copy with the supplied Resource. Negative
// values in any numeric field return ErrResourceInvalid and the receiver
// unchanged — the caller should treat the returned error as fatal because
// a corrupted Resource would propagate into the audit log.
func (r *TaskReport) WithResource(res Resource) (*TaskReport, error) {
	if res.TokensUsed < 0 || res.TokensBudget < 0 || res.TimeElapsed < 0 ||
		res.StepCount < 0 || res.ToolInvocations < 0 {
		return r, NewResourceInvalidError()
	}
	c := *r
	c.Resource = res
	return &c, nil
}

// WithFallbackUsed returns a shallow copy with FallbackUsed set.
func (r *TaskReport) WithFallbackUsed(used bool) *TaskReport {
	c := *r
	c.FallbackUsed = used
	return &c
}

// WithMVPArtifact returns a shallow copy with the supplied MVPArtifact.
// PR-A leaves MVPArtifact nil; PR-B populates it from the pessimistic
// fallback path.
func (r *TaskReport) WithMVPArtifact(mvp *MVPArtifact) *TaskReport {
	c := *r
	c.MVPArtifact = mvp
	return &c
}

// Validate enforces the invariants NewTaskReport already established. Cheap
// and idempotent; callers can call it any time they receive a TaskReport
// from an untrusted source.
func (r *TaskReport) Validate() error {
	if r == nil {
		return NewTaskReportTraceIDEmptyError()
	}
	if strings.TrimSpace(r.TraceID) == "" {
		return NewTaskReportTraceIDEmptyError()
	}
	return nil
}