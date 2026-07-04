package sessionorchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/prompttags"
	"github.com/devrix/devrix/internal/shared/types"
)

// LLMObservationProposer calls D2 MaterializeForMUPS then D3 LLMInvoker (DM-20260704-001).
type LLMObservationProposer struct {
	LLM    orchtypes.LLMInvoker
	MUPS   contracts.IMUPSContextMaterializer
	Locale i18n.Locale
}

// NewLLMObservationProposer constructs a D2→D3 observation proposer.
func NewLLMObservationProposer(llm orchtypes.LLMInvoker, mups contracts.IMUPSContextMaterializer, loc i18n.Locale) *LLMObservationProposer {
	if llm == nil || mups == nil {
		return nil
	}
	if loc == "" {
		loc = i18n.DefaultLocale
	}
	return &LLMObservationProposer{LLM: llm, MUPS: mups, Locale: loc}
}

func (p *LLMObservationProposer) ProposeObservations(ctx context.Context, in ObserveSignalInput) ([]ObservationProposal, error) {
	if p == nil || p.LLM == nil || p.MUPS == nil {
		return nil, nil
	}
	req := buildObserveMUPSRequest(in, string(p.Locale))
	prepared, err := p.MUPS.MaterializeForMUPS(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("observe proposer: d2 materialize: %w", err)
	}
	systemPrompt := mergeMUPSPreparedSystem(prepared)
	user := buildLLMObservationUserPrompt(in)
	ch, err := p.LLM.InvokeStream(ctx, orchtypes.LLMInvokeRequest{
		SessionID:    in.SessionID,
		SystemPrompt: systemPrompt,
		Messages: []types.Message{{
			SessionID: in.SessionID,
			Role:      types.MessageRoleUser,
			Content:   user,
		}},
	})
	if err != nil {
		return nil, err
	}
	raw := collectLLMText(ch)
	return parseObservationProposalsJSON(raw)
}

func buildLLMObservationUserPrompt(in ObserveSignalInput) string {
	fields := map[prompttags.TagName]any{
		prompttags.TagWorkItemID: in.WorkItemID,
		prompttags.TagDirective:  in.Directive,
	}
	if in.PriorMean > 0 {
		fields[prompttags.TagPriorMean] = in.PriorMean
	}
	if in.ScopeContract != nil {
		if s := strings.TrimSpace(in.ScopeContract.GoalStatement); s != "" {
			fields[prompttags.TagScopeGoal] = s
		}
		var questions []string
		for _, q := range in.ScopeContract.OpenQuestions {
			if strings.TrimSpace(q) != "" {
				questions = append(questions, q)
			}
		}
		if len(questions) > 0 {
			fields[prompttags.TagScopeOpenQuestion] = questions
		}
	}
	if len(in.InboundSignalLines) > 0 {
		fields[prompttags.TagSignal] = in.InboundSignalLines
	}
	return prompttags.BuildLineFrame(prompttags.ObserveUserFrame, fields)
}

func collectLLMText(ch <-chan llmgateway.Chunk) string {
	var b strings.Builder
	for chunk := range ch {
		if chunk.Content != "" {
			b.WriteString(chunk.Content)
		}
	}
	return strings.TrimSpace(b.String())
}

type rawObsProposal struct {
	Kind      string   `json:"kind"`
	Strength  float64  `json:"strength"`
	Statement string   `json:"statement"`
	Question  string   `json:"question"`
	Evidence  []string `json:"evidence"`
}

func parseObservationProposalsJSON(raw string) ([]ObservationProposal, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil, nil
	}
	if rows, ok := prompttags.ParseWholeBody[[]rawObsProposal](raw); ok {
		return mapRawObsProposals(rows), nil
	}
	var rows []rawObsProposal
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	candidate := raw
	if start >= 0 && end > start {
		candidate = raw[start : end+1]
	}
	if err := json.Unmarshal([]byte(candidate), &rows); err != nil {
		return nil, fmt.Errorf("parse observation proposals: %w", err)
	}
	return mapRawObsProposals(rows), nil
}

func mapRawObsProposals(rows []rawObsProposal) []ObservationProposal {
	out := make([]ObservationProposal, 0, len(rows))
	for _, r := range rows {
		kind, ok := mapRawObsKind(r.Kind)
		if !ok {
			continue
		}
		out = append(out, ObservationProposal{
			Kind:      kind,
			Category:  orchtypes.CatBusiness,
			Strength:  r.Strength,
			Statement: r.Statement,
			Question:  r.Question,
			Evidence:  append([]string(nil), r.Evidence...),
		})
	}
	return out
}

func mapRawObsKind(s string) (orchtypes.ObservationKind, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "obs_fact", "fact":
		return orchtypes.ObsFact, true
	case "obs_signal", "signal":
		return orchtypes.ObsSignal, true
	case "obs_deviation", "deviation":
		return orchtypes.ObsDeviation, true
	case "obs_uncertainty", "uncertainty":
		return orchtypes.ObsUncertainty, true
	default:
		return 0, false
	}
}
