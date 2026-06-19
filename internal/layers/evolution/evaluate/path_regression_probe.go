package evaluate

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/observability/configure/runtime"
)

func init() {
	RegisterProbe(&PathRegressionProbe{})
}

// PathRegressionProbe asserts legacy harness path counter stays at zero.
type PathRegressionProbe struct{}

func (p *PathRegressionProbe) ID() string {
	return "path_regression"
}

const pathRegressionLegacyMax = int64(0)

func (p *PathRegressionProbe) Run(ctx context.Context, item EvalItem, _ Judge) (*DomainScore, error) {
	snap := runtime.Snapshot()

	details := map[string]float64{
		"runtime.d7_turn_total":        float64(snap.D7Turn),
		"runtime.legacy_harness_total": float64(snap.LegacyHarness),
	}

	score := 1.0
	regression := false

	switch {
	case snap.LegacyHarness > pathRegressionLegacyMax:
		score = 0.0
		regression = true
	case snap.D7Turn == 0 && snap.LegacyHarness == 0:
		details["no_traffic"] = 1.0
	}

	severity := SeverityStable
	if regression {
		severity = SeverityRegression
	}

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
	if regression {
		ds.Details["reason"] = float64(len(
			fmt.Sprintf("legacy_harness=%d > max=%d", snap.LegacyHarness, pathRegressionLegacyMax),
		))
	}
	return ds, nil
}
