package evaluate

import "math"

const (
	// RegressionRedThreshold 是 delta 红色门禁阈值：自动标记 regression。
	RegressionRedThreshold = -0.05
	// RegressionYellowThreshold 是 delta 黄色预警阈值。
	RegressionYellowThreshold = -0.02
)

// DeltaAnalyzer 对比当前评分与基线。
type DeltaAnalyzer struct {
	baseline *EvalReport
}

// NewDeltaAnalyzer 创建 delta 分析器。
func NewDeltaAnalyzer(baseline *EvalReport) *DeltaAnalyzer {
	return &DeltaAnalyzer{baseline: baseline}
}

// Compare 对比当前评分与基线，返回 delta。
func (da *DeltaAnalyzer) Compare(current *EvalReport) *EvalDelta {
	if da.baseline == nil || current == nil {
		return nil
	}

	byDimension := make(map[string]DeltaEntry)
	byBucket := make(map[string]DeltaEntry)
	var regressions []DeltaEntry

	// 按维度对比
	for _, cur := range current.Scores {
		prev := da.findBaselineScore(cur.Domain, cur.Dimension)
		entry := DeltaEntry{
			Dimension: cur.Domain + "." + cur.Dimension,
			Previous:  prev,
			Current:   cur.Score,
			Delta:     cur.Score - prev,
			Severity:  classifyDelta(cur.Score - prev),
		}
		byDimension[entry.Dimension] = entry
		if entry.Severity == SeverityRegression {
			regressions = append(regressions, entry)
		}

		// 按分桶对比
		for bucket, curBucketScore := range cur.Buckets {
			prevBucket := da.findBaselineBucketScore(cur.Domain, cur.Dimension, bucket)
			bucketEntry := DeltaEntry{
				Dimension: cur.Domain + "." + cur.Dimension + "." + bucket,
				Previous:  prevBucket,
				Current:   curBucketScore,
				Delta:     curBucketScore - prevBucket,
				Severity:  classifyDelta(curBucketScore - prevBucket),
			}
			byBucket[bucketEntry.Dimension] = bucketEntry
		}
	}

	return &EvalDelta{
		BaselineID:  da.baseline.ID,
		ByDimension: byDimension,
		ByBucket:    byBucket,
		Regressions: regressions,
	}
}

func (da *DeltaAnalyzer) findBaselineScore(domain, dimension string) float64 {
	for _, s := range da.baseline.Scores {
		if s.Domain == domain && s.Dimension == dimension {
			return s.Score
		}
	}
	return 0
}

func (da *DeltaAnalyzer) findBaselineBucketScore(domain, dimension, bucket string) float64 {
	for _, s := range da.baseline.Scores {
		if s.Domain == domain && s.Dimension == dimension {
			if s.Buckets != nil {
				if v, ok := s.Buckets[bucket]; ok {
					return v
				}
			}
			return s.Score
		}
	}
	return 0
}

func classifyDelta(d float64) string {
	if d < RegressionRedThreshold {
		return SeverityRegression
	}
	if d < RegressionYellowThreshold {
		return "borderline"
	}
	if d > math.Abs(RegressionRedThreshold) {
		return SeverityImprovement
	}
	return SeverityStable
}
