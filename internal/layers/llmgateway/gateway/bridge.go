// Package gateway is a backward-compatibility bridge.
// Deprecated: types formerly in this package are now in route, stream, and protect sub-packages.
package gateway

import (
	"github.com/devrix/devrix/internal/layers/llmgateway/protect"
	"github.com/devrix/devrix/internal/layers/llmgateway/route"
	"github.com/devrix/devrix/internal/layers/llmgateway/stream"
)

// Router resolves model names to provider + concrete model.
//
// Deprecated: use route.Router instead.
type Router = route.Router

// NewRouter creates a provider router.
//
// Deprecated: use route.NewRouter instead.
var NewRouter = route.NewRouter

// Gateway orchestrates routing, breaker, retry, and adapters.
//
// Deprecated: use stream.Gateway instead.
type Gateway = stream.Gateway

// Deps holds gateway dependencies.
//
// Deprecated: use stream.Deps instead.
type Deps = stream.Deps

// New creates a gateway from dependencies.
//
// Deprecated: use stream.New instead.
var New = stream.New

// NewFromConfig wires the full LLM gateway stack.
//
// Deprecated: use stream.NewFromConfig instead.
var NewFromConfig = stream.NewFromConfig

// NewBreakerObserver constructs a StateObserver for breaker metrics.
//
// Deprecated: use protect.NewBreakerObserver instead.
var NewBreakerObserver = protect.NewBreakerObserver

// PublishBreakerStateDefault is the no-op publisher.
//
// Deprecated: use protect.PublishBreakerStateDefault instead.
type PublishBreakerStateDefault = protect.PublishBreakerStateDefault
