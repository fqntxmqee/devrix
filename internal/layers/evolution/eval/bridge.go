// Package eval is a backward-compatibility bridge.
//
// Deprecated: use github.com/devrix/devrix/internal/layers/evolution/evaluate instead.
// This bridge will be removed in v2.1.
package eval

import "github.com/devrix/devrix/internal/layers/evolution/evaluate"

// Types — config & options

type (
	EvalOpts           = evaluate.EvalOpts
	SamplingOpts       = evaluate.SamplingOpts
	EvalConfig         = evaluate.EvalConfig
	JudgeConfig        = evaluate.JudgeConfig
	DatasetConfig      = evaluate.DatasetConfig
	SamplingConfig     = evaluate.SamplingConfig
	CalibrationConfig  = evaluate.CalibrationConfig
)

// Types — report & scoring

type (
	EvalReport         = evaluate.EvalReport
	ScoreDashboard     = evaluate.ScoreDashboard
	TokenCost          = evaluate.TokenCost
	DomainScore        = evaluate.DomainScore
	JudgeLog           = evaluate.JudgeLog
	EvalDelta          = evaluate.EvalDelta
	DeltaEntry         = evaluate.DeltaEntry
	TuneSuggestion     = evaluate.TuneSuggestion
	JudgeScore         = evaluate.JudgeScore
	ScoreRubric        = evaluate.ScoreRubric
	GoldLabel          = evaluate.GoldLabel
	CalibrationReport  = evaluate.CalibrationReport
)

// Types — dataset

type (
	EvalDataset        = evaluate.EvalDataset
	BucketDef          = evaluate.BucketDef
	EvalItem           = evaluate.EvalItem
)

// Types — engine & judge

type (
	EvalEngine         = evaluate.EvalEngine
	JudgeManager       = evaluate.JudgeManager
	LLMClient          = evaluate.LLMClient
	ScoreDispute       = evaluate.ScoreDispute
	Judge              = evaluate.Judge
)

// Types — analysis

type (
	DeltaAnalyzer      = evaluate.DeltaAnalyzer
	GateResult         = evaluate.GateResult
	GateError          = evaluate.GateError
	TuneGenerator      = evaluate.TuneGenerator
)

// Types — LLM clients

type (
	GatewayLLMClient   = evaluate.GatewayLLMClient
	StaticLLMClient    = evaluate.StaticLLMClient
)

// Types — probes

type (
	CompressionRecallProbe          = evaluate.CompressionRecallProbe
	AgentForkJoinProbe              = evaluate.AgentForkJoinProbe
	ProviderQualityProbe            = evaluate.ProviderQualityProbe
	ToolAccuracyProbe               = evaluate.ToolAccuracyProbe
	LayerViolationProbe             = evaluate.LayerViolationProbe
	PathRegressionProbe             = evaluate.PathRegressionProbe
	SessionIsolationProbe           = evaluate.SessionIsolationProbe
	SafetyLatencyProbe              = evaluate.SafetyLatencyProbe
	TierResolutionProbe             = evaluate.TierResolutionProbe
	BreakerAnomalyTransitionProbe   = evaluate.BreakerAnomalyTransitionProbe
)

// Functions — engine & judge

var (
	NewEvalEngine    = evaluate.NewEvalEngine
	NewJudgeManager  = evaluate.NewJudgeManager
)

// Functions — dataset

var (
	LoadDataset         = evaluate.LoadDataset
	LoadDatasetVersion  = evaluate.LoadDatasetVersion
	StratifiedSample    = evaluate.StratifiedSample
	SaveBaseline        = evaluate.SaveBaseline
	LoadBaseline        = evaluate.LoadBaseline
)

// Functions — probes

var (
	RegisterProbe  = evaluate.RegisterProbe
	GetProbe       = evaluate.GetProbe
)

// Functions — analysis

var (
	NewDeltaAnalyzer   = evaluate.NewDeltaAnalyzer
	CheckDeltaGate     = evaluate.CheckDeltaGate
	FormatDeltaSummary = evaluate.FormatDeltaSummary
	NewTuneGenerator   = evaluate.NewTuneGenerator
)

// Functions — LLM clients

var (
	NewGatewayLLMClient  = evaluate.NewGatewayLLMClient
	NewStaticLLMClient   = evaluate.NewStaticLLMClient
)
