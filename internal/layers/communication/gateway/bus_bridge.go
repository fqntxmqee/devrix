package gateway

import (
	"github.com/devrix/devrix/internal/layers/communication/eventbus"
)

// busSubscribeFn is the Subscribe method shape we need from a bus
// implementation. The port interface doesn't expose it because the
// gateway is a producer, not a consumer — but for the S4 deliverable
// (consume-back path) we use a type assertion bridge.
type busSubscribeFn func(sessionID string) (subID string, ch <-chan eventbus.Event, done <-chan struct{}, cancel func())

// extractBusSubscribe returns the bus's Subscribe function if the
// bus implementation supports it. Returns nil otherwise.
func extractBusSubscribe(bus EventBusPort) busSubscribeFn {
	if bus == nil {
		return nil
	}
	// The concrete *eventbus.Bus has the Subscribe method we need.
	type subscriber interface {
		Subscribe(sessionID string) (string, <-chan eventbus.Event, <-chan struct{}, func())
	}
	if s, ok := bus.(subscriber); ok {
		return s.Subscribe
	}
	return nil
}
