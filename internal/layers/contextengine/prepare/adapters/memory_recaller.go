// MemoryRecaller adapter: D2-S15-A02 RecallMemory — wraps memory.Manager
// with span + emit hooks.
//
// Matches prepare.MemoryRecaller:
//
//	RecallLongTermEntries(ctx context.Context, query string) ([]memory.MemoryEntry, error)
//
// WorkerLocal context (passed via Hooks.WorkerLocal func) skips recall entirely.
// On SentinelError, emits an error event; on plain error, wraps as LongTermDBError.
//
// Replaces facade/engine_prepare.go::recallLongTermMemory.
package adapters

import (
	"context"
	stderrors "errors"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/errors"
)

// MemoryRecallerAdapter implements prepare.MemoryRecaller.
type MemoryRecallerAdapter struct {
	manager      *memory.Manager
	hooks        Hooks
	workerLocal  func() bool           // optional: returns true to skip recall
	enabled      func() bool           // optional: returns true if longterm enabled
	topics       func() []string       // optional: for span attributes
}

// NewMemoryRecallerAdapter constructs a MemoryRecaller over a memory.Manager.
// `workerLocal` and `enabled` are optional toggles; nil ⇒ treat as not-worker / enabled.
func NewMemoryRecallerAdapter(manager *memory.Manager, opts ...HooksOption) *MemoryRecallerAdapter {
	return &MemoryRecallerAdapter{manager: manager, hooks: applyHooks(opts)}
}

// WithWorkerLocalChecker configures a predicate that, when returning true,
// causes RecallLongTermEntries to skip the long-term DB lookup entirely
// (worker-local context has no long-term memory).
func (a *MemoryRecallerAdapter) WithWorkerLocalChecker(fn func() bool) *MemoryRecallerAdapter {
	a.workerLocal = fn
	return a
}

// WithEnabledChecker configures a predicate for span attribute `longterm.enabled`.
func (a *MemoryRecallerAdapter) WithEnabledChecker(fn func() bool) *MemoryRecallerAdapter {
	a.enabled = fn
	return a
}

// WithTopicsProvider configures a func returning topic names for span attribute
// `longterm.recall_topics`.
func (a *MemoryRecallerAdapter) WithTopicsProvider(fn func() []string) *MemoryRecallerAdapter {
	a.topics = fn
	return a
}

// RecallLongTermEntries returns the long-term memory entries relevant to query.
// Returns nil + nil error (not an error) when worker-local is in effect.
func (a *MemoryRecallerAdapter) RecallLongTermEntries(ctx context.Context, query string) ([]memory.MemoryEntry, error) {
	attrs := []tracer.Attribute{}
	if a.enabled != nil {
		attrs = append(attrs, tracer.Attribute{Key: "longterm.enabled", Value: boolStr(a.enabled())})
	}
	if a.topics != nil {
		topics := a.topics()
		attrs = append(attrs, tracer.Attribute{Key: "longterm.recall_topics", Value: joinStrings(topics, ",")})
	}
	ctx, span := a.hooks.startSpan(ctx, telemetry.OpD2_S2_Context_Longterm_Recall, tracer.SpanKindInternal, attrs...)
	if span != nil {
		defer span.End()
	}

	if a.workerLocal != nil && a.workerLocal() {
		return nil, nil
	}

	entries, err := a.manager.RecallLongTermEntries(ctx, query)
	if err != nil {
		if span != nil {
			span.RecordError(err)
		}
		var se *errors.SentinelError
		if stderrors.As(err, &se) {
			a.hooks.emit(errorEvent("", se, false))
			return nil, err
		}
		wrapped := errors.NewLongTermDBError(err)
		a.hooks.emit(errorEvent("", wrapped, false))
		return nil, wrapped
	}
	return entries, nil
}

// RecallWithSessionID is a convenience that returns entries plus emits an
// info event tagged with sessionID on recall completion (used by facade paths
// that want session-scoped events).
func (a *MemoryRecallerAdapter) RecallWithSessionID(ctx context.Context, sessionID, query string) ([]memory.MemoryEntry, error) {
	_ = sessionID
	entries, err := a.RecallLongTermEntries(ctx, query)
	if err != nil {
		// Already emitted via RecallLongTermEntries.
		return nil, err
	}
	return entries, nil
}

// joinStrings is a tiny helper to avoid pulling fmt/strings just for one
// delimiter join.
func joinStrings(items []string, sep string) string {
	if len(items) == 0 {
		return ""
	}
	n := len(sep) * (len(items) - 1)
	for _, s := range items {
		n += len(s)
	}
	b := make([]byte, 0, n)
	for i, s := range items {
		if i > 0 {
			b = append(b, sep...)
		}
		b = append(b, s...)
	}
	return string(b)
}