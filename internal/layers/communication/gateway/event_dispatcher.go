package gateway

import (
	"context"
	"log/slog"
	"sync"

	"github.com/devrix/devrix/internal/layers/communication/eventbus"
)

// EventBusPort is the minimum interface the gateway needs from a
// backpressure-aware event bus. Defined here (consumer side) so the
// gateway does not import eventbus internals; the concrete
// *eventbus.Bus satisfies this interface.
type EventBusPort interface {
	Publish(ctx context.Context, ev eventbus.Event) error
	PublishCritical(ctx context.Context, ev eventbus.Event) error
	Backlog() int
}

// eventDispatcher glues the gateway's event handler to a backpressure
// event bus. When the bus is nil, the gateway falls back to its
// original synchronous fanout path.
//
// DM-20260611-003 (devrix-event-channel): wires the new BackpressureEventBus
// into the gateway without changing the wire-level OutboundMessage /
// OnMessage / metric contracts.
type eventDispatcher struct {
	mu sync.RWMutex
	// bus may be a *eventbus.Bus (concrete) or any other EventBusPort
	// implementation. We keep the field as the port interface so the
	// gateway never reaches into bus internals.
	bus EventBusPort
	// subCancel cancels the bus subscription when the dispatcher is
	// torn down (e.g. on Close).
	subCancel func()
}

func newEventDispatcher(bus EventBusPort) *eventDispatcher {
	return &eventDispatcher{bus: bus}
}

// IsEnabled reports whether the dispatcher has a bus wired in.
func (d *eventDispatcher) IsEnabled() bool {
	return d != nil && d.bus != nil
}

// SetBus attaches a bus at runtime. Safe to call once during bootstrap.
func (d *eventDispatcher) SetBus(bus EventBusPort) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.bus = bus
}

// SetSubCancel records the subscription cancel for shutdown.
func (d *eventDispatcher) SetSubCancel(cancel func()) {
	d.mu.Lock()
	d.subCancel = cancel
	d.mu.Unlock()
}

// Close cancels the subscription. Safe to call multiple times.
func (d *eventDispatcher) Close() {
	d.mu.Lock()
	cancel := d.subCancel
	d.subCancel = nil
	d.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Publish routes a *EngineEvent through the bus. Critical terminator
// events (complete / error) go via PublishCritical; everything else
// via Publish. When the dispatcher has no bus, the call is a no-op
// (the caller is expected to fall back to direct handling).
func (d *eventDispatcher) Publish(ctx context.Context, ev *EngineEvent) {
	if d == nil {
		return
	}
	d.mu.RLock()
	bus := d.bus
	d.mu.RUnlock()
	if bus == nil {
		return
	}
	if ev == nil {
		return
	}
	busEv := eventbus.Event{EngineEvent: ev}
	if ev.Type == "complete" || ev.Type == "error" {
		busEv = busEv.WithPriority(eventbus.PriorityCritical)
		if err := bus.PublishCritical(ctx, busEv); err != nil {
			slog.Warn("eventbus: PublishCritical failed", "type", ev.Type, "err", err)
		}
		return
	}
	busEv = busEv.WithPriority(eventbus.PriorityNormal)
	if err := bus.Publish(ctx, busEv); err != nil {
		slog.Warn("eventbus: Publish failed", "type", ev.Type, "err", err)
	}
}
