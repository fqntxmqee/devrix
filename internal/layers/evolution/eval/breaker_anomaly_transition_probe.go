package eval

import (
	"context"
)

func init() {
	RegisterProbe(&BreakerAnomalyTransitionProbe{})
}

// BreakerAnomalyTransitionProbe (D6 探针 #2, v2.2.0, DSAFT: D6-S3-A01-T21).
//
// 检测 Breaker 状态切换的异常模式：D3-S3-A01 F07 OnStateTransitionEmit
// 在 `llm_breaker_transitions_total{from, to}` 上累计。探针按时间窗
// 检测 3 类异常：
//   - 频繁翻转：5min 滚动窗口内翻转 > 3 次 → Yellow
//   - 同 provider 在 30s 内 open→closed ↔ closed→open 交替 ≥ 2 次 → Red
//   - half_open→open 连续 2 次无 closed 介入 → Red
//
// 工作方式：
//   - 确定性探针，不调用 judge。
//   - 输入：EvalItem.Input 提供每条 transition 的 (provider, from, to, at_seconds)
//     时间序列（按 at 升序）。
type BreakerAnomalyTransitionProbe struct{}

func (p *BreakerAnomalyTransitionProbe) ID() string {
	return "breaker_anomaly_transition"
}

const (
	breakerFrequentFlipWindowSec   = 300 // 5 minutes
	breakerFrequentFlipYellowLimit = 3
	breakerAlternateWindowSec      = 30
	breakerAlternateRedLimit       = 2
)

type breakerTransition struct {
	provider string
	from     string
	to       string
	atSec    int64
}

func (p *BreakerAnomalyTransitionProbe) Run(ctx context.Context, item EvalItem, _ Judge) (*DomainScore, error) {
	transitions := readTransitions(item.Input)

	details := map[string]float64{
		"transitions_total": float64(len(transitions)),
	}

	if len(transitions) == 0 {
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

	frequentFlips := detectFrequentFlip(transitions, breakerFrequentFlipWindowSec)
	alternates := detectRapidAlternate(transitions, breakerAlternateWindowSec)
	halfOpenReopen := detectHalfOpenReopenWithoutClose(transitions)

	score := 1.0
	regression := false
	yellow := false

	if halfOpenReopen > 0 {
		score = 0.0
		regression = true // Red
	} else if alternates >= breakerAlternateRedLimit {
		score = 0.0
		regression = true
	} else if frequentFlips > breakerFrequentFlipYellowLimit {
		score = 0.5
		yellow = true
	}

	details["frequent_flip_count"] = float64(frequentFlips)
	details["rapid_alternate_count"] = float64(alternates)
	details["half_open_reopen_streak"] = float64(halfOpenReopen)
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

// readTransitions extracts ordered transitions from input.
// Expected schema:
//
//	transitions: []any{
//	  {provider: "deepseek", from: "closed", to: "open",     at: 100},
//	  {provider: "deepseek", from: "open",   to: "half-open", at: 350},
//	  ...
//	}
func readTransitions(input map[string]any) []breakerTransition {
	raw, ok := input["transitions"]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]breakerTransition, 0, len(items))
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		t := breakerTransition{
			provider: stringFromInput(m, "provider"),
			from:     stringFromInput(m, "from"),
			to:       stringFromInput(m, "to"),
			atSec:    int64FromInput(m, "at"),
		}
		out = append(out, t)
	}
	return out
}

// detectFrequentFlip returns the max count of transitions inside any
// rolling window of windowSec.
func detectFrequentFlip(ts []breakerTransition, windowSec int64) int {
	maxCount := 0
	for i := range ts {
		cutoff := ts[i].atSec
		count := 0
		for j := i; j < len(ts) && ts[j].atSec-cutoff <= windowSec; j++ {
			count++
		}
		if count > maxCount {
			maxCount = count
		}
	}
	return maxCount
}

// detectRapidAlternate counts the number of "open→closed ↔ closed→open"
// alternation events inside windowSec, per provider. With the spec's
// "≥ 2 alternations" rule, two consecutive alternations in a 30s window
// already constitute a regression. We count each alternation event and
// compare against the limit in the caller.
func detectRapidAlternate(ts []breakerTransition, windowSec int64) int {
	count := 0
	for i := 1; i < len(ts); i++ {
		prev, cur := ts[i-1], ts[i]
		if cur.provider != prev.provider {
			continue
		}
		if cur.atSec-prev.atSec > windowSec {
			continue
		}
		isAlt := (prev.from == "open" && prev.to == "closed" &&
			cur.from == "closed" && cur.to == "open") ||
			(prev.from == "closed" && prev.to == "open" &&
				cur.from == "open" && cur.to == "closed")
		if isAlt {
			count++
		}
	}
	return count
}

// detectHalfOpenReopenWithoutClose returns the longest streak of consecutive
// half_open→open transitions with no closed in between. ≥ 1 is regression.
func detectHalfOpenReopenWithoutClose(ts []breakerTransition) int {
	maxStreak := 0
	curStreak := 0
	for _, t := range ts {
		if t.from == "half-open" && t.to == "open" {
			curStreak++
			if curStreak > maxStreak {
				maxStreak = curStreak
			}
		} else {
			curStreak = 0
		}
	}
	return maxStreak
}
