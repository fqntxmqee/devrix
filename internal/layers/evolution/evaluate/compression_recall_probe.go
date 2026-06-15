package evaluate

import (
	"context"
	"fmt"
	"strings"
)

func init() {
	RegisterProbe(&CompressionRecallProbe{})
}

// CompressionRecallProbe 评测 compression 前后的事实保留率。
type CompressionRecallProbe struct{}

func (p *CompressionRecallProbe) ID() string {
	return "compression_recall"
}

func (p *CompressionRecallProbe) Run(ctx context.Context, item EvalItem, jm Judge) (*DomainScore, error) {
	rubric := ScoreRubric{
		Dimension:   p.ID(),
		Instruction: "Evaluate whether ALL key facts from the original context are preserved in the compressed version. Focus on factual completeness, not phrasing.",
		Scale:       "0-1",
		Reference:   "A score of 1.0 means every key fact is fully preserved. A score of 0.0 means no key facts survived compression.",
	}

	judgeScore, err := jm.Score(ctx, item, rubric)
	if err != nil {
		return nil, fmt.Errorf("compression recall judge score: %w", err)
	}

	recallF1 := judgeScore.Score
	details := map[string]float64{
		"judge_score": judgeScore.Score,
	}

	if mustKeep, ok := item.Expectation["must_keep"].([]any); ok && len(mustKeep) > 0 {
		compressed := ""
		if c, ok := item.Input["compressed"].(string); ok {
			compressed = c
		}

		kept := 0
		for _, fact := range mustKeep {
			factStr, ok := fact.(string)
			if !ok {
				continue
			}
			if stringContains(compressed, factStr) {
				kept++
			}
		}

		recall := float64(kept) / float64(len(mustKeep))
		details["recall"] = recall
		details["must_keep_count"] = float64(len(mustKeep))
		details["kept_count"] = float64(kept)

		if recall < judgeScore.Score {
			recallF1 = recall
		}
	}

	buckets := map[string]float64{}
	if item.Bucket != "" {
		buckets[item.Bucket] = recallF1
	}

	return &DomainScore{
		Domain:     "d2",
		Dimension:  p.ID(),
		Score:      recallF1,
		Confidence: judgeScore.Confidence,
		Buckets:    buckets,
		Details:    details,
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

func stringContains(s, substr string) bool {
	return len(substr) > 0 && strings.Contains(s, substr)
}
