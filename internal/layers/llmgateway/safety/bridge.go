// Package safety is a backward-compatibility bridge.
// Deprecated: use github.com/devrix/devrix/internal/layers/llmgateway/guard instead.
package safety

import "github.com/devrix/devrix/internal/layers/llmgateway/guard"

// Action defines what happens when a pattern matches.
type Action = guard.Action

// Match represents a matched pattern.
type Match = guard.Match

// Result is the safety check result.
type Result = guard.Result

// Pattern defines a safety pattern.
type Pattern = guard.Pattern

// LatencySink receives safety check durations.
type LatencySink = guard.LatencySink

// Filter performs safety checks.
type Filter = guard.Filter

// NewFilter creates a Filter.
var NewFilter = guard.NewFilter
