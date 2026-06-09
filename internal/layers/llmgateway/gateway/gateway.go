package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/adapter"
	"github.com/devrix/devrix/internal/layers/llmgateway/retry"
	"github.com/devrix/devrix/internal/layers/llmgateway/token"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/metrics"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

const (
	defaultPromptTokenBudget = 128000
	defaultStreamTimeout     = 30 * time.Second
)

// Deps holds gateway dependencies.
type Deps struct {
	Config   *sharedconfig.LLMGatewayConfig
	Registry *adapter.Registry
	Breaker  llmgateway.ICircuitBreaker
	Retry    *retry.Executor
	Counter  *token.Counter
	Obs      *observability.Bridge
}

// Gateway orchestrates routing, breaker, retry, and adapters.
type Gateway struct {
	cfg     *sharedconfig.LLMGatewayConfig
	router  *Router
	breaker llmgateway.ICircuitBreaker
	retry   *retry.Executor
	reg     *adapter.Registry
	counter *token.Counter
	obs     *observability.Bridge
}

// New creates a gateway from dependencies.
func New(deps Deps) *Gateway {
	cfg := deps.Config
	if cfg == nil {
		cfg = sharedconfig.DefaultLLMGatewayConfig()
	}
	g := &Gateway{
		cfg:     cfg,
		router:  NewRouter(cfg),
		breaker: deps.Breaker,
		retry:   deps.Retry,
		reg:     deps.Registry,
		counter: deps.Counter,
		obs:     deps.Obs,
	}
	if deps.Retry == nil {
		g.retry = retry.NewExecutor()
	}
	return g
}

// TokenCounter returns the gateway token counter as the shared contract.
func (g *Gateway) TokenCounter() contracts.ITokenCounter {
	return g.counter
}

// Stream performs a streaming LLM call.
func (g *Gateway) Stream(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	// Main llm.stream span (parent for all sub-operations)
	streamCtx, streamSpan := g.startSpan(ctx, telemetry.OpLLMStream, tracer.SpanKindClient)
	streamStart := time.Now()
	if streamSpan != nil {
		g.recordStreamRequest(streamSpan, req)
	}
	finishStream := func(err error, usage llmgateway.TokenUsage, provider, model string) {
		if streamSpan == nil {
			return
		}
		g.recordStreamResponse(streamSpan, err, usage, provider, model)
		attrs := telemetry.SpanAttrs(telemetry.OpLLMStream,
			tracer.Attribute{Key: "llm.provider", Value: provider},
			tracer.Attribute{Key: "llm.model", Value: model},
		)
		attrs = append(attrs, telemetry.LLMUsageAttrs(usage.PromptTokens, usage.CompletionTokens, time.Since(streamStart).Milliseconds())...)
		if err != nil {
			streamSpan.RecordError(err)
			streamSpan.SetStatus(tracer.StatusCodeError, err.Error())
			attrs = append(attrs, tracer.Attribute{Key: "llm.status", Value: "error"})
		} else {
			streamSpan.SetStatus(tracer.StatusCodeOk, "")
			attrs = append(attrs, tracer.Attribute{Key: "llm.status", Value: "ok"})
		}
		streamSpan.SetAttributes(attrs...)
		streamSpan.End()
	}

	var provider, model string

	// llm.provider.route child span
	{
		_, routeSpan := g.startSpan(streamCtx, telemetry.OpLLMProviderRoute, tracer.SpanKindInternal,
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
			finishStream(err, llmgateway.TokenUsage{}, "", "")
			return nil, err
		}
		provider, model = p, m
	}

	providerCfg, ok := g.cfg.Providers[provider]
	if !ok {
		err := fmt.Errorf("provider config missing: %s", provider)
		finishStream(err, llmgateway.TokenUsage{}, provider, model)
		return nil, err
	}

	count := g.counter.CountWithSystemPrompt(req.SystemPrompt, req.Messages)
	if err := g.counter.CheckBudget(count, defaultPromptTokenBudget); err != nil {
		finishStream(err, llmgateway.TokenUsage{}, provider, model)
		return nil, err
	}

	// llm.circuit_breaker child span
	{
		_, cbSpan := g.startSpan(streamCtx, telemetry.OpLLMCircuitBreaker, tracer.SpanKindInternal,
			tracer.Attribute{Key: "llm.provider", Value: provider},
		)
		allowed, err := g.breaker.Allow(provider)
		if cbSpan != nil {
			cbSpan.SetAttributes(tracer.Attribute{Key: "llm.breaker_allowed", Value: fmt.Sprintf("%t", allowed)})
			cbSpan.End()
		}
		if err != nil {
			finishStream(err, llmgateway.TokenUsage{}, provider, model)
			return nil, err
		}
		if !allowed {
			finishStream(fmt.Errorf("circuit breaker rejected: %s", provider), llmgateway.TokenUsage{}, provider, model)
			return nil, fmt.Errorf("circuit breaker rejected: %s", provider)
		}
	}

	ad, err := g.reg.Get(provider)
	if err != nil {
		finishStream(err, llmgateway.TokenUsage{}, provider, model)
		return nil, err
	}

	// Timeout setup
	var cancel context.CancelFunc
	if _, ok := streamCtx.Deadline(); !ok {
		timeout := providerCfg.Timeout
		if timeout <= 0 {
			timeout = defaultStreamTimeout
		}
		streamCtx, cancel = context.WithTimeout(streamCtx, timeout)
	}

	// llm.retry span (wraps retry + stream lifecycle)
	retryCtx, retrySpan := g.startSpan(streamCtx, telemetry.OpLLMRetry, tracer.SpanKindInternal,
		tracer.Attribute{Key: "llm.provider", Value: provider},
		tracer.Attribute{Key: "llm.model_primary", Value: model},
	)
	if providerCfg.FallbackModel != "" && providerCfg.FallbackModel != model {
		if retrySpan != nil {
			retrySpan.SetAttributes(tracer.Attribute{Key: "llm.model_fallback", Value: providerCfg.FallbackModel})
		}
	}

	start := time.Now()

	streamCall := func(callCtx context.Context, callModel string) (<-chan *llmgateway.AdapterChunk, error) {
		// llm.adapter.stream child span (child of llm.retry via retryCtx)
		_, adSpan := g.startSpan(callCtx, telemetry.OpLLMAdapterStream, tracer.SpanKindClient,
			tracer.Attribute{Key: "llm.provider", Value: provider},
			tracer.Attribute{Key: "llm.model", Value: callModel},
		)

		callReq := *req
		callReq.Provider = provider
		callReq.Model = callModel
		callReq.MaxTokens = providerCfg.MaxTokens
		callReq.Temperature = providerCfg.Temperature
		callReq.Stream = true
		ch, err := ad.Stream(callCtx, &callReq)

		if adSpan != nil {
			if err != nil {
				adSpan.RecordError(err)
				adSpan.SetStatus(tracer.StatusCodeError, err.Error())
			}
			adSpan.End()
		}
		return ch, err
	}

	primaryModel := model
	if primaryModel == "" {
		primaryModel = providerCfg.DefaultModel
	}

	adapterCh, err := g.retry.Stream(retryCtx, streamCall, primaryModel, providerCfg.FallbackModel, providerCfg.Retry)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		g.breaker.RecordFailure(provider)
		g.recordError(provider, primaryModel)
		if retrySpan != nil {
			retrySpan.RecordError(err)
			retrySpan.End()
		}
		finishStream(err, llmgateway.TokenUsage{}, provider, model)
		return nil, err
	}

	out := make(chan llmgateway.Chunk, 32)
	go func() {
		defer close(out)
		if cancel != nil {
			defer cancel()
		}
		if retrySpan != nil {
			defer retrySpan.End()
		}

		var streamErr error
		var usage llmgateway.TokenUsage
	streamLoop:
		for {
			select {
			case <-streamCtx.Done():
				streamErr = streamCtx.Err()
				break streamLoop
			case ac, ok := <-adapterCh:
				if !ok {
					break streamLoop
				}
				if ac.Error != nil {
					streamErr = ac.Error
					break streamLoop
				}
				if ac.Parsed == nil {
					continue
				}
				if ac.Parsed.Usage.PromptTokens > 0 || ac.Parsed.Usage.CompletionTokens > 0 {
					usage = ac.Parsed.Usage
				}
				select {
				case <-streamCtx.Done():
					streamErr = streamCtx.Err()
					break streamLoop
				case out <- *ac.Parsed:
				}
			}
		}

		if streamErr != nil {
			if shouldRecordBreakerFailure(streamErr) {
				g.breaker.RecordFailure(provider)
			}
			g.recordError(provider, primaryModel)
			finishStream(streamErr, usage, provider, model)
			return
		}
		g.breaker.RecordSuccess(provider)
		g.recordSuccess(provider, primaryModel, start)
		finishStream(nil, usage, provider, model)
	}()

	return out, nil
}

// Close releases gateway resources.
func (g *Gateway) Close() error {
	return nil
}

// startSpan creates a child span if observability is configured.
func (g *Gateway) startSpan(ctx context.Context, operation string, kind tracer.SpanKind, attrs ...tracer.Attribute) (context.Context, tracer.Span) {
	if g.obs == nil || g.obs.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(kind),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(operation, attrs...)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return g.obs.Tracer().Start(ctx, operation, opts...)
}

func (g *Gateway) recordSuccess(provider, model string, start time.Time) {
	if g.obs == nil || g.obs.Meter() == nil {
		return
	}
	labels := metrics.LabelMap{"provider": provider, "model": model}
	if c, err := g.obs.Meter().Int64Counter("llm_requests_total", metrics.WithLabels(labels)); err == nil && c != nil {
		c.Add(1)
	}
	if h, err := g.obs.Meter().Float64Histogram("llm_latency_seconds",
		metrics.WithHistogramLabels(labels),
		metrics.WithBounds(metrics.LLMHistogramBounds()),
	); err == nil && h != nil {
		h.Observe(time.Since(start).Seconds())
	}
}

func shouldRecordBreakerFailure(err error) bool {
	return err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

func (g *Gateway) recordError(provider, model string) {
	if g.obs == nil || g.obs.Meter() == nil {
		return
	}
	labels := metrics.LabelMap{"provider": provider, "model": model, "error_type": "stream"}
	if c, err := g.obs.Meter().Int64Counter("llm_errors_total", metrics.WithLabels(labels)); err == nil && c != nil {
		c.Add(1)
	}
}

func (g *Gateway) recordStreamRequest(span tracer.Span, req *llmgateway.Request) {
	if req == nil {
		return
	}
	full := observability.LLMLogContentEnabled()
	msgs := make([]map[string]string, 0, len(req.Messages))
	limit := 500
	if full {
		limit = 0
	}
	for _, m := range req.Messages {
		content := m.Content
		if limit > 0 && len(content) > limit {
			content = content[:limit] + "..."
		}
		msgs = append(msgs, map[string]string{"role": string(m.Role), "content": content})
	}
	info := map[string]interface{}{
		"model":                req.Model,
		"message_count":        len(req.Messages),
		"tool_count":           len(req.Tools),
		"system_prompt_length": len(req.SystemPrompt),
		"messages":             msgs,
	}
	if full {
		info["system_prompt"] = req.SystemPrompt
	}
	bz, _ := json.Marshal(info)
	observability.RecordLLMSpanPayload(
		span, "", "request", "llm.request", "llm.request_json", string(bz),
		0, req.Model,
		tracer.Attribute{Key: "llm.messages_count", Value: len(req.Messages)},
		tracer.Attribute{Key: "llm.tools_count", Value: len(req.Tools)},
	)
}

func (g *Gateway) recordStreamResponse(span tracer.Span, err error, usage llmgateway.TokenUsage, provider, model string) {
	info := map[string]interface{}{
		"provider":          provider,
		"model":             model,
		"prompt_tokens":     usage.PromptTokens,
		"completion_tokens": usage.CompletionTokens,
	}
	if err != nil {
		info["error"] = err.Error()
	}
	bz, _ := json.Marshal(info)
	observability.RecordLLMSpanPayload(
		span, "", "response", "llm.response", "llm.response_json", string(bz),
		0, model,
		tracer.Attribute{Key: "llm.provider", Value: provider},
	)
}

var _ llmgateway.IGateway = (*Gateway)(nil)
