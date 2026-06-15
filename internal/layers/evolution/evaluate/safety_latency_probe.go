package evaluate

import (
	"context"
	"sort"
)

func init() {
	RegisterProbe(&SafetyLatencyProbe{})
}

// SafetyLatencyProbe (D6 探针 #4, v2.2.0, DSAFT: D6-S3-A01-T22).
//
// 验证 D3-S5 safety filter P99 < 1ms（D5-A 决议）。上游 D3-S5-A01 F04
// EmitSafetyLatencyEvent 在 `safety.check.duration_ms` span event 上累计。
//
// 工作方式：
//   - 确定性探针，不调用 judge。
//   - 输入：EvalItem.Input 提供 duration_ms 数组（≥ 100 样本）；
//     若样本 < 100，标记 insufficient_samples 并返回 Score=1.0 + warning。
//   - 评分：
//     - P99 < 1ms          → Score = 1.0
//     - P99 ∈ [1ms, 2ms)   → Score = 0.5（Yellow）
//     - P99 ≥ 2ms          → Score = 0.0（Red）
type SafetyLatencyProbe struct{}

func (p *SafetyLatencyProbe) ID() string {
	return "safety_latency"
}

const (
	safetyLatencyTargetP99Ms = 1.0
	safetyLatencyRedP99Ms    = 2.0
	safetyLatencyMinSamples  = 100
)

func (p *SafetyLatencyProbe) Run(ctx context.Context, item EvalItem, _ Judge) (*DomainScore, error) {
	samples := readDurationSamples(item.Input)

	details := map[string]float64{
		"samples_total": float64(len(samples)),
	}

	if len(samples) < safetyLatencyMinSamples {
		details["insufficient_samples"] = 1.0
		ds := &DomainScore{
			Domain:     "d3",
			Dimension:  p.ID(),
			Score:      1.0,
			Confidence: 0.5,
			Details:    details,
		}
		if item.Bucket != "" {
			ds.Buckets = map[string]float64{item.Bucket: 1.0}
		}
		return ds, nil
	}

	sort.Float64s(samples)
	p50 := percentile(samples, 0.50)
	p95 := percentile(samples, 0.95)
	p99 := percentile(samples, 0.99)
	max := samples[len(samples)-1]

	score := 1.0
	regression := false
	yellow := false
	switch {
	case p99 >= safetyLatencyRedP99Ms:
		score = 0.0
		regression = true
	case p99 >= safetyLatencyTargetP99Ms:
		score = 0.5
		yellow = true
	}

	details["p50_ms"] = p50
	details["p95_ms"] = p95
	details["p99_ms"] = p99
	details["max_ms"] = max
	if regression {
		details["severity"] = 2.0
	} else if yellow {
		details["severity"] = 1.0
	}

	ds := &DomainScore{
		Domain:     "d3",
		Dimension:  p.ID(),
		Score:      score,
		Confidence: 1.0,
		Details:    details,
	}
	if item.Bucket != "" {
		ds.Buckets = map[string]float64{item.Bucket: score}
	}
	return ds, nil
}

// readDurationSamples pulls a []float64 (or []any{number,...}) of durations
// in milliseconds from input["durations_ms"].
func readDurationSamples(input map[string]any) []float64 {
	raw, ok := input["durations_ms"]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []float64:
		out := make([]float64, len(v))
		copy(out, v)
		return out
	case []any:
		out := make([]float64, 0, len(v))
		for _, item := range v {
			if f, ok := item.(float64); ok {
				out = append(out, f)
			} else if n, ok := item.(int); ok {
				out = append(out, float64(n))
			} else if n, ok := item.(int64); ok {
				out = append(out, float64(n))
			}
		}
		return out
	default:
		return nil
	}
}

// percentile returns the p-th percentile (0 ≤ p ≤ 1) of a sorted slice.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}
