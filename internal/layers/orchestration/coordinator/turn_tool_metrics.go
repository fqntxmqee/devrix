package coordinator

import (
	"log/slog"

	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
)

// TurnToolMetrics records loop-first orchestration tool invocations.
type TurnToolMetrics struct {
	DelegateWave metrics.Counter // orchestration.tool.delegate_wave
}

// NewTurnToolMetrics registers turn tool counters. Nil meter → nil metrics (no-op).
func NewTurnToolMetrics(meter *metrics.Meter) *TurnToolMetrics {
	if meter == nil {
		return nil
	}
	delegateWave, err := meter.Int64Counter("orchestration.tool.delegate_wave")
	if err != nil {
		slog.Error("orchestrator: failed to register delegate_wave counter", "err", err)
		return nil
	}
	return &TurnToolMetrics{DelegateWave: delegateWave}
}
