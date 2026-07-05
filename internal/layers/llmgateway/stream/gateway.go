package stream

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/budget"
	"github.com/devrix/devrix/internal/layers/llmgateway/configure"
	"github.com/devrix/devrix/internal/layers/llmgateway/protect"
	"github.com/devrix/devrix/internal/layers/llmgateway/protect/errorclass"
	"github.com/devrix/devrix/internal/layers/llmgateway/route"
	"github.com/devrix/devrix/internal/layers/llmgateway/stream/adapter"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
)

const (
	defaultPromptTokenBudget = 128000
	defaultStreamTimeout     = 30 * time.Second
)

// Deps holds gateway dependencies.
type Deps struct {
	Config   *configure.LLMGatewayConfig
	Registry *adapter.Registry
	Breaker  llmgateway.ICircuitBreaker
	Retry    *protect.Executor
	Counter  *budget.Counter
	Obs      *observability.Bridge
}

// Gateway orchestrates routing, breaker, retry, and adapters.
type Gateway struct {
	cfg        *configure.LLMGatewayConfig
	router     *route.Router
	breaker    llmgateway.ICircuitBreaker
	retry      *protect.Executor
	reg        *adapter.Registry
	counter    *budget.Counter
	obs        *observability.Bridge
	classifier errorclass.Classifier // optional, DM-20260617-002 W1
}

// WithClassifier 注入错误分类器。nil 时降级为仅做短栈包装。
func (g *Gateway) WithClassifier(c errorclass.Classifier) *Gateway {
	g.classifier = c
	return g
}

// classify 是 classifyAndWrap 在 Gateway 上的便捷方法。
//
// DM-20260620-003 (PR-C M2): ctx is now passed so downstream code can pull
// the cached Classification via ClassifyResultFromCtx.
func (g *Gateway) classify(ctx context.Context, err error) error {
	return classifyAndWrap(ctx, g.classifier, err, 0, "")
}

// classifyWithStatus 带 HTTP status 的错误分类（adapter 层有非零 status 时用）。
func (g *Gateway) classifyWithStatus(ctx context.Context, err error, status int, raw string) error {
	return classifyAndWrap(ctx, g.classifier, err, status, raw)
}

// New creates a gateway from dependencies.
func New(deps Deps) *Gateway {
	cfg := deps.Config
	if cfg == nil {
		cfg = configure.DefaultLLMGatewayConfig()
	}
	g := &Gateway{
		cfg:     cfg,
		router:  route.NewRouter(cfg),
		breaker: deps.Breaker,
		retry:   deps.Retry,
		reg:     deps.Registry,
		counter: deps.Counter,
		obs:     deps.Obs,
	}
	if deps.Retry == nil {
		g.retry = protect.NewExecutor()
	}

	// DSAFT: D3-S3-A01-F01 + F02 + F03 (v1.1).
	// Attach a state-change observer if the breaker implementation supports it.
	if obsBreaker, ok := deps.Breaker.(llmgateway.ICircuitBreakerWithObserver); ok {
		obsBreaker.WithObserver(protect.NewBreakerObserver(deps.Obs, protect.PublishBreakerStateDefault{}))
	}

	return g
}

// TokenCounter returns the gateway token counter as the shared contract.
func (g *Gateway) TokenCounter() contracts.ITokenCounter {
	return g.counter
}

// ResolveTier resolves a tier alias to a concrete model name.
func (g *Gateway) ResolveTier(tier string) string {
	return g.router.ResolveTier(tier)
}

// Stream performs a streaming LLM call. The orchestration delegates to:
//
//  1. pipeline.routeAndCheck — S1 RouteModel + S4 BudgetTokens + S3 Breaker.Allow
//  2. startRetrySpan — opens the `D3_LLM_Retry` span wrapping the retry+adapter loop
//  3. openAdapterCall — inner closure that emits `D3_LLM_Adapter_Stream` per attempt
//  4. fanout loop — channels adapter chunks to the consumer, records usage/breaker
//  5. finishStream — closes the outer `D3_LLM_Stream` span with status/usage
//
// DM-20260629-003 PR-2: Stream() was 235 LOC god fn, split into pipeline.go +
// protected.go + instrument.go. The orchestration here is now ~120 LOC.
func (g *Gateway) Stream(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	// Phase 1: open outer stream span + request payload.
	streamCtx, streamSpan := g.startSpan(ctx, telemetry.OpD3_S3_LLM_Stream, tracer.SpanKindClient)
	streamStart := time.Now()
	if streamSpan != nil {
		g.recordStreamRequest(streamSpan, req)
	}
	finishStream := func(err error, usage llmgateway.TokenUsage, usageReceived bool, provider, model string, capture *streamResponseCapture) {
		if streamSpan == nil {
			return
		}
		g.recordStreamResponse(streamSpan, err, usage, provider, model, capture)
		attrs := telemetry.SpanAttrs(telemetry.OpD3_S3_LLM_Stream,
			tracer.Attribute{Key: "llm.provider", Value: provider},
			tracer.Attribute{Key: "llm.model", Value: model},
		)
		attrs = append(attrs, telemetry.LLMUsageAttrs(usage.PromptTokens, usage.CompletionTokens, time.Since(streamStart).Milliseconds())...)
		attrs = append(attrs, tracer.Attribute{Key: "llm.usage_received", Value: fmt.Sprintf("%t", usageReceived)})
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
		metrics.RecordGenAITokenUsage(g.obs.Meter(), model, metrics.GenAITokenBreakdown{
			Input:     usage.PromptTokens,
			Output:    usage.CompletionTokens,
			CacheRead: usage.CacheReadTokens,
			Reasoning: usage.ReasoningTokens,
		})
	}

	// Phase 2: route + budget + breaker.
	rr, err := g.routeAndCheck(streamCtx, req)
	if err != nil {
		finishStream(err, llmgateway.TokenUsage{}, false, rr.provider, rr.model, nil)
		return nil, g.classify(streamCtx, err)
	}
	provider, model := rr.provider, rr.model
	providerCfg := rr.cfg

	// Phase 3: lookup adapter.
	ad, err := g.reg.Get(provider)
	if err != nil {
		finishStream(err, llmgateway.TokenUsage{}, false, provider, model, nil)
		return nil, g.classify(streamCtx, err)
	}

	// Phase 4: timeout setup.
	var cancel context.CancelFunc
	if _, ok := streamCtx.Deadline(); !ok {
		timeout := providerCfg.Timeout
		if timeout <= 0 {
			timeout = defaultStreamTimeout
		}
		streamCtx, cancel = context.WithTimeout(streamCtx, timeout)
	}

	// Phase 5: retry span + adapter call closure.
	envelope := g.startRetrySpan(streamCtx, provider, model, providerCfg.FallbackModel)
	primaryReq := *req
	streamCall := g.openAdapterCall(ad, provider, providerCfg, &primaryReq)
	primaryModel := model
	if primaryModel == "" {
		primaryModel = providerCfg.DefaultModel
	}

	// Phase 6: dispatch retry + adapter call.
	retryCfg := configure.LLMRetryConfig{
		MaxAttempts:  providerCfg.Retry.MaxAttempts,
		InitialDelay: providerCfg.Retry.InitialDelay,
		MaxDelay:     providerCfg.Retry.MaxDelay,
		Backoff:      providerCfg.Retry.Backoff,
	}
	adapterCh, err := g.retry.Stream(envelope.ctx, streamCall, primaryModel, providerCfg.FallbackModel, retryCfg)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		g.breaker.RecordFailure(provider)
		g.recordError(provider, primaryModel)
		if envelope.span != nil {
			envelope.span.RecordError(err)
			envelope.span.End()
		}
		finishStream(err, llmgateway.TokenUsage{}, false, provider, model, nil)
		return nil, g.classify(streamCtx, err)
	}

	// Phase 7: fanout loop.
	out := make(chan llmgateway.Chunk, 32)
	go func() {
		defer close(out)
		if cancel != nil {
			defer cancel()
		}
		if envelope.span != nil {
			defer envelope.span.End()
		}

		// Open a child span covering the SSE chunk consumption phase.
		// Without this, the gap between Adapter_Stream end (HTTP connect)
		// and Retry end (all chunks consumed) appears as untraced "ghost time".
		_, consumeSpan := g.startSpan(envelope.ctx, telemetry.OpD3_S3_LLM_Stream_Consume, tracer.SpanKindInternal,
			tracer.Attribute{Key: "llm.provider", Value: provider},
			tracer.Attribute{Key: "llm.model", Value: model},
		)

		var streamErr error
		var usage llmgateway.TokenUsage
		var usageReceived bool
		var chunksConsumed int
		responseCapture := newStreamResponseCapture()
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
				chunksConsumed++
				if ac.Parsed.Usage.PromptTokens > 0 || ac.Parsed.Usage.CompletionTokens > 0 {
					usage = ac.Parsed.Usage
					usageReceived = true
				}
				responseCapture.observe(*ac.Parsed)
				select {
				case <-streamCtx.Done():
					streamErr = streamCtx.Err()
					break streamLoop
				case out <- *ac.Parsed:
				}
			}
		}

		if consumeSpan != nil {
			consumeSpan.SetAttributes(
				tracer.Attribute{Key: "llm.chunks_consumed", Value: fmt.Sprintf("%d", chunksConsumed)},
			)
			if streamErr != nil {
				consumeSpan.RecordError(streamErr)
				consumeSpan.SetStatus(tracer.StatusCodeError, streamErr.Error())
			} else {
				consumeSpan.SetStatus(tracer.StatusCodeOk, "")
			}
			consumeSpan.End()
		}

		if streamErr != nil {
			if shouldRecordBreakerFailure(streamErr) {
				g.breaker.RecordFailure(provider)
			}
			g.recordError(provider, primaryModel)
			classified := g.classify(streamCtx, streamErr)
			finishStream(classified, usage, usageReceived, provider, model, responseCapture)
			return
		}
		g.breaker.RecordSuccess(provider)
		g.recordSuccess(provider, primaryModel, streamStart)
		finishStream(nil, usage, usageReceived, provider, model, responseCapture)
	}()

	return out, nil
}

// Close releases gateway resources.
func (g *Gateway) Close() error {
	return nil
}

func shouldRecordBreakerFailure(err error) bool {
	return err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

var _ llmgateway.IGateway = (*Gateway)(nil)
