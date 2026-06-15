// Package coverage is a backward-compatibility bridge.
//
// Deprecated: use github.com/devrix/devrix/internal/layers/observability/diagnose/coverage instead.
// This bridge will be removed in v2.1.
package coverage

import "github.com/devrix/devrix/internal/layers/observability/diagnose/coverage"

// Types

type (
	CLI          = coverage.CLI
	Config       = coverage.Config
	ZeroHitEntry = coverage.ZeroHitEntry
	Report       = coverage.Report
	Counter      = coverage.Counter
	Persistence  = coverage.Persistence
	DailyReport  = coverage.DailyReport
	OperationMeta = coverage.OperationMeta
	SpanProvider = coverage.SpanProvider
	Reporter     = coverage.Reporter
	OpInfo       = coverage.OpInfo
	ByLayer      = coverage.ByLayer
)

// Functions

var (
	NewCLI           = coverage.NewCLI
	DefaultConfig    = coverage.DefaultConfig
	NewCounter       = coverage.NewCounter
	InitGlobal       = coverage.InitGlobal
	Global           = coverage.Global
	RecordHit        = coverage.RecordHit
	RecordUnknown    = coverage.RecordUnknown
	NewPersistence   = coverage.NewPersistence
	RegisterProvider = coverage.RegisterProvider
	RegisteredSpans  = coverage.RegisteredSpans
	AllOperations    = coverage.AllOperations
	KnownOperations  = coverage.KnownOperations
	IsKnown          = coverage.IsKnown
	NewReporter      = coverage.NewReporter
)
