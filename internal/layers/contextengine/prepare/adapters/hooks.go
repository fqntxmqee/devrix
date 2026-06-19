// Package adapters provides concrete implementations of the prepare scenario
// orchestrator's port interfaces (SessionLoader, MemoryRecaller, ContextCompressor,
// PromptAssembler). These wrap the existing prepare/memory.Manager,
// prepare/compression.Pipeline, and prepare/prompt.SystemPromptAssembler,
// emitting observability spans/events through injected hooks so the facade
// retains control of trace context and event channels.
//
// DSAFT: D2-S15 (PrepareExecutionContext) — A01..A04 adapters.
//
// Change: devrix-d2-structure-closure (DM-20260619-007) — P1-b.
package adapters

import (
	"context"

	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// SpanStarter starts a tracer span around an A-level operation.
// Returns (ctx, span); span may be nil when observability is disabled.
type SpanStarter func(ctx context.Context, operation string, kind tracer.SpanKind, attrs ...tracer.Attribute) (context.Context, tracer.Span)

// EventEmitter emits a domain event back to the engine event channel.
// May be nil; adapters must degrade to no-op when nil.
type EventEmitter func(*contracts.EngineEvent)

// Hooks bundles the observability and event hooks shared across the four
// prepare adapters. All fields are optional; nil fields are treated as no-ops.
type Hooks struct {
	StartSpan SpanStarter
	Emit      EventEmitter
}

// HooksOption configures the adapter Hooks bundle.
type HooksOption func(*Hooks)

// WithSpanStarter injects the span-starter hook (used for OTel/observability
// per-A-level spans). Without this hook, adapters skip span emission entirely.
func WithSpanStarter(s SpanStarter) HooksOption {
	return func(h *Hooks) { h.StartSpan = s }
}

// WithEventEmitter injects the engine-event emitter (used for snapshot-restore,
// compression-complete, etc.). Without this hook, adapters drop events silently.
func WithEventEmitter(e EventEmitter) HooksOption {
	return func(h *Hooks) { h.Emit = e }
}

// applyHooks applies the given options to a fresh Hooks struct.
func applyHooks(opts []HooksOption) Hooks {
	h := Hooks{}
	for _, o := range opts {
		o(&h)
	}
	return h
}

// startSpan is a small helper: invoke h.StartSpan if set, else return ctx, nil.
func (h Hooks) startSpan(ctx context.Context, op string, kind tracer.SpanKind, attrs ...tracer.Attribute) (context.Context, tracer.Span) {
	if h.StartSpan == nil {
		return ctx, nil
	}
	return h.StartSpan(ctx, op, kind, attrs...)
}

// emit is a small helper: invoke h.Emit if set, else drop.
func (h Hooks) emit(ev *contracts.EngineEvent) {
	if h.Emit != nil && ev != nil {
		h.Emit(ev)
	}
}