// Package breaker is a backward-compatibility bridge.
// Deprecated: use github.com/devrix/devrix/internal/layers/llmgateway/protect instead.
package breaker

import "github.com/devrix/devrix/internal/layers/llmgateway/protect"

// CircuitBreaker protects providers from cascading failures.
type CircuitBreaker = protect.CircuitBreaker

// Clock is an injectable time source.
type Clock = protect.Clock

// New creates a circuit breaker.
//
// Deprecated: use protect.New instead.
var New = protect.New
