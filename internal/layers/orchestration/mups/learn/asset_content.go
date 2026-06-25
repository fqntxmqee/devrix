package learn

import (
	"errors"
	"time"
)

// AssetContent is the polymorphic content interface for the 5 LearningAsset
// classes (LP-1 衍生: validate before storage).
type AssetContent interface {
	// Validate returns nil if the content satisfies its class-specific
	// required-field invariants. Otherwise returns a non-nil error (typically
	// wrapped ErrAssetIncomplete).
	Validate() error

	// SchemaVersion returns the AssetContent schema version. Bumped when the
	// struct adds/removes fields.
	SchemaVersion() string

	// ByteSize returns an estimate of the marshaled size (for D5 metrics).
	ByteSize() int
}

// ──────────────────────────────────────────────────────────
// ★5 SOPAssetContent — Standard Operating Procedure
// Source: ComplianceVerdict.
// ──────────────────────────────────────────────────────────

// SOPAssetContent is the content for LearningSOP. A SOP is a deterministic,
// ordered procedure: PreConditions → Steps → PostConditions.
type SOPAssetContent struct {
	Name            string   `json:"name"`
	Description     string   `json:"description,omitempty"`
	Steps           []string `json:"steps"`
	PreConditions   []string `json:"pre_conditions,omitempty"`
	PostConditions  []string `json:"post_conditions,omitempty"`
	ApplicableTools []string `json:"applicable_tools,omitempty"`
	EstimatedMs     int      `json:"estimated_ms,omitempty"`
}

// Validate — Name and Steps (≥1) are required.
func (c *SOPAssetContent) Validate() error {
	if c.Name == "" {
		return errors.New("sop: Name is required")
	}
	if len(c.Steps) == 0 {
		return errors.New("sop: Steps must have at least 1 entry")
	}
	return nil
}

// SchemaVersion implements AssetContent.
func (c *SOPAssetContent) SchemaVersion() string { return CurrentAssetSchemaVersion }

// ByteSize implements AssetContent.
func (c *SOPAssetContent) ByteSize() int {
	return len(c.Name) + len(c.Description) + len(c.Steps)*64
}

// ──────────────────────────────────────────────────────────
// ★4 ProtocolAssetContent — Protocol
// Source: TimelinessVerdict.
// ──────────────────────────────────────────────────────────

// SLAConfig configures the protocol's latency / retry budget.
type SLAConfig struct {
	TargetMs    int           `json:"target_ms"`
	MaxRetries  int           `json:"max_retries"`
	OpenTimeout time.Duration `json:"open_timeout_ns"`
}

// ProtocolAssetContent is the content for LearningProtocol. A protocol is a
// conditional response: when Trigger matches, run Actions under SLA, then
// Fallback.
type ProtocolAssetContent struct {
	Name     string    `json:"name"`
	Trigger  string    `json:"trigger"`
	Actions  []string  `json:"actions,omitempty"`
	SLA      SLAConfig `json:"sla"`
	Fallback string    `json:"fallback,omitempty"`
}

// Validate — Trigger is required.
func (c *ProtocolAssetContent) Validate() error {
	if c.Trigger == "" {
		return errors.New("protocol: Trigger is required")
	}
	return nil
}

// SchemaVersion implements AssetContent.
func (c *ProtocolAssetContent) SchemaVersion() string { return CurrentAssetSchemaVersion }

// ByteSize implements AssetContent.
func (c *ProtocolAssetContent) ByteSize() int { return 256 }

// ──────────────────────────────────────────────────────────
// ★3 KnowledgeAssetContent — Knowledge
// Source: RootCauseVerdict.
// ──────────────────────────────────────────────────────────

// KnowledgeAssetContent is the content for LearningKnowledge. Knowledge is
// a hypothesis with evidence + counter-evidence.
type KnowledgeAssetContent struct {
	Topic        string   `json:"topic"`
	Hypothesis   string   `json:"hypothesis"`
	Evidence     []string `json:"evidence,omitempty"`
	CounterEvid  []string `json:"counter_evid,omitempty"`
	Confidence   float64  `json:"confidence"`
	RelatedCases []string `json:"related_cases,omitempty"`
}

// Validate — Topic, Hypothesis required; Confidence must be in [0,1].
func (c *KnowledgeAssetContent) Validate() error {
	if c.Topic == "" {
		return errors.New("knowledge: Topic is required")
	}
	if c.Hypothesis == "" {
		return errors.New("knowledge: Hypothesis is required")
	}
	if c.Confidence < 0 || c.Confidence > 1 {
		return errors.New("knowledge: Confidence must be in [0, 1]")
	}
	return nil
}

// SchemaVersion implements AssetContent.
func (c *KnowledgeAssetContent) SchemaVersion() string { return CurrentAssetSchemaVersion }

// ByteSize implements AssetContent.
func (c *KnowledgeAssetContent) ByteSize() int { return 512 }

// ──────────────────────────────────────────────────────────
// ★2 ConclusionAssetContent — Conclusion
// Source: StatisticalVerdict.
// ──────────────────────────────────────────────────────────

// ConclusionAssetContent is the content for LearningConclusion. A conclusion
// is a statistical statement with confidence interval.
type ConclusionAssetContent struct {
	Statement          string    `json:"statement"`
	PValue             float64   `json:"p_value"`
	ConfidenceInterval [2]float64 `json:"confidence_interval"`
	SampleSize         int       `json:"sample_size"`
	Methodology        string    `json:"methodology,omitempty"`
	Limitations        []string  `json:"limitations,omitempty"`
}

// Validate — Statement is required.
func (c *ConclusionAssetContent) Validate() error {
	if c.Statement == "" {
		return errors.New("conclusion: Statement is required")
	}
	return nil
}

// SchemaVersion implements AssetContent.
func (c *ConclusionAssetContent) SchemaVersion() string { return CurrentAssetSchemaVersion }

// ByteSize implements AssetContent.
func (c *ConclusionAssetContent) ByteSize() int { return 256 }

// ──────────────────────────────────────────────────────────
// ⭐★1 PendingAssetContent — Pending retry (5th class, ⭐new in Phase 5)
// Source: VerdictIndeterminate.
// ──────────────────────────────────────────────────────────

// PendingAssetContent is the content for LearningPending. A pending asset
// represents a VerdictIndeterminate that requires retry (env-limited) or
// carries an MVE checkpoint state (user-decision pending).
//
// IndeterminateReason distinguishes env-limited vs verifier-failure (see
// doc 46 §3.3 G8-1 fix):
//
//	"verifier_parse_failure" — verifier LLM output format issue
//	(other)                  — env-limited or other transient issue
type PendingAssetContent struct {
	IndeterminateReason string    `json:"indeterminate_reason"`
	OriginalArtifactID  string    `json:"original_artifact_id"`
	RetryAttempts       int       `json:"retry_attempts"`
	MaxRetries          int       `json:"max_retries"`
	NextRetryAt         time.Time `json:"next_retry_at"`
	BlockedReason       string    `json:"blocked_reason,omitempty"`

	// MVE checkpoint state (from doc 44 §4.4 StrategyDecider, MVP ⭐new).
	// When MVEState is non-nil, the PendingAsset represents a user-decision
	// pending state rather than an env-limited retry. The Question + Options
	// fields are surfaced to the user.
	MVEState  *PendingMVEState `json:"mve_state,omitempty"`
	PlanID    string           `json:"plan_id,omitempty"`
	SessionID string           `json:"session_id,omitempty"`
	Question  string           `json:"question,omitempty"`
	Options   []string         `json:"options,omitempty"`
}

// PendingMVEState captures the relevant fields of execute.MVEState at the
// point of pending, so the next Observe can resume without re-fetching from
// execute. We use a local type to avoid an import cycle (learn ↔ execute);
// the conversion happens in PR-E5 E5.4 when wiring.
type PendingMVEState struct {
	Round             int    `json:"round"`
	Mode              string `json:"mode"`
	StrategyDecision  string `json:"strategy_decision"`
	LastDecisionAtMs  int64  `json:"last_decision_at_ms"`
	ResolvedAnswerIdx int    `json:"resolved_answer_idx,omitempty"`
}

// Validate — IndeterminateReason, OriginalArtifactID required;
// RetryAttempts ∈ [0, 3]; MVEState non-nil implies Question required.
func (c *PendingAssetContent) Validate() error {
	if c.IndeterminateReason == "" {
		return errors.New("pending: IndeterminateReason is required")
	}
	if c.OriginalArtifactID == "" {
		return errors.New("pending: OriginalArtifactID is required")
	}
	if c.RetryAttempts < 0 || c.RetryAttempts > 3 {
		return errors.New("pending: RetryAttempts must be in [0, 3]")
	}
	if c.MVEState != nil && c.Question == "" {
		return errors.New("pending: Question is required when MVEState is set")
	}
	return nil
}

// SchemaVersion implements AssetContent.
func (c *PendingAssetContent) SchemaVersion() string { return CurrentAssetSchemaVersion }

// ByteSize implements AssetContent.
func (c *PendingAssetContent) ByteSize() int { return 384 }