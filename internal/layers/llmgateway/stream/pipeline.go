package stream

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/configure"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
)

// routeResult is the outcome of the pipeline routing phase.
type routeResult struct {
	provider string
	model    string
	cfg      configure.LLMProviderRuntimeConfig
}

// routeAndCheck runs the S1 RouteModel phase: resolves model alias via
// the router and emits a `D3_LLM_Provider_Route` span child of the
// outer stream span.
//
// DM-20260629-003 PR-2: extracted from gateway.go::Stream() (originally
// 17 LOC, 7 step pipeline). Routing + budget + breaker checks all live
// in this file.
func (g *Gateway) routeAndCheck(streamCtx context.Context, req *llmgateway.Request) (routeResult, error) {
	_, routeSpan := g.startSpan(streamCtx, telemetry.OpD3_S3_LLM_Provider_Route, tracer.SpanKindInternal,
		tracer.Attribute{Key: "llm.model_requested", Value: req.Model},
	)
	p, m, err := g.router.Resolve(req.Model)
	if routeSpan != nil {
		routeSpan.SetAttributes(
			tracer.Attribute{Key: "llm.provider", Value: p},
			tracer.Attribute{Key: "llm.model_resolved", Value: m},
		)
		routeSpan.End()
	}
	if err != nil {
		return routeResult{}, err
	}

	providerCfg, ok := g.cfg.Providers[p]
	if !ok {
		return routeResult{provider: p, model: m}, fmt.Errorf("provider config missing: %s", p)
	}

	// S4 BudgetTokens (injection mode): count tokens and check budget.
	count := g.counter.CountWithSystemPrompt(req.SystemPrompt, req.Messages)
	if err := g.counter.CheckBudget(count, defaultPromptTokenBudget); err != nil {
		return routeResult{provider: p, model: m}, err
	}

	// S3 ProtectCall: circuit breaker allow check.
	if err := g.checkBreakerAllow(streamCtx, p); err != nil {
		return routeResult{provider: p, model: m}, err
	}

	return routeResult{provider: p, model: m, cfg: providerCfg}, nil
}

// checkBreakerAllow emits a `D3_LLM_CircuitBreaker` span and queries the
// breaker. The call is allowed to continue only when breaker.Allow returns
// (true, nil); any other outcome yields an error suitable for the caller
// to forward to finishStream.
//
// DM-20260629-003 PR-2: extracted from gateway.go::Stream().
func (g *Gateway) checkBreakerAllow(streamCtx context.Context, provider string) error {
	_, cbSpan := g.startSpan(streamCtx, telemetry.OpD3_S3_LLM_CircuitBreaker, tracer.SpanKindInternal,
		tracer.Attribute{Key: "llm.provider", Value: provider},
	)
	allowed, err := g.breaker.Allow(provider)
	if cbSpan != nil {
		cbSpan.SetAttributes(tracer.Attribute{Key: "llm.breaker_allowed", Value: fmt.Sprintf("%t", allowed)})
		cbSpan.End()
	}
	if err != nil {
		return err
	}
	if !allowed {
		return fmt.Errorf("circuit breaker rejected: %s", provider)
	}
	return nil
}
