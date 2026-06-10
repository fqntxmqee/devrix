package eval

import (
	"context"
	"fmt"
)

func init() {
	RegisterProbe(&ProviderQualityProbe{})
}

// ProviderQualityProbe 评测 provider 响应的语义一致性与指令遵循率。
type ProviderQualityProbe struct{}

func (p *ProviderQualityProbe) ID() string {
	return "provider_quality"
}

func (p *ProviderQualityProbe) Run(ctx context.Context, item EvalItem, jm Judge) (*DomainScore, error) {
	responseA := stringFromInput(item.Input, "response_a")
	responseB := stringFromInput(item.Input, "response_b")
	mustFollow := stringSliceFromExpectation(item.Expectation, "must_follow")

	followA := instructionFollowingRate(responseA, mustFollow)
	followB := instructionFollowingRate(responseB, mustFollow)
	semanticSim := wordJaccard(responseA, responseB)
	instructionAvg := avgScore(followA, followB)

	deterministic := conservativeMin(semanticSim, instructionAvg)

	rubric := ScoreRubric{
		Dimension: p.ID(),
		Instruction: "Evaluate semantic consistency between two LLM provider responses to the same prompt, " +
			"and whether each response follows the required instructions.",
		Scale:     "0-1",
		Reference: "1.0 = semantically equivalent and fully instruction-compliant; 0.0 = unrelated or non-compliant.",
	}

	judgeScore, err := jm.Score(ctx, item, rubric)
	if err != nil {
		return nil, fmt.Errorf("provider quality judge score: %w", err)
	}

	finalScore := judgeScore.Score
	if deterministic < finalScore {
		finalScore = deterministic
	}

	buckets := map[string]float64{}
	if item.Bucket != "" {
		buckets[item.Bucket] = finalScore
	}

	return &DomainScore{
		Domain:     "d3",
		Dimension:  p.ID(),
		Score:      finalScore,
		Confidence: judgeScore.Confidence,
		Buckets:    buckets,
		Details: map[string]float64{
			"judge_score":           judgeScore.Score,
			"semantic_similarity":   semanticSim,
			"instruction_following_a": followA,
			"instruction_following_b": followB,
			"instruction_following": instructionAvg,
		},
		JudgeLogs: []JudgeLog{
			{
				ItemID:    item.ID,
				Score:     judgeScore.Score,
				Reasoning: judgeScore.Reasoning,
				Cost:      judgeScore.TokenUsage,
			},
		},
	}, nil
}
