// Package guard provides configurable safety filtering for LLM requests.
//
// Inspired by Claude Fable-5's refusal_handling and harmful_content_safety
// sections, the safety filter checks system prompts and messages for
// dangerous content before they are sent to an LLM provider.
package guard

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Action defines what happens when a pattern matches.
type Action string

const (
	ActionAllow Action = "allow" // no action, allow the request through
	ActionReject Action = "reject" // reject the request with an error
	ActionWarn   Action = "warn" // allow but log a warning
)

// Match contains details about a safety pattern match.
type Match struct {
	Pattern  string
	Severity string
	Action   Action
	Location string // "system_prompt" or "message"
}

// Result contains the overall safety check result.
type Result struct {
	Allowed bool
	Matches []Match
	Reason  string // human-readable summary when rejected
}

// Pattern defines a single safety check pattern.
type Pattern struct {
	Name        string
	Description string
	Patterns    []string // substrings to search for (lowercase)
	Action      Action
	Severity    string // "critical", "high", "medium", "low"
	Locations   []string // where to check: "system_prompt", "message", or both
}

// LatencySink receives safety check durations for observability.
//
// DSAFT: D3-S5-A01-F04 EmitSafetyLatencyEvent (v1.1 F8, D5-A 决议 P99 < 1ms).
// The sink is the seam between safety and observability; the safety package
// stays decoupled from tracer/metrics. A no-op sink is used when the feature
// flag `d3_safety_latency_event_enabled` is off.
type LatencySink interface {
	RecordSafetyCheckDuration(durationMs int64)
}

// noopLatencySink is the default sink when no wiring is provided.
type noopLatencySink struct{}

func (noopLatencySink) RecordSafetyCheckDuration(int64) {}

// Filter performs safety checks on LLM request content.
type Filter struct {
	mu       sync.RWMutex
	patterns []Pattern
	sink     LatencySink
	emit     bool
}

// NewFilter creates a safety filter with the default patterns.
func NewFilter() *Filter {
	return &Filter{
		patterns: defaultPatterns(),
		sink:     noopLatencySink{},
		emit:     false,
	}
}

// WithLatencySink attaches a sink and the emit flag.
//
// DSAFT: D3-S5-A01-F04 (v1.1 F8). Pass emit=false to keep the v1.0
// no-emit behavior (feature flag `d3_safety_latency_event_enabled=OFF`).
func (f *Filter) WithLatencySink(sink LatencySink, emit bool) *Filter {
	f.mu.Lock()
	defer f.mu.Unlock()
	if sink == nil {
		f.sink = noopLatencySink{}
	} else {
		f.sink = sink
	}
	f.emit = emit
	return f
}

// Check evaluates system prompt and messages against safety patterns.
// Returns a Result indicating whether the request is allowed.
//
// DSAFT: D3-S5-A01-F04 — every check is timed; the duration is forwarded
// to the sink only when emit=true. Timing itself is O(1) and lock-free on
// the hot path (time.Now + arithmetic), so the v1.1 F8 budget stays under
// the D5-A P99 < 1ms target.
func (f *Filter) Check(ctx context.Context, systemPrompt string, messages []string) *Result {
	start := time.Now()
	result := f.check(ctx, systemPrompt, messages)

	f.mu.RLock()
	emit, sink := f.emit, f.sink
	f.mu.RUnlock()
	if emit && sink != nil {
		sink.RecordSafetyCheckDuration(time.Since(start).Milliseconds())
	}
	return result
}

func (f *Filter) check(ctx context.Context, systemPrompt string, messages []string) *Result {
	f.mu.RLock()
	patterns := f.patterns
	f.mu.RUnlock()

	if len(patterns) == 0 {
		return &Result{Allowed: true}
	}

	var matches []Match
	sysLower := strings.ToLower(systemPrompt)

	for _, p := range patterns {
		// Check system prompt
		for _, checkLoc := range p.Locations {
			if checkLoc == "system_prompt" || checkLoc == "all" {
				for _, pat := range p.Patterns {
					if strings.Contains(sysLower, pat) {
						matches = append(matches, Match{
							Pattern:  p.Name,
							Severity: p.Severity,
							Action:   p.Action,
							Location: "system_prompt",
						})
						goto nextPattern // one match per pattern is enough
					}
				}
			}

			if checkLoc == "message" || checkLoc == "all" {
				for _, msg := range messages {
					msgLower := strings.ToLower(msg)
					for _, pat := range p.Patterns {
						if strings.Contains(msgLower, pat) {
							matches = append(matches, Match{
								Pattern:  p.Name,
								Severity: p.Severity,
								Action:   p.Action,
								Location: "message",
							})
							goto nextPattern
						}
					}
				}
			}
		}
	nextPattern:
	}

	var allowed bool
	var reason string

	// Check for reject-level matches first
	for _, m := range matches {
		if m.Action == ActionReject {
			allowed = false
			reason = fmt.Sprintf("safety filter rejected: %s (%s)", m.Pattern, m.Severity)
			return &Result{Allowed: false, Matches: matches, Reason: reason}
		}
	}

	allowed = true
	return &Result{Allowed: allowed, Matches: matches, Reason: ""}
}

// AddPattern adds a custom safety pattern.
func (f *Filter) AddPattern(p Pattern) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.patterns = append(f.patterns, p)
}

// Patterns returns a copy of the current patterns.
func (f *Filter) Patterns() []Pattern {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]Pattern, len(f.patterns))
	copy(out, f.patterns)
	return out
}
