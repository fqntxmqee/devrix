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
