package eval

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

// EvalEngine 评测引擎，负责编排评测流程。
type EvalEngine struct {
	config        EvalConfig
	judge         *JudgeManager
	deltaAnalyzer *DeltaAnalyzer
	probeRegistry map[string]Probe // 可注入自定义探针，nil 时使用默认注册表
}

// NewEvalEngine 创建评测引擎。
func NewEvalEngine(config EvalConfig, judge *JudgeManager) *EvalEngine {
	return &EvalEngine{
		config: config,
		judge:  judge,
	}
}

// WithProbeRegistry 注入自定义探针注册表。
func (e *EvalEngine) WithProbeRegistry(registry map[string]Probe) *EvalEngine {
	e.probeRegistry = registry
	return e
}

// WithBaseline 设置基线用于 delta 对比。
func (e *EvalEngine) WithBaseline(baseline *EvalReport) *EvalEngine {
	e.deltaAnalyzer = NewDeltaAnalyzer(baseline)
	return e
}

// Run 执行一次评测。
func (e *EvalEngine) Run(ctx context.Context, opts EvalOpts) (*EvalReport, error) {
	if !e.config.Enabled {
		return nil, nil
	}

	// 1. 加载评测集
	datasetPath := opts.DatasetPath
	if datasetPath == "" {
		datasetPath = e.config.Dataset.Path
	}
	ds, err := e.loadDataset(datasetPath)
	if err != nil {
		return nil, fmt.Errorf("load dataset: %w", err)
	}

	// 2. 抽样
	items := ds.Items
	if opts.Sampling != nil && opts.Sampling.MaxItems > 0 && opts.Sampling.MaxItems < len(items) {
		items = StratifiedSample(items, opts.Sampling.MaxItems)
	} else if e.config.Sampling.Enabled && e.config.Sampling.MaxItems > 0 && e.config.Sampling.MaxItems < len(items) {
		items = StratifiedSample(items, e.config.Sampling.MaxItems)
	}

	// 3. 逐项评测
	var mu sync.Mutex
	scoresByDimension := make(map[string]*aggregatedScore)
	totalCost := TokenCost{}

	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("eval cancelled: %w", err)
		}

		probe := e.findProbe(item.Dimension)
		if probe == nil {
			continue
		}

		domainScore, err := probe.Run(ctx, item, e.judge)
		if err != nil {
			return nil, fmt.Errorf("probe %s item %s: %w", item.Dimension, item.ID, err)
		}

		mu.Lock()
		key := domainScore.Domain + "." + domainScore.Dimension
		agg, ok := scoresByDimension[key]
		if !ok {
			agg = &aggregatedScore{
				domain:    domainScore.Domain,
				dimension: domainScore.Dimension,
			}
			scoresByDimension[key] = agg
		}
		agg.add(domainScore)
		if len(domainScore.JudgeLogs) > 0 {
			totalCost.PromptTokens += domainScore.JudgeLogs[0].Cost.PromptTokens
			totalCost.CompletionTokens += domainScore.JudgeLogs[0].Cost.CompletionTokens
			totalCost.TotalTokens += domainScore.JudgeLogs[0].Cost.TotalTokens
		}
		mu.Unlock()
	}

	// 4. 聚合评分
	var finalScores []DomainScore
	byDomain := make(map[string]float64)
	for _, agg := range scoresByDimension {
		finalScores = append(finalScores, agg.finalize())
		if _, ok := byDomain[agg.domain]; !ok {
			byDomain[agg.domain] = agg.totalScore / float64(agg.count)
		} else {
			byDomain[agg.domain] = (byDomain[agg.domain] + agg.totalScore/float64(agg.count)) / 2
		}
	}

	overallScore := 0.0
	if len(finalScores) > 0 {
		total := 0.0
		for _, s := range finalScores {
			total += s.Score
		}
		overallScore = total / float64(len(finalScores))
	}

	report := &EvalReport{
		ID:         fmt.Sprintf("eval-%s", strings.ReplaceAll(now().Format("20060102150405"), " ", "")),
		DatasetID:  ds.ID,
		RunAt:      now(),
		JudgeModel: e.config.Judge.Model,
		Scores:     finalScores,
		Dashboard: ScoreDashboard{
			OverallScore:   overallScore,
			DimensionCount: len(finalScores),
			ItemCount:      len(items),
			JudgeCost:      totalCost,
			ByDomain:       byDomain,
		},
	}

	// 5. Delta 对比
	if e.deltaAnalyzer != nil {
		report.Delta = e.deltaAnalyzer.Compare(report)
		if report.Delta != nil {
			report.TuneSuggest = NewTuneGenerator().Suggest(report.Delta)
		}
	}

	// 6. 保存基线
	if opts.SaveBaseline {
		basePath := filepath.Join(filepath.Dir(opts.DatasetPath), "baseline.yaml")
		if err := SaveBaseline(basePath, report); err != nil {
			return nil, fmt.Errorf("save baseline: %w", err)
		}
	}

	return report, nil
}

func (e *EvalEngine) loadDataset(path string) (*EvalDataset, error) {
	if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
		return LoadDatasetVersion(path, "latest")
	}
	return LoadDataset(path)
}

func (e *EvalEngine) findProbe(dimension string) Probe {
	if e.probeRegistry != nil {
		if p, ok := e.probeRegistry[dimension]; ok {
			return p
		}
	}
	return GetProbe(dimension)
}

// aggregatedScore 聚合多条评测用例的评分。
type aggregatedScore struct {
	domain     string
	dimension  string
	totalScore float64
	totalConf  float64
	count      int
	buckets    map[string]float64
	details    map[string]float64
	judgeLogs  []JudgeLog
}

func (a *aggregatedScore) add(s *DomainScore) {
	a.totalScore += s.Score
	a.totalConf += s.Confidence
	a.count++
	if a.buckets == nil {
		a.buckets = make(map[string]float64)
	}
	for k, v := range s.Buckets {
		if existing, ok := a.buckets[k]; ok {
			a.buckets[k] = (existing + v) / 2
		} else {
			a.buckets[k] = v
		}
	}
	if a.details == nil {
		a.details = make(map[string]float64)
	}
	for k, v := range s.Details {
		a.details[k] = v
	}
	a.judgeLogs = append(a.judgeLogs, s.JudgeLogs...)
}

func (a *aggregatedScore) finalize() DomainScore {
	return DomainScore{
		Domain:     a.domain,
		Dimension:  a.dimension,
		Score:      safeDiv(a.totalScore, float64(a.count)),
		Confidence: safeDiv(a.totalConf, float64(a.count)),
		Buckets:    a.buckets,
		Details:    a.details,
		JudgeLogs:  a.judgeLogs,
	}
}

func safeDiv(num, den float64) float64 {
	if den == 0 {
		return 0
	}
	return num / den
}
