package guard

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/types"
)

// RuntimeJudge wraps the D3 LLM Gateway for cross-model validation of routing decisions.
type RuntimeJudge struct {
	gw     llmgateway.IGateway
	config OrchestrationConfig
}

// NewRuntimeJudge creates a judge that routes validation prompts through the LLM capture.
func NewRuntimeJudge(gw llmgateway.IGateway, config OrchestrationConfig) *RuntimeJudge {
	return &RuntimeJudge{gw: gw, config: config}
}

// ValidateDecision sends the decision to the judge LLM and returns a verdict.
func (rj *RuntimeJudge) ValidateDecision(ctx context.Context, rec DecisionRecord) (*ValidationResult, error) {
	prompt := rj.buildJudgePrompt(rec)
	req := &llmgateway.Request{
		Provider:     rj.config.JudgeProvider,
		Model:        rj.config.JudgeModel,
		SystemPrompt: "You are a routing decision validator. Respond in JSON only.",
		Messages: []types.Message{
			{Role: "user", Content: prompt},
		},
		Stream: false,
	}

	start := time.Now()
	ch, err := rj.gw.Stream(ctx, req)
	if err != nil {
		return rj.tryFallback(ctx, rec, err)
	}

	var response strings.Builder
	for chunk := range ch {
		response.WriteString(chunk.Content)
	}
	duration := time.Since(start)

	result := rj.parseResponse(response.String())
	result.DecisionID = rec.ID
	result.JudgeModel = rj.config.JudgeModel
	result.Duration = duration
	return result, nil
}

func (rj *RuntimeJudge) tryFallback(ctx context.Context, rec DecisionRecord, originalErr error) (*ValidationResult, error) {
	if rj.config.FallbackJudgeProvider == "" || rj.config.FallbackJudgeModel == "" {
		return nil, fmt.Errorf("judge call failed and no fallback configured: %w", originalErr)
	}

	req := &llmgateway.Request{
		Provider:     rj.config.FallbackJudgeProvider,
		Model:        rj.config.FallbackJudgeModel,
		SystemPrompt: "You are a routing decision validator. Respond in JSON only.",
		Messages: []types.Message{
			{Role: "user", Content: rj.buildJudgePrompt(rec)},
		},
		Stream: false,
	}

	ch, err := rj.gw.Stream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("judge + fallback both failed: %w", err)
	}

	var response strings.Builder
	for chunk := range ch {
		response.WriteString(chunk.Content)
	}

	result := rj.parseResponse(response.String())
	result.DecisionID = rec.ID
	result.JudgeModel = rj.config.FallbackJudgeModel
	return result, nil
}

func (rj *RuntimeJudge) buildJudgePrompt(rec DecisionRecord) string {
	return fmt.Sprintf(`Evaluate this agent routing decision:

Decision: %s
Risk Level: %d
Tool: %s
Tool Input: %s
Target Agent: %s
Category: %s

Respond with JSON:
{"valid": true/false, "confidence": 0.0-1.0, "reasoning": "...", "suggested_action": "none|terminate|reroute|deny", "suggested_agent_id": "..."}`,
		rec.Category, rec.RiskClass, rec.ToolName, rec.ToolInput,
		rec.TargetAgentID, rec.Category)
}

func (rj *RuntimeJudge) parseResponse(raw string) *ValidationResult {
	var resp struct {
		Valid           bool    `json:"valid"`
		Confidence      float64 `json:"confidence"`
		Reason          string  `json:"reason"`
		Reasoning       string  `json:"reasoning"`
		SuggestedAction string  `json:"suggested_action"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return &ValidationResult{
			Valid:           false,
			Confidence:      0.0,
			SuggestedAction: "none",
			Reasoning:       fmt.Sprintf("parse error: %v", err),
		}
	}
	reasoning := resp.Reason
	if reasoning == "" {
		reasoning = resp.Reasoning
	}
	return &ValidationResult{
		Valid:           resp.Valid,
		Confidence:      resp.Confidence,
		Reasoning:       reasoning,
		SuggestedAction: resp.SuggestedAction,
	}
}
