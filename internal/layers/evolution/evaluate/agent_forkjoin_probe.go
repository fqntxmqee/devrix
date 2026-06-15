package evaluate

import (
	"context"
	"fmt"
)

func init() {
	RegisterProbe(&AgentForkJoinProbe{})
}

// AgentForkJoinProbe 评测多 Agent Fork/Join 的消息隔离与结果合并质量。
type AgentForkJoinProbe struct{}

func (p *AgentForkJoinProbe) ID() string {
	return "agent_forkjoin"
}

func (p *AgentForkJoinProbe) Run(ctx context.Context, item EvalItem, jm Judge) (*DomainScore, error) {
	msgsA := messagesFromInput(item.Input, "agent_a_messages")
	msgsB := messagesFromInput(item.Input, "agent_b_messages")
	joinResult := stringFromInput(item.Input, "join_result")

	mustInclude := stringSliceFromExpectation(item.Expectation, "must_include_in_join")
	forbiddenA := stringSliceFromExpectation(item.Expectation, "agent_a_forbidden")
	forbiddenB := stringSliceFromExpectation(item.Expectation, "agent_b_forbidden")

	isolationA := isolationRate(msgsA, forbiddenA)
	isolationB := isolationRate(msgsB, forbiddenB)
	joinCompleteness := instructionFollowingRate(joinResult, mustInclude)

	deterministic := conservativeMin(isolationA, isolationB, joinCompleteness)

	rubric := ScoreRubric{
		Dimension: p.ID(),
		Instruction: "Evaluate fork/join multi-agent coordination: sub-agents must not leak each other's " +
			"private context, and the join result must include all required sub-agent outputs without hallucination.",
		Scale:     "0-1",
		Reference: "1.0 = perfect isolation and complete join; 0.0 = cross-contamination or missing join outputs.",
	}

	judgeScore, err := jm.Score(ctx, item, rubric)
	if err != nil {
		return nil, fmt.Errorf("agent forkjoin judge score: %w", err)
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
		Domain:     "d4",
		Dimension:  p.ID(),
		Score:      finalScore,
		Confidence: judgeScore.Confidence,
		Buckets:    buckets,
		Details: map[string]float64{
			"judge_score":       judgeScore.Score,
			"isolation_a":       isolationA,
			"isolation_b":       isolationB,
			"join_completeness": joinCompleteness,
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
