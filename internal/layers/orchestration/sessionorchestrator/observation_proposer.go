package sessionorchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

const (
	maxLLMObsFactStrength = 0.85
	llmObsProposerSource  = "llm_observation_proposer"
)

// ObserveSignalInput carries structured signals for LLM observation proposals.
// MUST NOT include WorkItem private ReAct transcript (DM-20260627-003 LC6 / T35).
type ObserveSignalInput struct {
	SessionID          string
	WorkItemID         string
	Directive          string
	ScopeContract      *workmodel.ScopeContract
	InboundSignalLines []string
	PriorMean          float64
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
		in.ScopeContract = item.ScopeContract
	}
	if item != nil && item.LastRound != nil {
		if s := strings.TrimSpace(item.LastRound.ArtifactSummary); s != "" {
			in.InboundSignalLines = append(in.InboundSignalLines,
				"artifact_summary: "+workmodel.TruncateArtifactSummary(s, 240))
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
) ([]orchtypes.Observation, error) {
	if proposer == nil || item == nil {
		return nil, nil
	}
	in := buildObserveSignalInput(sessionID, item, tm)
	if prior != nil {
		in.PriorMean = prior.PriorBeta.Mean()
	}
	proposals, err := proposer.ProposeObservations(ctx, in)
	if err != nil || len(proposals) == 0 {
		return nil, err
	}
	return ValidateObservationProposals(proposals, sessionID, item.ID)
}
