package workmodel

import (
	"context"
	"strconv"

	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
)

type locatorFrameCtxKey struct{}

// WithLocatorFrame attaches the active pipeline locator to ctx for Jaeger emitters.
func WithLocatorFrame(ctx context.Context, frame LocatorFrame) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, locatorFrameCtxKey{}, frame)
}

// LocatorFrameFrom reads the pipeline locator from ctx.
func LocatorFrameFrom(ctx context.Context) (LocatorFrame, bool) {
	if ctx == nil {
		return LocatorFrame{}, false
	}
	v, ok := ctx.Value(locatorFrameCtxKey{}).(LocatorFrame)
	return v, ok
}

// WithLocatorPhase returns ctx with an updated phase segment (immutable copy).
func WithLocatorPhase(ctx context.Context, phase string) context.Context {
	f, ok := LocatorFrameFrom(ctx)
	if !ok {
		return ctx
	}
	f.Phase = phase
	return WithLocatorFrame(ctx, f)
}

// WithLocatorIter returns ctx with an updated execute iter segment.
func WithLocatorIter(ctx context.Context, iter int) context.Context {
	f, ok := LocatorFrameFrom(ctx)
	if !ok {
		return ctx
	}
	f.Iter = iter
	return WithLocatorFrame(ctx, f)
}

// LocatorSpanAttrsFromContext builds Jaeger attrs for hardening emitters.
// Wired from bootstrap to avoid hardening → workmodel import cycle.
func LocatorSpanAttrsFromContext(ctx context.Context) []tracer.Attribute {
	f, ok := LocatorFrameFrom(ctx)
	if !ok {
		return nil
	}
	attrs := []tracer.Attribute{
		{Key: "locator", Value: BuildLocator(f)},
	}
	if sem := f.SemanticID; sem != "" {
		attrs = append(attrs, tracer.Attribute{Key: "worktree.semantic_id", Value: sem})
	}
	if f.RoundNo > 0 {
		attrs = append(attrs, tracer.Attribute{Key: "pipeline.round_no", Value: strconv.Itoa(f.RoundNo)})
	}
	if tr := f.Trigger; tr != "" {
		attrs = append(attrs, tracer.Attribute{Key: "pipeline.trigger", Value: tr})
	}
	if f.LoopTick > 0 {
		attrs = append(attrs, tracer.Attribute{Key: "session.loop_tick", Value: strconv.Itoa(f.LoopTick)})
	}
	return attrs
}
