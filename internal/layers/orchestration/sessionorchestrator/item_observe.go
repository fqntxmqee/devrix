package sessionorchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

func itemDirective(item *workmodel.WorkItem) string {
	if item == nil {
		return ""
	}
	if s := strings.TrimSpace(item.Directive); s != "" {
		return s
	}
	return strings.TrimSpace(item.Title)
}

func buildObserveRequestForItem(ctx context.Context, sessionID, message, trackMode string, learner learn.Learner) (orchtypes.ObserveRequest, error) {
	var prior *learn.AdaptivePrior
	if learner != nil {
		injected, err := learner.Inject(ctx, sessionID, trackMode)
		if err == nil {
			prior = injected
		}
	}
	return orchtypes.NewObserveRequest(sessionID, message, prior)
}

func observeWorkItem(
	ctx context.Context,
	sessionID string,
	item *workmodel.WorkItem,
	classifier decisionplanning.IntentClassifier,
	learner learn.Learner,
	trackMode string,
	tasks *workmodel.TaskManager,
) (orchtypes.UncertaintyReport, []string, error) {
	directive := itemDirective(item)
	if directive == "" {
		return orchtypes.UncertaintyReport{}, nil, fmt.Errorf("item_pipeline: empty directive")
	}
	observeReq, err := buildObserveRequestForItem(ctx, sessionID, directive, trackMode, learner)
	if err != nil {
		return orchtypes.UncertaintyReport{}, nil, err
	}
	prior := observeReq.EffectivePrior()

	var intent orchtypes.IntentClassification
	if classifier != nil {
		intent, err = classifier.ClassifyWithPrior(ctx, directive, prior)
		if err != nil {
			return orchtypes.UncertaintyReport{}, nil, fmt.Errorf("item_pipeline: classify: %w", err)
		}
	} else {
		intent = orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate, Confidence: 80}
	}

	obs, err := observationsFromItem(sessionID, item, intent, prior)
	if err != nil {
		return orchtypes.UncertaintyReport{}, nil, err
	}
	scopeObs, err := mapScopeContractToObservations(sessionID, item)
	if err != nil {
		return orchtypes.UncertaintyReport{}, nil, err
	}
	obs = append(obs, scopeObs...)
	childObs, err := observationsFromChildStructuredBubbles(tasks, sessionID, item)
	if err != nil {
		return orchtypes.UncertaintyReport{}, nil, err
	}
	obs = append(obs, childObs...)
	if item.NeedsRollup {
		summaryObs, err := observationsFromChildSummaryBubbles(tasks, sessionID, item)
		if err != nil {
			return orchtypes.UncertaintyReport{}, nil, err
		}
		obs = append(obs, summaryObs...)
		checklistObs, err := observationsFromChecklistChildBubbles(tasks, sessionID, item)
		if err != nil {
			return orchtypes.UncertaintyReport{}, nil, err
		}
		obs = append(obs, checklistObs...)
	}
	report, err := orchtypes.NewUncertaintyReport(sessionID, obs)
	if err != nil {
		return orchtypes.UncertaintyReport{}, nil, err
	}
	report.QuantizedIntent = &orchtypes.QuantizedIntent{
		Kind:       intent.Kind,
		Confidence: float64(intent.Confidence) / 100,
		Reason:     intent.Reason,
		Source:     "item_pipeline",
	}
	if prior != nil {
		report.Prior = &orchtypes.AdaptivePrior{
			Score:      prior.PriorBeta.Mean(),
			Confidence: prior.PriorBeta.Mean(),
			Source:     "reputation",
		}
	}

	ids := make([]string, 0, len(obs))
	for _, o := range obs {
		ids = append(ids, o.ID)
	}
	return report, ids, nil
}

func observationsFromItem(
	sessionID string,
	item *workmodel.WorkItem,
	intent orchtypes.IntentClassification,
	prior *learn.AdaptivePrior,
) ([]orchtypes.Observation, error) {
	uncertainty := item.Uncertainty
	if uncertainty <= 0 {
		uncertainty = workmodel.DefaultUncertaintyDecomposeThreshold - 0.1
	}

	var obs []orchtypes.Observation
	if uncertainty >= workmodel.DefaultUncertaintyDecomposeThreshold {
		o, err := orchtypes.NewObservation(
			orchtypes.ObsUncertainty,
			orchtypes.CatBusiness,
			uncertainty,
			orchtypes.UncertaintyPayload{
				Question:   itemDirective(item),
				Confidence: 1 - uncertainty,
				RequiresMore: true,
			},
			"item_pipeline",
		)
		if err != nil {
			return nil, err
		}
		obs = append(obs, o)
	}

	if intent.Kind == orchtypes.IntentOrchestrate {
		strength := 0.55
		if prior != nil {
			strength = prior.PriorBeta.Mean()
		}
		o, err := orchtypes.NewObservation(
			orchtypes.ObsSignal,
			orchtypes.CatBusiness,
			strength,
			orchtypes.SignalPayload{Name: "orchestrate_intent", Value: float64(intent.Confidence), Threshold: 70},
			"item_pipeline",
		)
		if err != nil {
			return nil, err
		}
		obs = append(obs, o)
	}

	strength := 0.85
	if prior != nil {
		strength = prior.PriorBeta.Mean()
	}
	fact, err := orchtypes.NewObservation(
		orchtypes.ObsFact,
		orchtypes.CatBusiness,
		strength,
		orchtypes.FactPayload{Statement: itemDirective(item), Evidence: []string{sessionID, item.ID}},
		"item_pipeline",
	)
	if err != nil {
		return nil, err
	}
	obs = append(obs, fact)
	return obs, nil
}

// mapScopeContractToObservations applies R-OBS-1/R-OBS-2 rules (DM-20260627-003).
// LastRound carries Execute signals; Obs* taxonomy is produced only here at Observe.
func mapScopeContractToObservations(sessionID string, item *workmodel.WorkItem) ([]orchtypes.Observation, error) {
	if item == nil || item.ScopeContract == nil {
		return nil, nil
	}
	sc := item.ScopeContract
	var obs []orchtypes.Observation
	if sc.HasOpenQuestions() {
		for _, q := range sc.OpenQuestions {
			q = strings.TrimSpace(q)
			if q == "" {
				continue
			}
			o, err := orchtypes.NewObservation(
				orchtypes.ObsUncertainty,
				orchtypes.CatBusiness,
				0.9,
				orchtypes.UncertaintyPayload{
					Question:     q,
					Confidence:   0.1,
					RequiresMore: true,
				},
				"scope_contract",
			)
			if err != nil {
				return nil, err
			}
			obs = append(obs, o)
		}
	}
	if sc.IsCompleteEnoughForDecompose() {
		stmt := strings.TrimSpace(sc.GoalStatement)
		if stmt == "" && len(sc.InScope) > 0 {
			stmt = "ScopeIn: " + strings.Join(sc.InScope, ", ")
		}
		if stmt != "" {
			o, err := orchtypes.NewObservation(
				orchtypes.ObsFact,
				orchtypes.CatBusiness,
				0.9,
				orchtypes.FactPayload{Statement: stmt, Evidence: []string{item.ID, sessionID}},
				"scope_contract",
			)
			if err != nil {
				return nil, err
			}
			obs = append(obs, o)
		}
	}
	return obs, nil
}

func observationsFromChildSummaryBubbles(
	tm *workmodel.TaskManager,
	sessionID string,
	parent *workmodel.WorkItem,
) ([]orchtypes.Observation, error) {
	if tm == nil || parent == nil {
		return nil, nil
	}
	bubbles := workmodel.CollectSummaryChildBubbles(tm, sessionID, parent.ID)
	var obs []orchtypes.Observation
	for _, b := range bubbles {
		stmt := workmodel.SummaryBubbleStatement(b.ChildID, b.Summary)
		if stmt == "" {
			continue
		}
		o, err := orchtypes.NewObservation(
			orchtypes.ObsFact,
			orchtypes.CatBusiness,
			0.85,
			orchtypes.FactPayload{Statement: stmt, Evidence: []string{b.ChildID}},
			"context_summary_bubble",
		)
		if err != nil {
			return nil, err
		}
		obs = append(obs, o)
	}
	return obs, nil
}

func observationsFromChildStructuredBubbles(
	tm *workmodel.TaskManager,
	sessionID string,
	parent *workmodel.WorkItem,
) ([]orchtypes.Observation, error) {
	if tm == nil || parent == nil {
		return nil, nil
	}
	bubbles := workmodel.CollectStructuredChildBubbles(tm, sessionID, parent.ID)
	var obs []orchtypes.Observation
	for _, b := range bubbles {
		stmt := workmodel.StructuredBubbleStatement(b.ChildID, b.Round)
		if stmt == "" {
			continue
		}
		strength := 0.9
		if b.Round.UncertaintyMean > 0 {
			strength = 1 - b.Round.UncertaintyMean
			if strength < 0.1 {
				strength = 0.1
			}
		}
		o, err := orchtypes.NewObservation(
			orchtypes.ObsFact,
			orchtypes.CatBusiness,
			strength,
			orchtypes.FactPayload{Statement: stmt, Evidence: []string{b.ChildID, b.Round.VerdictID}},
			"context_structured_bubble",
		)
		if err != nil {
			return nil, err
		}
		obs = append(obs, o)
	}
	return obs, nil
}

func observationsFromChecklistChildBubbles(
	tm *workmodel.TaskManager,
	sessionID string,
	parent *workmodel.WorkItem,
) ([]orchtypes.Observation, error) {
	if tm == nil || parent == nil {
		return nil, nil
	}
	bubbles := workmodel.CollectChecklistChildBubbles(tm, sessionID, parent.ID)
	var obs []orchtypes.Observation
	for _, b := range bubbles {
		stmt := workmodel.ChecklistBubbleStatement(b.ChildID, b.Item)
		if stmt == "" {
			continue
		}
		o, err := orchtypes.NewObservation(
			orchtypes.ObsFact,
			orchtypes.CatBusiness,
			0.75,
			orchtypes.FactPayload{Statement: stmt, Evidence: []string{b.ChildID}},
			"context_checklist_bubble",
		)
		if err != nil {
			return nil, err
		}
		obs = append(obs, o)
	}
	return obs, nil
}

func quantizedKindFromIntent(kind orchtypes.IntentKind) string {
	switch kind {
	case orchtypes.IntentOrchestrate:
		return "intent_orchestrate"
	case orchtypes.IntentCommand:
		return "intent_command"
	case orchtypes.IntentFast:
		return "intent_fast"
	default:
		return "intent_skip"
	}
}
