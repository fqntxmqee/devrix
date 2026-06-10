package testutil

import "time"

// WaitForGatewayAsync blocks until RouteInbound background session persist completes.
// Use in t.Cleanup before tests that call RouteInbound with a file session store.
func WaitForGatewayAsync() {
	time.Sleep(150 * time.Millisecond)
}
