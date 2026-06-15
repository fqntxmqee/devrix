package evaluate

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/configure/runtime"
)

func TestPathRegressionProbe_ID(t *testing.T) {
	p := &PathRegressionProbe{}
	if p.ID() != "path_regression" {
		t.Errorf("ID = %q, want path_regression", p.ID())
	}
}

// D2-S11-A01-T02: harnessEnabled 分支不再被生产路径触发。
// 场景：CI 中 baseline 是 query_loop=0 legacy_harness=0 → pass。
func TestPathRegressionProbe_AllZero_Pass(t *testing.T) {
	runtime.Reset()
	p := &PathRegressionProbe{}
	item := EvalItem{ID: "test", Bucket: "ci", Dimension: "path_regression"}
	ds, err := p.Run(context.Background(), item, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if ds.Score != 1.0 {
		t.Errorf("Score = %f, want 1.0", ds.Score)
	}
	if ds.Buckets["ci"] != 1.0 {
		t.Errorf("Bucket score = %f, want 1.0", ds.Buckets["ci"])
	}
	if v := ds.Details["no_traffic"]; v != 1.0 {
		t.Errorf("no_traffic = %v, want 1.0", v)
	}
}

// D2-S11-A01-T02: query_loop=1, legacy_harness=0 → pass。
func TestPathRegressionProbe_QueryLoopOnly_Pass(t *testing.T) {
	runtime.Reset()
	runtime.Record(runtime.PathQueryLoop)
	p := &PathRegressionProbe{}
	ds, err := p.Run(context.Background(), EvalItem{ID: "t"}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if ds.Score != 1.0 {
		t.Errorf("Score = %f, want 1.0", ds.Score)
	}
	if v := ds.Details["runtime.legacy_harness_total"]; v != 0 {
		t.Errorf("legacy_harness detail = %v, want 0", v)
	}
	if v := ds.Details["runtime.query_loop_total"]; v != 1 {
		t.Errorf("query_loop detail = %v, want 1", v)
	}
}

// D2-S11-A01-T02: legacy_harness > 0 → fail。
func TestPathRegressionProbe_LegacyHarnessFires_Fail(t *testing.T) {
	runtime.Reset()
	runtime.Record(runtime.PathLegacyHarness)
	p := &PathRegressionProbe{}
	ds, err := p.Run(context.Background(), EvalItem{ID: "t"}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if ds.Score != 0.0 {
		t.Errorf("Score = %f, want 0.0", ds.Score)
	}
}
