package sessionorchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/shared/types"
)

const llmObservationSystemPrompt = `You propose structured observations for an orchestration Observe node.
Return ONLY a JSON array (no markdown). Each element:
{"kind":"obs_fact|obs_signal|obs_uncertainty|obs_deviation","strength":0.0-1.0,"statement":"...","question":"...","evidence":["wi_id"]}

Rules:
- Use ONLY the provided directive and structured signals; do not invent tool outputs.
- Prefer obs_uncertainty when scope is unclear; obs_fact only when strongly supported.
- Maximum 3 proposals. Empty array [] is valid.`

// LLMObservationProposer calls D3 via LLMInvoker to propose Obs* candidates (T35).
type LLMObservationProposer struct {
	LLM orchtypes.LLMInvoker
}

// NewLLMObservationProposer constructs an LLM-backed proposer.
func NewLLMObservationProposer(llm orchtypes.LLMInvoker) *LLMObservationProposer {
	if llm == nil {
		return nil
	}
	return &LLMObservationProposer{LLM: llm}
}

func (p *LLMObservationProposer) ProposeObservations(ctx context.Context, in ObserveSignalInput) ([]ObservationProposal, error) {
	if p == nil || p.LLM == nil {
		return nil, nil
	}
	user := buildLLMObservationUserPrompt(in)
	ch, err := p.LLM.InvokeStream(ctx, orchtypes.LLMInvokeRequest{
		SessionID:    in.SessionID,
		SystemPrompt: llmObservationSystemPrompt,
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
	var b strings.Builder
	fmt.Fprintf(&b, "work_item_id: %s\n", in.WorkItemID)
	fmt.Fprintf(&b, "directive: %s\n", in.Directive)
	if in.PriorMean > 0 {
		fmt.Fprintf(&b, "prior_mean: %.3f\n", in.PriorMean)
	}
	if in.ScopeContract != nil {
		if s := strings.TrimSpace(in.ScopeContract.GoalStatement); s != "" {
			fmt.Fprintf(&b, "scope_goal: %s\n", s)
		}
		for _, q := range in.ScopeContract.OpenQuestions {
			if strings.TrimSpace(q) != "" {
				fmt.Fprintf(&b, "scope_open_question: %s\n", q)
			}
		}
	}
	for _, line := range in.InboundSignalLines {
		fmt.Fprintf(&b, "signal: %s\n", line)
	}
	return b.String()
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
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}
	var rows []rawObsProposal
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil, fmt.Errorf("parse observation proposals: %w", err)
	}
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
	return out, nil
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
