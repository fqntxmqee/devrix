// Package safety provides configurable safety filtering for LLM requests.
//
// Inspired by Claude Fable-5's refusal_handling and harmful_content_safety
// sections, the safety filter checks system prompts and messages for
// dangerous content before they are sent to an LLM provider.
package safety

import (
	"context"
	"fmt"
	"strings"
	"sync"
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

// Filter performs safety checks on LLM request content.
type Filter struct {
	mu       sync.RWMutex
	patterns []Pattern
}

// NewFilter creates a safety filter with the default patterns.
func NewFilter() *Filter {
	return &Filter{
		patterns: defaultPatterns(),
	}
}

// Check evaluates system prompt and messages against safety patterns.
// Returns a Result indicating whether the request is allowed.
func (f *Filter) Check(ctx context.Context, systemPrompt string, messages []string) *Result {
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
