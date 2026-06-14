package breaker

import "github.com/devrix/devrix/internal/layers/llmgateway"

// StateObserverFunc adapts a function to llmgateway.BreakerStateObserver.
//
// DSAFT: D3-S3-A01-F02 (OnStateTransitionEmit, v1.1).
type StateObserverFunc func(provider string, from, to llmgateway.CircuitState)

// OnBreakerStateChange implements llmgateway.BreakerStateObserver.
func (f StateObserverFunc) OnBreakerStateChange(provider string, from, to llmgateway.CircuitState) {
	f(provider, from, to)
}

// NotifyStateChange invokes the observer if set. Safe to call with nil observer.
//
// DSAFT: D3-S3-A01-F02 internal helper.
func NotifyStateChange(observer llmgateway.BreakerStateObserver, provider string, from, to llmgateway.CircuitState) {
	if observer == nil || from == to {
		return
	}
	observer.OnBreakerStateChange(provider, from, to)
}