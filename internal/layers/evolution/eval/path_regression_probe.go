package eval

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/observability/runtime"
)

func init() {
	RegisterProbe(&PathRegressionProbe{})
}

// PathRegressionProbe (D6 探针，DM-20260611-004)
//
// 职责：QueryLoop 退役 Legacy Harness 路径后，断言"旧路径调用计数 = 0"。
//
// 工作方式：
//   - 不需要 judge，仅读取 `runtime.Snapshot()` 的
//     `LegacyHarness` / `QueryLoop` 计数。
//   - baseline (Default): QueryLoop ≥ 1, LegacyHarness = 0。
//   - failure: LegacyHarness > 0 → score 0，并返回 regression。
//   - warning: QueryLoop = 0 且 LegacyHarness = 0（无运行实例）→ score 1
//     并标记 buckets，让 gate 能区分"无流量"和"全 query_loop"两种情形。
//
// 该探针不依赖 LLM，可在 CI 中作为 D6-Gate 关卡运行（gate_test 中可
// 单独调用 Probe.Score() 而不走 judge）。
type PathRegressionProbe struct{}

func (p *PathRegressionProbe) ID() string {
	return "path_regression"
}

// Baseline expectation.
const (
	pathRegressionLegacyMax = int64(0)
)

// Run executes the probe using the in-process runtime counters.
//
// The `judge` argument is unused (this probe does not consult an LLM);
// callers can pass nil.
func (p *PathRegressionProbe) Run(ctx context.Context, item EvalItem, _ Judge) (*DomainScore, error) {
	snap := runtime.Snapshot()

	details := map[string]float64{
		"runtime.query_loop_total":     float64(snap.QueryLoop),
		"runtime.legacy_harness_total": float64(snap.LegacyHarness),
	}

	score := 1.0
	regression := false

	switch {
	case snap.LegacyHarness > pathRegressionLegacyMax:
		score = 0.0
		regression = true
	case snap.QueryLoop == 0 && snap.LegacyHarness == 0:
		// No traffic at all — pass but flag as a warning so dashboards
		// can distinguish "nothing ran" from "everything went through
		// QueryLoop". Score is still 1.0.
		details["no_traffic"] = 1.0
	}

	severity := SeverityStable
	if regression {
		severity = SeverityRegression
	}

	// Dimension: path_regression. Domain: d2 (ContextEngine).
	// The probe is intentionally self-contained; we don't fill
	// JudgeLogs because no LLM judge is involved.
	ds := &DomainScore{
		Domain:     "d2",
		Dimension:  p.ID(),
		Score:      score,
		Confidence: 1.0,
		Details:    details,
	}
	if item.Bucket != "" {
		ds.Buckets = map[string]float64{item.Bucket: score}
		_ = severity
	}
	// Surface a human-readable reasoning field via Details so the
	// dashboard can show why we scored this way.
	if regression {
		ds.Details["reason"] = float64(len(
			fmt.Sprintf("legacy_harness=%d > max=%d", snap.LegacyHarness, pathRegressionLegacyMax),
		))
	}
	return ds, nil
}
