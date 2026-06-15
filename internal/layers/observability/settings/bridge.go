// Package settings is a backward-compatibility bridge.
//
// Deprecated: use github.com/devrix/devrix/internal/layers/observability/configure/settings instead.
// This bridge will be removed in v2.1.
package settings

import "github.com/devrix/devrix/internal/layers/observability/configure/settings"

// Types

type (
	TracingConfig  = settings.TracingConfig
	SamplingConfig = settings.SamplingConfig
	OTLPConfig     = settings.OTLPConfig
	MetricsConfig  = settings.MetricsConfig
	LabelsConfig   = settings.LabelsConfig
)
