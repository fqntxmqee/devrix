// Package sessionorchestrator — convergence_metric_test.go
//
// DM-20260705-010 (devrix-d7-mups-frame-delta-closure) Phase 3 T14:
// 5 子测试覆盖 ComputeConvergenceMetric deterministic 收敛度量 (design.md §6.1).
//
// 子测试清单 (AC4 + AC7):
//  1. TestComputeConvergenceMetric_FirstRoundZero — 空 subTurns → 零值 ConvergenceMetric{}
//  2. TestComputeConvergenceMetric_ToolDiffRate — initialGaps/residualGaps → uncertainty_reduction_rate
//  3. TestBuildRoundSubTurnRecord_ClaimAccumulation — ResolutionClaim 闭合 obs gap 计数 + report 优先
//  4. TestEmitConvergenceMetric_JaegerSpanComplete — nil-bridge span emit 不 panic (telemetry 未初始化)
//  5. TestComputeConvergenceMetric_ZeroLLMDeterministic — 纯函数, 相同输入产相同输出, 0 LLM
//
// T IDs: D7-S9-A113-T01..T05.
package sessionorchestrator

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

// TestComputeConvergenceMetric_FirstRoundZero — case 1 (D7-S9-A113-T01):
// 空 subTurns → 零值兜底, 不阻塞 sub-turn.
func TestComputeConvergenceMetric_FirstRoundZero(t *testing.T) {
	got := ComputeConvergenceMetric(nil, nil)
	if got.UncertaintyReductionRate != 0 || got.ObservedGapsClosedCount != 0 || got.FrameDeltaConsumed {
		t.Fatalf("empty subTurns should return zero ConvergenceMetric, got %+v", got)
	}
	got = ComputeConvergenceMetric([]SubTurnRecord{}, nil)
	if got != (ConvergenceMetric{}) {
		t.Fatalf("empty slice should return zero-value ConvergenceMetric, got %+v", got)
	}
}

// TestComputeConvergenceMetric_ToolDiffRate — case 2 (D7-S9-A113-T02):
// deterministic 工具结果 diff → uncertainty_reduction_rate.
func TestComputeConvergenceMetric_ToolDiffRate(t *testing.T) {
	cases := []struct {
		name         string
		initial      int
		residual     int
		wantRate     float64
		wantClosed   int
		consumed     bool
		wantConsumed bool
	}{
		{"5→0 full converge", 5, 0, 1.0, 5, true, true},
		{"5→2 partial 60%", 5, 2, 0.6, 3, true, true},
		{"4→2 half", 4, 2, 0.5, 2, false, false},
		{"initialGaps 0 → rate 0", 0, 0, 0.0, 0, true, true},
		{"residual > initial clamps to 0 closed", 3, 5, 0.0, 0, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := ComputeConvergenceMetric([]SubTurnRecord{
				{InitialObsGaps: c.initial, ResidualObsGaps: c.residual, PromptContainsPlanFrameDelta: c.consumed},
			}, nil)
			if m.UncertaintyReductionRate != c.wantRate {
				t.Errorf("rate = %v, want %v", m.UncertaintyReductionRate, c.wantRate)
			}
			if m.ObservedGapsClosedCount != c.wantClosed {
				t.Errorf("closed = %d, want %d", m.ObservedGapsClosedCount, c.wantClosed)
			}
			if m.FrameDeltaConsumed != c.wantConsumed {
				t.Errorf("consumed = %v, want %v", m.FrameDeltaConsumed, c.wantConsumed)
			}
		})
	}
	// AC7: 末轮 uncertainty_reduction_rate ≥ 0.5 for a 5-sub-turn converging run.
	multi := ComputeConvergenceMetric([]SubTurnRecord{
		{InitialObsGaps: 6, ResidualObsGaps: 6},
		{InitialObsGaps: 6, ResidualObsGaps: 4},
		{InitialObsGaps: 4, ResidualObsGaps: 3},
		{InitialObsGaps: 3, ResidualObsGaps: 2},
		{InitialObsGaps: 2, ResidualObsGaps: 1, PromptContainsPlanFrameDelta: true},
	}, nil)
	// first.Initial=6, last.Residual=1 → (6-1)/6 = 0.833 ≥ 0.5.
	if multi.UncertaintyReductionRate < 0.5 {
		t.Fatalf("AC7: 5-sub-turn末轮 rate = %v, want ≥ 0.5", multi.UncertaintyReductionRate)
	}
	if !multi.FrameDeltaConsumed {
		t.Error("AC7: last sub-turn consumed frame delta, want FrameDeltaConsumed=true")
	}
}

// TestBuildRoundSubTurnRecord_ClaimAccumulation — case 3 (D7-S9-A113-T03):
// ResolutionClaim 闭合 obs gap 计数 (legacy) + ResolutionReport 优先路径.
func TestBuildRoundSubTurnRecord_ClaimAccumulation(t *testing.T) {
	obsIDs := []string{"obs-1", "obs-2", "obs-3"}

	// Legacy path (nil report): 2 of 3 obs answered → residual 1.
	claims := []interfaces.ResolutionClaim{
		{ObsID: "obs-1", Answer: "resolved via tool X"},
		{ObsID: "obs-2", Answer: ""}, // empty answer → not closed
		{ObsID: "obs-3", Answer: "confirmed"},
	}
	rec := BuildRoundSubTurnRecord(obsIDs, claims, nil, true)
	if rec.InitialObsGaps != 3 {
		t.Errorf("InitialObsGaps = %d, want 3", rec.InitialObsGaps)
	}
	if rec.ResidualObsGaps != 1 { // obs-1 + obs-3 closed, obs-2 empty answer残留
		t.Errorf("ResidualObsGaps = %d, want 1 (obs-1+obs-3 closed)", rec.ResidualObsGaps)
	}
	if !rec.PromptContainsPlanFrameDelta {
		t.Error("PromptContainsPlanFrameDelta = false, want true")
	}

	// Report path takes precedence over claim counting.
	report := &interfaces.ResolutionReport{
		TotalStrategies: 5,
		UnresolvedObs:   []interfaces.UnresolvedObs{{ObsID: "obs-x"}, {ObsID: "obs-y"}},
	}
	recR := BuildRoundSubTurnRecord(obsIDs, claims, report, false)
	if recR.InitialObsGaps != 5 {
		t.Errorf("report path InitialObsGaps = %d, want 5 (TotalStrategies)", recR.InitialObsGaps)
	}
	if recR.ResidualObsGaps != 2 {
		t.Errorf("report path ResidualObsGaps = %d, want 2 (len UnresolvedObs)", recR.ResidualObsGaps)
	}

	// Feeding the record through ComputeConvergenceMetric closes the loop.
	m := ComputeConvergenceMetric([]SubTurnRecord{rec}, nil)
	if m.ObservedGapsClosedCount != 2 {
		t.Errorf("closed via claims = %d, want 2", m.ObservedGapsClosedCount)
	}
}

// TestEmitConvergenceMetric_JaegerSpanComplete — case 4 (D7-S9-A113-T04):
// nil-bridge span emit does not panic when telemetry is uninitialized.
func TestEmitConvergenceMetric_JaegerSpanComplete(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("EmitConvergenceMetric panicked on nil-bridge path: %v", r)
		}
	}()
	end := hardening.EmitConvergenceMetric(context.Background(), "sess_test", 0.83, 5, true)
	if end == nil {
		t.Fatal("EmitConvergenceMetric returned nil end func")
	}
	end(nil)
}

// TestComputeConvergenceMetric_ZeroLLMDeterministic — case 5 (D7-S9-A113-T05):
// pure function — same input yields same output, 0 LLM invocation.
func TestComputeConvergenceMetric_ZeroLLMDeterministic(t *testing.T) {
	subTurns := []SubTurnRecord{
		{InitialObsGaps: 5, ResidualObsGaps: 1, PromptContainsPlanFrameDelta: true},
	}
	first := ComputeConvergenceMetric(subTurns, nil)
	for i := 0; i < 100; i++ {
		got := ComputeConvergenceMetric(subTurns, nil)
		if got != first {
			t.Fatalf("iteration %d: non-deterministic output %+v != %+v", i, got, first)
		}
	}
	// lastMetric arg must not change the v1.0 result (reserved for v1.1 smoothing).
	withLast := ComputeConvergenceMetric(subTurns, &ConvergenceMetric{UncertaintyReductionRate: 0.99})
	if withLast != first {
		t.Fatalf("lastMetric must be ignored in v1.0, got %+v != %+v", withLast, first)
	}
}
