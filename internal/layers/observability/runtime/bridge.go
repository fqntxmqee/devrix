// Package runtime is a backward-compatibility bridge.
//
// Deprecated: use github.com/devrix/devrix/internal/layers/observability/configure/runtime instead.
// This bridge will be removed in v2.1.
package runtime

import "github.com/devrix/devrix/internal/layers/observability/configure/runtime"

// Types

type (
	PathKind     = runtime.PathKind
	PathCounters = runtime.PathCounters
)

// Constants

const (
	PathQueryLoop           = runtime.PathQueryLoop
	PathLegacyHarness       = runtime.PathLegacyHarness
	PathResolvedTotalMetric = runtime.PathResolvedTotalMetric
	PathLabelKey            = runtime.PathLabelKey
	PathLabelQueryLoop      = runtime.PathLabelQueryLoop
	PathLabelLegacyHarness  = runtime.PathLabelLegacyHarness
)

// Types — metrics

type (
	RuntimeMetric = runtime.RuntimeMetric
)

// Functions

var (
	Global                = runtime.Global
	Reset                 = runtime.Reset
	Snapshot              = runtime.Snapshot
	Record                = runtime.Record
	RegisterRuntimeMetric = runtime.RegisterRuntimeMetric
	ResetRuntimeMetric    = runtime.ResetRuntimeMetric
	RuntimeMetricRegistered = runtime.RuntimeMetricRegistered
	IncRuntimeMetric      = runtime.IncRuntimeMetric
)
