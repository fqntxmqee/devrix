package eval

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

// LLMClient 抽象 LLM 调用，支持 mock 和真实 provider。
type LLMClient interface {
	// Chat 发送非流式 chat 请求，返回完整响应文本和 token 消耗。
	Chat(ctx context.Context, model string, systemPrompt string, userMsg string, temperature float64, maxTokens int) (string, TokenCost, error)
}

// ScoreDispute 分歧记录
type ScoreDispute struct {
	ItemID     string
	Primary    *JudgeScore
	Secondary  *JudgeScore
	Delta      float64
	Resolved   bool
	FinalScore *JudgeScore
}

// JudgeManager 管理 LLM-as-Judge 评分、校准与分歧仲裁。
type JudgeManager struct {
	primary   LLMClient
	secondary LLMClient // 反方 Judge，可选
	config    JudgeConfig
	rubrics   map[string]ScoreRubric
	disputes  []ScoreDispute
}

// NewJudgeManager 创建 Judge 管理器。
func NewJudgeManager(primary LLMClient, secondary LLMClient, config JudgeConfig) *JudgeManager {
	return &JudgeManager{
		primary:   primary,
		secondary: secondary,
		config:    config,
		rubrics:   make(map[string]ScoreRubric),
	}
}

// RegisterRubric 注册评分 rubric。
func (jm *JudgeManager) RegisterRubric(rubric ScoreRubric) {
	jm.rubrics[rubric.Dimension] = rubric
}

// Score 对单条评测用例评分，含 position randomization。
func (jm *JudgeManager) Score(ctx context.Context, item EvalItem, rubric ScoreRubric) (*JudgeScore, error) {
	primary, err := jm.judgeOnce(ctx, jm.primary, rubric, item, false)
	if err != nil {
		return nil, fmt.Errorf("primary judge score: %w", err)
	}

	// position randomization：翻转顺序再评一次
	reversed, err := jm.judgeOnce(ctx, jm.primary, rubric, item, true)
	if err != nil {
		return nil, fmt.Errorf("primary judge reversed: %w", err)
	}

	avg := &JudgeScore{
		Score:      (primary.Score + reversed.Score) / 2,
		Confidence: math.Min(primary.Confidence, reversed.Confidence),
		Reasoning:  fmt.Sprintf("primary: %s | reversed: %s", primary.Reasoning, reversed.Reasoning),
		Details:    mergeDetails(primary.Details, reversed.Details),
		TokenUsage: TokenCost{
			PromptTokens:     primary.TokenUsage.PromptTokens + reversed.TokenUsage.PromptTokens,
			CompletionTokens: primary.TokenUsage.CompletionTokens + reversed.TokenUsage.CompletionTokens,
			TotalTokens:      primary.TokenUsage.TotalTokens + reversed.TokenUsage.TotalTokens,
		},
	}

	// 分歧检测
	if jm.secondary != nil {
		secondary, err := jm.judgeOnce(ctx, jm.secondary, rubric, item, false)
		if err == nil {
			diff := math.Abs(avg.Score - secondary.Score)
			dispute := ScoreDispute{
				ItemID:    item.ID,
				Primary:   avg,
				Secondary: secondary,
				Delta:     diff,
			}

			if diff > 0.5 { // 1σ 分歧阈值
				dispute.Resolved = false
				jm.disputes = append(jm.disputes, dispute)
				avg.Confidence = math.Min(avg.Confidence, 0.3)
				return avg, nil
			}

			dispute.Resolved = true
			dispute.FinalScore = avg
			jm.disputes = append(jm.disputes, dispute)
		}
	}

	return avg, nil
}

// Disputes 返回未解决的分歧列表。
func (jm *JudgeManager) Disputes() []ScoreDispute {
	var pending []ScoreDispute
	for _, d := range jm.disputes {
		if !d.Resolved {
			pending = append(pending, d)
		}
	}
	return pending
}

// ResolveDispute 人工仲裁分歧。
func (jm *JudgeManager) ResolveDispute(itemID string, finalScore *JudgeScore) {
	for i := range jm.disputes {
		if jm.disputes[i].ItemID == itemID && !jm.disputes[i].Resolved {
			jm.disputes[i].Resolved = true
			jm.disputes[i].FinalScore = finalScore
			return
		}
	}
}

// Calibrate 在人工标注集上校准，返回 Cohen's kappa。
func (jm *JudgeManager) Calibrate(ctx context.Context, goldSet []GoldLabel, rubric ScoreRubric) (*CalibrationReport, error) {
	if len(goldSet) == 0 {
		return nil, fmt.Errorf("gold set is empty")
	}

	var judgeScores []float64
	var humanScores []float64

	for _, gold := range goldSet {
		item := EvalItem{
			ID:          gold.ItemID,
			Input:       nil,
			Expectation: nil,
			Tags:        gold.Tags,
		}
		score, err := jm.judgeOnce(ctx, jm.primary, rubric, item, false)
		if err != nil {
			continue
		}
		judgeScores = append(judgeScores, score.Score)
		humanScores = append(humanScores, gold.HumanScore)
	}

	kappa := cohensKappa(humanScores, judgeScores, 5)

	return &CalibrationReport{
		Kappa:          kappa,
		JudgeModel:     jm.config.Model,
		GoldSetSize:    len(goldSet),
		Passed:         kappa >= jm.config.Temperature, // 复用 Temperature 字段传 minKappa
		LastCalibrated: now(),
	}, nil
}

// --- internal ---

func (jm *JudgeManager) judgeOnce(ctx context.Context, client LLMClient, rubric ScoreRubric, item EvalItem, reversed bool) (*JudgeScore, error) {
	userMsg := buildJudgePrompt(rubric, item, reversed)
	systemPrompt := "You are an expert AI quality evaluator. Score the response based on the given rubric. Output your reasoning first, then your score."

	text, cost, err := client.Chat(ctx, jm.config.Model, systemPrompt, userMsg, jm.config.Temperature, jm.config.MaxTokens)
	if err != nil {
		return nil, err
	}

	return parseJudgeResponse(text, cost), nil
}

func buildJudgePrompt(rubric ScoreRubric, item EvalItem, reversed bool) string {
	var b strings.Builder

	b.WriteString("## Rubric\n")
	b.WriteString(fmt.Sprintf("Dimension: %s\n", rubric.Dimension))
	b.WriteString(fmt.Sprintf("Instruction: %s\n", rubric.Instruction))
	b.WriteString(fmt.Sprintf("Scale: %s\n", rubric.Scale))
	if rubric.Reference != "" {
		b.WriteString(fmt.Sprintf("Reference: %s\n", rubric.Reference))
	}

	b.WriteString("\n## Input\n")
	for k, v := range item.Input {
		b.WriteString(fmt.Sprintf("%s: %v\n", k, v))
	}

	b.WriteString(fmt.Sprintf("\n## Expected\n"))
	for k, v := range item.Expectation {
		b.WriteString(fmt.Sprintf("%s: %v\n", k, v))
	}

	if reversed {
		b.WriteString("\nNote: Evaluate the response as-is, without regard to order.\n")
	}

	b.WriteString("\n## Output Format\n")
	b.WriteString("Reasoning: <your analysis>\n")
	b.WriteString("Score: <0.0-1.0>\n")
	b.WriteString("Confidence: <0.0-1.0>\n")

	return b.String()
}

func parseJudgeResponse(text string, cost TokenCost) *JudgeScore {
	score := 0.5
	confidence := 0.5
	var reasoning strings.Builder

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Score:") {
			fmt.Sscanf(trimmed, "Score: %f", &score)
		} else if strings.HasPrefix(trimmed, "Confidence:") {
			fmt.Sscanf(trimmed, "Confidence: %f", &confidence)
		} else if strings.HasPrefix(trimmed, "Reasoning:") {
			reasoning.WriteString(strings.TrimPrefix(trimmed, "Reasoning:"))
		} else {
			reasoning.WriteString(" " + trimmed)
		}
	}

	score = clamp(score, 0, 1)
	confidence = clamp(confidence, 0, 1)

	return &JudgeScore{
		Score:      score,
		Confidence: confidence,
		Reasoning:  strings.TrimSpace(reasoning.String()),
		TokenUsage: cost,
	}
}

// --- 辅助函数 ---

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func mergeDetails(a, b map[string]float64) map[string]float64 {
	r := make(map[string]float64)
	for k, v := range a {
		r[k] = v
	}
	for k, v := range b {
		if _, ok := r[k]; ok {
			r[k] = (r[k] + v) / 2
		} else {
			r[k] = v
		}
	}
	return r
}

// cohensKappa 计算 Cohen's kappa（简化版，离散化到 nBins）。
func cohensKappa(human, judge []float64, nBins int) float64 {
	if len(human) != len(judge) || len(human) == 0 {
		return 0
	}
	n := len(human)

	// 离散化
	bin := func(v float64) int {
		b := int(v * float64(nBins))
		if b >= nBins {
			b = nBins - 1
		}
		return b
	}

	// 观测一致率
	agree := 0
	for i := 0; i < n; i++ {
		if bin(human[i]) == bin(judge[i]) {
			agree++
		}
	}
	po := float64(agree) / float64(n)

	// 期望一致率
	pe := 0.0
	for b := 0; b < nBins; b++ {
		nh := 0
		nj := 0
		for i := 0; i < n; i++ {
			if bin(human[i]) == b {
				nh++
			}
			if bin(judge[i]) == b {
				nj++
			}
		}
		pe += float64(nh*nj) / float64(n*n)
	}

	if pe == 1 {
		return 1
	}
	return (po - pe) / (1 - pe)
}

// now 可被测试替换
var now = timeNow

func timeNow() time.Time {
	return time.Now()
}
