package eval

import (
	"context"
	"time"
)

// --- 评测运行参数 ---

// EvalOpts 评测运行参数
type EvalOpts struct {
	DatasetPath  string        // 评测集路径
	Sampling     *SamplingOpts // 抽样配置（nil=全量）
	SaveBaseline bool          // 是否保存基线
}

// SamplingOpts 抽样配置
type SamplingOpts struct {
	MaxItems int `yaml:"max_items"` // 单次最多评多少条
}

// --- 评测报告 ---

// EvalReport 评测报告
type EvalReport struct {
	ID          string           `json:"id" yaml:"id"`
	DatasetID   string           `json:"dataset_id" yaml:"dataset_id"`
	RunAt       time.Time        `json:"run_at" yaml:"run_at"`
	JudgeModel  string           `json:"judge_model" yaml:"judge_model"`
	Scores      []DomainScore    `json:"scores" yaml:"scores"`
	Dashboard   ScoreDashboard   `json:"dashboard" yaml:"dashboard"`
	Delta       *EvalDelta       `json:"delta,omitempty" yaml:"delta,omitempty"`
	TuneSuggest []TuneSuggestion `json:"tune_suggest,omitempty" yaml:"tune_suggest,omitempty"`
}

// ScoreDashboard 评分面板（一次纵览）
type ScoreDashboard struct {
	OverallScore   float64            `json:"overall_score"`
	DimensionCount int                `json:"dimension_count"`
	ItemCount      int                `json:"item_count"`
	JudgeCost      TokenCost          `json:"judge_cost"`
	ByDomain       map[string]float64 `json:"by_domain"`
}

// TokenCost Judge 调用的 token 消耗
type TokenCost struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// --- 单维度评分 ---

// DomainScore 单维度评分
type DomainScore struct {
	Domain     string             `json:"domain" yaml:"domain"`
	Dimension  string             `json:"dimension" yaml:"dimension"`
	Score      float64            `json:"score" yaml:"score"`
	Confidence float64            `json:"confidence" yaml:"confidence"`
	Buckets    map[string]float64 `json:"buckets,omitempty" yaml:"buckets,omitempty"`
	Details    map[string]float64 `json:"details,omitempty" yaml:"details,omitempty"`
	JudgeLogs  []JudgeLog         `json:"judge_logs,omitempty" yaml:"judge_logs,omitempty"`
}

// JudgeLog 单次 Judge 调用的日志
type JudgeLog struct {
	ItemID    string    `json:"item_id"`
	Score     float64   `json:"score"`
	Reasoning string    `json:"reasoning"`
	Cost      TokenCost `json:"cost"`
}

// --- Delta 对比 ---

// EvalDelta 对比基线
type EvalDelta struct {
	BaselineID  string                `json:"baseline_id"`
	ByDimension map[string]DeltaEntry `json:"by_dimension"`
	ByBucket    map[string]DeltaEntry `json:"by_bucket"`
	Regressions []DeltaEntry          `json:"regressions"`
}

// DeltaEntry 单条 delta
type DeltaEntry struct {
	Dimension string  `json:"dimension"`
	Previous  float64 `json:"previous"`
	Current   float64 `json:"current"`
	Delta     float64 `json:"delta"`
	Severity  string  `json:"severity"` // "regression" | "improvement" | "stable"
}

const (
	SeverityRegression  = "regression"
	SeverityImprovement = "improvement"
	SeverityStable      = "stable"
)

// --- 调优建议 ---

// TuneSuggestion 调优建议
type TuneSuggestion struct {
	Target       string `json:"target"`
	Reason       string `json:"reason"`
	CurrentVal   string `json:"current_val"`
	SuggestedVal string `json:"suggested_val"`
	Confidence   string `json:"confidence"` // "high" | "medium" | "low"
}

const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

// --- Judge 相关 ---

// JudgeScore 单次 Judge 评分结果
type JudgeScore struct {
	Score      float64            `json:"score"`
	Confidence float64            `json:"confidence"`
	Reasoning  string             `json:"reasoning"`
	Details    map[string]float64 `json:"details,omitempty"`
	TokenUsage TokenCost          `json:"token_usage"`
}

// ScoreRubric Judge 评分指令
type ScoreRubric struct {
	Dimension   string `json:"dimension"`
	Instruction string `json:"instruction"`
	Scale       string `json:"scale"`    // "0-1", "1-5"
	Reference   string `json:"reference"` // 参考示例
}

// GoldLabel 人工标注的校准样本
type GoldLabel struct {
	ItemID     string            `json:"item_id"`
	HumanScore float64           `json:"human_score"`
	Reason     string            `json:"reason"`
	Tags       map[string]string `json:"tags,omitempty"`
}

// CalibrationReport 校准报告
type CalibrationReport struct {
	Kappa          float64   `json:"kappa"`
	JudgeModel     string    `json:"judge_model"`
	GoldSetSize    int       `json:"gold_set_size"`
	Passed         bool      `json:"passed"`
	LastCalibrated time.Time `json:"last_calibrated"`
}

// --- 评测集 ---

// EvalDataset 评测集
type EvalDataset struct {
	ID        string      `yaml:"id"`
	Version   string      `yaml:"version"`
	CreatedAt time.Time   `yaml:"created_at"`
	Buckets   []BucketDef `yaml:"buckets"`
	Items     []EvalItem  `yaml:"items"`
}

// BucketDef 分桶定义
type BucketDef struct {
	Name   string  `yaml:"name"`
	Weight float64 `yaml:"weight"`
}

// EvalItem 单条评测用例
type EvalItem struct {
	ID          string            `yaml:"id"`
	Bucket      string            `yaml:"bucket"`
	Domain      string            `yaml:"domain"`
	Dimension   string            `yaml:"dimension"`
	Input       map[string]any    `yaml:"input"`
	Expectation map[string]any    `yaml:"expectation"`
	RubricRef   string            `yaml:"rubric_ref"`
	Tags        map[string]string `yaml:"tags,omitempty"`
}

// --- Judge 接口 ---

// Judge 是评分器接口，JudgeManager 实现此接口。定义在 types.go 而非 judge.go
// 以供 probes 包和测试代码按接口 mock，不依赖 JudgeManager 具体实现。
type Judge interface {
	Score(ctx context.Context, item EvalItem, rubric ScoreRubric) (*JudgeScore, error)
	Calibrate(ctx context.Context, goldSet []GoldLabel, rubric ScoreRubric) (*CalibrationReport, error)
	RegisterRubric(rubric ScoreRubric)
	Disputes() []ScoreDispute
	ResolveDispute(itemID string, finalScore *JudgeScore)
}

// --- 配置 ---

// EvalConfig 评测配置
type EvalConfig struct {
	Enabled     bool             `yaml:"enabled"`
	Judge       JudgeConfig      `yaml:"judge"`
	Dataset     DatasetConfig    `yaml:"dataset"`
	Sampling    SamplingConfig   `yaml:"sampling"`
	Calibration CalibrationConfig `yaml:"calibration"`
}

// JudgeConfig Judge 配置
type JudgeConfig struct {
	Provider         string  `yaml:"provider"`
	Model            string  `yaml:"model"`
	FallbackProvider string  `yaml:"fallback_provider"`
	Temperature      float64 `yaml:"temperature"`
	MaxTokens        int     `yaml:"max_tokens"`
}

// DatasetConfig 评测集配置
type DatasetConfig struct {
	Path string `yaml:"path"`
}

// SamplingConfig 抽样配置
type SamplingConfig struct {
	Enabled  bool `yaml:"enabled"`
	MaxItems int  `yaml:"max_items"`
}

// CalibrationConfig 校准配置
type CalibrationConfig struct {
	Enabled    bool    `yaml:"enabled"`
	MinKappa   float64 `yaml:"min_kappa"`
	GoldSetPath string `yaml:"gold_set_path"`
}
