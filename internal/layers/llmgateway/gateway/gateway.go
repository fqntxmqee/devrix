package gateway

import (
	"context"
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

	provider, model, err := g.router.Resolve(req.Model)
	if err != nil {
		return nil, err
	}
	providerCfg, ok := g.cfg.Providers[provider]
	if !ok {
		return nil, fmt.Errorf("provider config missing: %s", provider)
	}

	count := g.counter.CountWithSystemPrompt(req.SystemPrompt, req.Messages)
	if err := g.counter.CheckBudget(count, defaultPromptTokenBudget); err != nil {
		return nil, err
	}

	allowed, err := g.breaker.Allow(provider)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, err
	}

	ad, err := g.reg.Get(provider)
	if err != nil {
		return nil, err
	}

	streamCtx := ctx
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		timeout := providerCfg.Timeout
		if timeout <= 0 {
			timeout = defaultStreamTimeout
		}
		streamCtx, cancel = context.WithTimeout(ctx, timeout)
	}

	ctx, finishSpan := g.startStreamSpan(streamCtx, provider, model)
	start := time.Now()

	streamCall := func(callCtx context.Context, callModel string) (<-chan *llmgateway.AdapterChunk, error) {
		callReq := *req
		callReq.Provider = provider
		callReq.Model = callModel
		callReq.MaxTokens = providerCfg.MaxTokens
		callReq.Temperature = providerCfg.Temperature
		callReq.Stream = true
		return ad.Stream(callCtx, &callReq)
	}

	primaryModel := model
	if primaryModel == "" {
		primaryModel = providerCfg.DefaultModel
	}
	adapterCh, err := g.retry.Stream(streamCtx, streamCall, primaryModel, providerCfg.FallbackModel, providerCfg.Retry)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		g.breaker.RecordFailure(provider)
		g.recordError(provider, primaryModel)
		finishSpan(err, llmgateway.TokenUsage{})
		return nil, err
	}

	out := make(chan llmgateway.Chunk, 32)
	go func() {
		defer close(out)
		if cancel != nil {
			defer cancel()
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
			finishSpan(streamErr, usage)
			return
		}
		g.breaker.RecordSuccess(provider)
		g.recordSuccess(provider, primaryModel, start)
		finishSpan(nil, usage)
	}()

	return out, nil
}

// Close releases gateway resources.
func (g *Gateway) Close() error {
	return nil
}

func (g *Gateway) startStreamSpan(ctx context.Context, provider, model string) (context.Context, func(error, llmgateway.TokenUsage)) {
	if g.obs == nil || g.obs.Tracer() == nil {
		return ctx, func(error, llmgateway.TokenUsage) {}
	}
	start := time.Now()
	ctx, span := g.obs.Tracer().Start(ctx, telemetry.OpLLMStream,
		tracer.WithSpanKind(tracer.SpanKindClient),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpLLMStream,
			tracer.Attribute{Key: "llm.provider", Value: provider},
			tracer.Attribute{Key: "llm.model", Value: model},
		)...),
	)
	return ctx, func(err error, usage llmgateway.TokenUsage) {
		if span == nil {
			return
		}
		attrs := telemetry.LLMUsageAttrs(usage.PromptTokens, usage.CompletionTokens, time.Since(start).Milliseconds())
		if err != nil {
			span.RecordError(err)
			span.SetStatus(tracer.StatusCodeError, err.Error())
			attrs = append(attrs, tracer.Attribute{Key: "llm.status", Value: "error"})
		} else {
			span.SetStatus(tracer.StatusCodeOk, "")
			attrs = append(attrs, tracer.Attribute{Key: "llm.status", Value: "ok"})
		}
		span.SetAttributes(attrs...)
		span.End()
	}
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

var _ llmgateway.IGateway = (*Gateway)(nil)
