package capture

import (
	"context"
	"log/slog"
	"sync"

	"github.com/devrix/devrix/internal/layers/communication/delivery/eventbus"
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
type eventDispatcher struct {
	mu sync.RWMutex
	bus EventBusPort
	subCancel func()
}

func newEventDispatcher(bus EventBusPort) *eventDispatcher {
	return &eventDispatcher{bus: bus}
}

func (d *eventDispatcher) IsEnabled() bool {
	return d != nil && d.bus != nil
}

func (d *eventDispatcher) SetBus(bus EventBusPort) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.bus = bus
}

func (d *eventDispatcher) SetSubCancel(cancel func()) {
	d.mu.Lock()
	d.subCancel = cancel
	d.mu.Unlock()
}

func (d *eventDispatcher) Close() {
	d.mu.Lock()
	cancel := d.subCancel
	d.subCancel = nil
	d.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (d *eventDispatcher) Publish(ctx context.Context, ev *EngineEvent) {
	if d == nil {
		return
	}
	d.mu.RLock()
	bus := d.bus
	d.mu.RUnlock()
	if bus == nil || ev == nil {
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

// SetEventBus wires a BackpressureEventBus into the capture.
func (g *CommunicationGateway) SetEventBus(bus EventBusPort) {
	if g.eventDispatcher == nil {
		g.eventDispatcher = newEventDispatcher(bus)
	} else {
		g.eventDispatcher.SetBus(bus)
	}
}

// EventBusEnabled reports whether a backpressure event bus is wired in.
func (g *CommunicationGateway) EventBusEnabled() bool {
	return g.eventDispatcher != nil && g.eventDispatcher.IsEnabled()
}
