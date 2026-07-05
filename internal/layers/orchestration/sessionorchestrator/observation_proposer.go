package sessionorchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/prompttags"
)

const (
	maxLLMObsFactStrength    = 0.85
	maxObservationProposals  = 3 // matches i18n ObservationTaskAppendix "Maximum 3 proposals"
	llmObsProposerSource     = "observation_proposer" // StaticObservationProposer / tests only
)

// ObserveSignalInput carries structured signals for LLM observation proposals.
// MUST NOT include WorkItem private ReAct transcript (DM-20260627-003 LC6 / T35).
//
// Field order matches ObserveUserFrame.Fields; pt struct tag is the SoT for
// the user-prompt schema (DM-20260705-003 go-struct-driven).
// init() registers this struct with prompttags.MustRegisterFrame; any drift
// between struct / FrameSpec / i18n guide panics at process start.
type ObserveSignalInput struct {
	// SessionID is not part of the user frame (routing only).
	SessionID string `pt:"-"`

	// 1. work_item_id (control) — always emitted.
	WorkItemID string `pt:"work_item_id,control"`

	// 2. directive (data) — always emitted.
	Directive string `pt:"directive,data"`

	// 3. prior_parse_reject (control, omit_empty) — DM-20260705-002 cross-round feedback.
	PriorParseReject string `pt:"prior_parse_reject,control,omit_empty"`

	// 4. prior_mean (control, omit_zero) — Bayesian prior (DM-20260624-001 LP-1).
	PriorMean float64 `pt:"prior_mean,control,omit_zero"`

	// 5. scope_goal (data, omit_empty) — flattened from item.ScopeContract.GoalStatement.
	ScopeGoal string `pt:"scope_goal,data,omit_empty"`

	// 6. scope_open_question (data, omit_empty, multi-line) — flattened from item.ScopeContract.OpenQuestions.
	ScopeOpenQuestions []string `pt:"scope_open_question,data,omit_empty"`

	// 7. signal (data, omit_empty, multi-line) — artifact_summary + child_downlink lines.
	InboundSignalLines []string `pt:"signal,data,omit_empty"`

	// 8. prior_observation_ids (control, omit_empty, comma-joined) — incremental context.
	PriorObservationIDs []string `pt:"prior_observation_ids,control,omit_empty"`

	// 9. incremental_only (control, omit_zero) — true iff PriorObservationIDs non-empty.
	IncrementalOnly bool `pt:"incremental_only,control,omit_zero"`
}

// init registers ObserveSignalInput with the prompttags user-frame registry.
// Panics at process start on any drift between struct / FrameSpec / i18n guide.
func init() {
	prompttags.MustRegisterFrame[ObserveSignalInput](prompttags.FrameObserveUser)
}

// ObservationProposal is a raw LLM proposal before rule validation (G3: propose → rule).
type ObservationProposal struct {
	Kind      orchtypes.ObservationKind
	Category  orchtypes.Category
	Strength  float64
	Statement string
	Question  string
	Evidence  []string
}

// ObservationProposer proposes Obs* candidates from structured signals only.
type ObservationProposer interface {
	ProposeObservations(ctx context.Context, in ObserveSignalInput) ([]ObservationProposal, error)
}

// StaticObservationProposer returns fixed proposals (tests / fixtures).
type StaticObservationProposer struct {
	Proposals []ObservationProposal
	Err       error
}

func (p StaticObservationProposer) ProposeObservations(_ context.Context, _ ObserveSignalInput) ([]ObservationProposal, error) {
	if p.Err != nil {
		return nil, p.Err
	}
	return append([]ObservationProposal(nil), p.Proposals...), nil
}

func buildObserveSignalInput(sessionID string, item *workmodel.WorkItem, tm *workmodel.TaskManager) ObserveSignalInput {
	in := ObserveSignalInput{
		SessionID:  sessionID,
		WorkItemID: item.ID,
		Directive:  itemDirective(item),
	}
	if item != nil && item.ScopeContract != nil {
		if s := strings.TrimSpace(item.ScopeContract.GoalStatement); s != "" {
			in.ScopeGoal = s
		}
		var questions []string
		for _, q := range item.ScopeContract.OpenQuestions {
			if strings.TrimSpace(q) != "" {
				questions = append(questions, q)
			}
		}
		in.ScopeOpenQuestions = questions
	}
	if item != nil && item.LastRound != nil {
		if s := strings.TrimSpace(item.LastRound.ArtifactSummary); s != "" {
			in.InboundSignalLines = append(in.InboundSignalLines,
				"artifact_summary: "+workmodel.TruncateArtifactSummary(s, 240))
		}
	}
	if item != nil && item.LastRound != nil && len(item.LastRound.ObservationIDs) > 0 {
		in.PriorObservationIDs = append([]string(nil), item.LastRound.ObservationIDs...)
		in.IncrementalOnly = true
	}
	if item != nil && item.LastRound != nil {
		if s := strings.TrimSpace(item.LastRound.ObserveParseReject); s != "" {
			in.PriorParseReject = s
		}
	}
	if tm != nil && item != nil {
		if dl, ok := tm.ChildDownlinkFor(sessionID, item.ID); ok {
			if len(dl.ScopeIn) > 0 {
				in.InboundSignalLines = append(in.InboundSignalLines,
					"child_downlink_scope_in: "+strings.Join(dl.ScopeIn, ", "))
			}
			if dl.ExpectedReturn != "" {
				in.InboundSignalLines = append(in.InboundSignalLines,
					"expected_return: "+dl.ExpectedReturn)
			}
		}
	}
	return in
}

// ValidateObservationProposals applies rule gates before Obs* enter UncertaintyReport.
func ValidateObservationProposals(proposals []ObservationProposal, sessionID, workItemID string) ([]orchtypes.Observation, error) {
	var out []orchtypes.Observation
	for _, p := range proposals {
		o, err := validateOneProposal(p, sessionID, workItemID)
		if err != nil {
			continue
		}
		out = append(out, o)
		if len(out) >= maxObservationProposals {
			break
		}
	}
	return out, nil
}

func validateOneProposal(p ObservationProposal, sessionID, workItemID string) (orchtypes.Observation, error) {
	category := p.Category
	if category > orchtypes.CatSystem {
		category = orchtypes.CatBusiness
	}
	strength := p.Strength
	if strength <= 0 {
		strength = 0.5
	}
	if p.Kind == orchtypes.ObsFact && strength > maxLLMObsFactStrength {
		strength = maxLLMObsFactStrength
	}
	evidence := append([]string(nil), p.Evidence...)
	if !evidenceContains(evidence, workItemID) {
		evidence = append(evidence, workItemID)
	}
	if !evidenceContains(evidence, sessionID) {
		evidence = append(evidence, sessionID)
	}

	var payload orchtypes.Payload
	switch p.Kind {
	case orchtypes.ObsFact:
		stmt := strings.TrimSpace(p.Statement)
		if stmt == "" {
			return orchtypes.Observation{}, fmt.Errorf("obs fact: empty statement")
		}
		payload = orchtypes.FactPayload{Statement: stmt, Evidence: evidence}
	case orchtypes.ObsSignal:
		stmt := strings.TrimSpace(p.Statement)
		if stmt == "" {
			stmt = "llm_signal"
		}
		payload = orchtypes.SignalPayload{Name: stmt, Value: strength, Threshold: 0.5}
	case orchtypes.ObsDeviation:
		stmt := strings.TrimSpace(p.Statement)
		if stmt == "" {
			return orchtypes.Observation{}, fmt.Errorf("obs deviation: empty statement")
		}
		payload = orchtypes.DeviationPayload{Metric: stmt, Expected: 0, Observed: strength, Delta: strength}
	case orchtypes.ObsUncertainty:
		q := strings.TrimSpace(p.Question)
		if q == "" {
			q = strings.TrimSpace(p.Statement)
		}
		if q == "" {
			return orchtypes.Observation{}, fmt.Errorf("obs uncertainty: empty question")
		}
		payload = orchtypes.UncertaintyPayload{
			Question:     q,
			Confidence:   1 - strength,
			RequiresMore: true,
		}
	default:
		return orchtypes.Observation{}, fmt.Errorf("unknown kind")
	}
	return orchtypes.NewObservation(p.Kind, category, strength, payload, llmObsProposerSource)
}

func evidenceContains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func mergeProposedObservations(
	ctx context.Context,
	proposer ObservationProposer,
	sessionID string,
	item *workmodel.WorkItem,
	tm *workmodel.TaskManager,
	prior *learn.AdaptivePrior,
) ([]orchtypes.Observation, string, error) {
	if proposer == nil || item == nil {
		return nil, "", nil
	}
	in := buildObserveSignalInput(sessionID, item, tm)
	if prior != nil {
		in.PriorMean = prior.PriorBeta.Mean()
	}
	proposals, err := proposer.ProposeObservations(ctx, in)
	if err != nil {
		return nil, parseRejectFromObserveError(err, "").CompactJSON(), nil
	}
	if len(proposals) == 0 {
		return nil, "", nil
	}
	obs, valErr := ValidateObservationProposals(proposals, sessionID, item.ID)
	if valErr != nil {
		return obs, parseRejectFromObserveError(valErr, "").CompactJSON(), nil
	}
	if len(proposals) > 0 && len(obs) == 0 {
		return nil, prompttags.NewObserveParseReject(prompttags.RejectValidateEmpty, "all proposals failed validation", "").CompactJSON(), nil
	}
	return obs, "", nil
}
