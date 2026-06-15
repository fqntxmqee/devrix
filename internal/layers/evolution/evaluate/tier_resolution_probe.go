package evaluate

import (
	"context"
)

func init() {
	RegisterProbe(&TierResolutionProbe{})
}

// TierResolutionProbe (D6 探针 #1, v2.2.0, DSAFT: D6-S3-A01-T20).
//
// 评估 D3 Tier 解析正确性：D3-S1-A01 F06 ProbeTierResolution 在
// `llm_tier_resolve_total{outcome=hit/fallback/error}` 三个桶上累计。
// 探针读取这 3 个计数，计算 hit_ratio 并按 D2-B 决议阈值定级。
//
// 工作方式：
//   - 确定性探针，不调用 judge。
//   - 输入从 EvalItem.Input 读取三个桶计数（snapshot 来源由调用方决定）。
//   - 评分：hit_ratio = hit / (hit+fallback+error)
//     - hit_ratio ≥ 0.99 → Score = 1.0
//     - 0 ≤ hit_ratio < 0.99 → Score = hit_ratio（Yellow：轻微回归）
//     - error > 0 且 hit_ratio < 0.99 → Severity = regression（Red）
type TierResolutionProbe struct{}

func (p *TierResolutionProbe) ID() string {
	return "tier_resolution"
}

const tierResolutionHitRatioTarget = 0.99

func (p *TierResolutionProbe) Run(ctx context.Context, item EvalItem, _ Judge) (*DomainScore, error) {
	hit := int64FromInput(item.Input, "tier_hit")
	fallback := int64FromInput(item.Input, "tier_fallback")
	errorCnt := int64FromInput(item.Input, "tier_error")
	total := hit + fallback + errorCnt

	details := map[string]float64{
		"tier_hit":      float64(hit),
		"tier_fallback": float64(fallback),
		"tier_error":    float64(errorCnt),
	}

	if total == 0 {
		// No samples — pass with a warning so dashboards distinguish
		// "no traffic" from "healthy traffic".
		details["no_traffic"] = 1.0
		ds := &DomainScore{
			Domain:     "d3",
			Dimension:  p.ID(),
			Score:      1.0,
			Confidence: 1.0,
			Details:    details,
		}
		if item.Bucket != "" {
			ds.Buckets = map[string]float64{item.Bucket: 1.0}
		}
		return ds, nil
	}

	hitRatio := float64(hit) / float64(total)
	score := hitRatio
	regression := false
	switch {
	case hitRatio >= tierResolutionHitRatioTarget:
		score = 1.0
	case errorCnt > 0:
		regression = true // Red: error bucket non-zero is a hard regression
	}

	details["hit_ratio"] = hitRatio
	details["fallback_ratio"] = float64(fallback) / float64(total)
	details["error_ratio"] = float64(errorCnt) / float64(total)
	if regression {
		details["severity"] = 2.0 // Red marker for downstream dashboards
	} else if hitRatio < tierResolutionHitRatioTarget {
		details["severity"] = 1.0 // Yellow marker
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
