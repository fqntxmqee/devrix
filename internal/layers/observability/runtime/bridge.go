// Package runtime is a backward-compatibility bridge.
//
// Deprecated: use github.com/devrix/devrix/internal/layers/observability/configure/runtime instead.
package runtime

import "github.com/devrix/devrix/internal/layers/observability/configure/runtime"

type (
	PathKind     = runtime.PathKind
	PathCounters = runtime.PathCounters
)

const (
	PathD7Turn              = runtime.PathD7Turn
	PathLegacyHarness       = runtime.PathLegacyHarness
	PathResolvedTotalMetric = runtime.PathResolvedTotalMetric
	PathLabelKey            = runtime.PathLabelKey
	PathLabelD7Turn         = runtime.PathLabelD7Turn
	PathLabelLegacyHarness  = runtime.PathLabelLegacyHarness
)

type RuntimeMetric = runtime.RuntimeMetric

var (
	Global                  = runtime.Global
	Reset                   = runtime.Reset
	Snapshot                = runtime.Snapshot
	Record                  = runtime.Record
	RegisterRuntimeMetric   = runtime.RegisterRuntimeMetric
	ResetRuntimeMetric      = runtime.ResetRuntimeMetric
	RuntimeMetricRegistered = runtime.RuntimeMetricRegistered
	IncRuntimeMetric        = runtime.IncRuntimeMetric
)
