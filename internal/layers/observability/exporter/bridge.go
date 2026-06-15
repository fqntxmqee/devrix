// Package exporter is a backward-compatibility bridge.
//
// Deprecated: use github.com/devrix/devrix/internal/layers/observability/export instead.
// This bridge will be removed in v2.1.
package exporter

import "github.com/devrix/devrix/internal/layers/observability/export"

// Types

type (
	ConsoleExporter      = export.ConsoleExporter
	NullExporter         = export.NullExporter
	OTLPExporter         = export.OTLPExporter
	MemoryExporter       = export.MemoryExporter
	OTLPResourceSpans    = export.OTLPResourceSpans
	OTLPResourceSpansV1  = export.OTLPResourceSpansV1
	OTLPResource         = export.OTLPResource
	OTLPKeyValue         = export.OTLPKeyValue
	OTLPAnyValue         = export.OTLPAnyValue
	OTLPScopeSpans       = export.OTLPScopeSpans
	OTLPSpan             = export.OTLPSpan
	OTLPEvent            = export.OTLPEvent
	OTLPStatus           = export.OTLPStatus
)

// Functions

var (
	NewConsoleExporter  = export.NewConsoleExporter
	NewNullExporter     = export.NewNullExporter
	NewOTLPExporter     = export.NewOTLPExporter
	NewMemoryExporter   = export.NewMemoryExporter
	ResolveOTLPEndpoint = export.ResolveOTLPEndpoint
	NewTracingExporter  = export.NewTracingExporter
)
