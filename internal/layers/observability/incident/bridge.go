// Package incident is a backward-compatibility bridge.
//
// Deprecated: use github.com/devrix/devrix/internal/layers/observability/diagnose/incident instead.
// This bridge will be removed in v2.1.
package incident

import "github.com/devrix/devrix/internal/layers/observability/diagnose/incident"

// Types

type (
	Bundle        = incident.Bundle
	TraceSection  = incident.TraceSection
	SpanEntry     = incident.SpanEntry
	CoverageHit   = incident.CoverageHit
	ExportOptions = incident.ExportOptions
)

// Functions

var (
	BuildBundle = incident.BuildBundle
	MarshalJSON = incident.MarshalJSON
)
