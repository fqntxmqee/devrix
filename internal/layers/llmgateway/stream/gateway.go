package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/budget"
	"github.com/devrix/devrix/internal/layers/llmgateway/configure"
	"github.com/devrix/devrix/internal/layers/llmgateway/protect"
	"github.com/devrix/devrix/internal/layers/llmgateway/protect/errorclass"
	"github.com/devrix/devrix/internal/layers/llmgateway/route"
	"github.com/devrix/devrix/internal/layers/llmgateway/stream/adapter"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/diagnose/incident"
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
	// The observer emits llm_breaker_state gauge, llm_breaker_transitions_total
	// counter, and (optionally) an EngineEvent for D7 to subscribe to.
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

// Stream performs a streaming LLM call.
func (g *Gateway) Stream(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	// Main llm.stream span (parent for all sub-operations)
	streamCtx, streamSpan := g.startSpan(ctx, telemetry.OpD3_S3_LLM_Stream, tracer.SpanKindClient)
	streamStart := time.Now()
	if streamSpan != nil {
		g.recordStreamRequest(streamSpan, req)
	}
	finishStream := func(err error, usage llmgateway.TokenUsage, usageReceived bool, provider, model string) {
		if streamSpan == nil {
			return
		}
		g.recordStreamResponse(streamSpan, err, usage, provider, model)
		attrs := telemetry.SpanAttrs(telemetry.OpD3_S3_LLM_Stream,
			tracer.Attribute{Key: "llm.provider", Value: provider},
			tracer.Attribute{Key: "llm.model", Value: model},
		)
		attrs = append(attrs, telemetry.LLMUsageAttrs(usage.PromptTokens, usage.CompletionTokens, time.Since(streamStart).Milliseconds())...)
		// DM-20260611-008：记录 provider 是否回传了 usage 帧。
		// false 表示 SSE 流里完全没出现 usage 字段 → 需排查 provider 协议。
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

	var provider, model string

	// llm.provider.route child span
	{
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
			finishStream(err, llmgateway.TokenUsage{}, false, "", "")
			return nil, g.classify(streamCtx, err)
		}
		provider, model = p, m
	}

	providerCfg, ok := g.cfg.Providers[provider]
	if !ok {
		err := fmt.Errorf("provider config missing: %s", provider)
		finishStream(err, llmgateway.TokenUsage{}, false, provider, model)
		return nil, g.classify(streamCtx, err)
	}

	count := g.counter.CountWithSystemPrompt(req.SystemPrompt, req.Messages)
	if err := g.counter.CheckBudget(count, defaultPromptTokenBudget); err != nil {
		finishStream(err, llmgateway.TokenUsage{}, false, provider, model)
		return nil, g.classify(streamCtx, err)
	}

	// llm.circuit_breaker child span
	{
		_, cbSpan := g.startSpan(streamCtx, telemetry.OpD3_S3_LLM_CircuitBreaker, tracer.SpanKindInternal,
			tracer.Attribute{Key: "llm.provider", Value: provider},
		)
		allowed, err := g.breaker.Allow(provider)
		if cbSpan != nil {
			cbSpan.SetAttributes(tracer.Attribute{Key: "llm.breaker_allowed", Value: fmt.Sprintf("%t", allowed)})
			cbSpan.End()
		}
		if err != nil {
			finishStream(err, llmgateway.TokenUsage{}, false, provider, model)
			return nil, g.classify(streamCtx, err)
		}
		if !allowed {
			finishStream(fmt.Errorf("circuit breaker rejected: %s", provider), llmgateway.TokenUsage{}, false, provider, model)
			return nil, g.classify(streamCtx, fmt.Errorf("circuit breaker rejected: %s", provider))
		}
	}

	ad, err := g.reg.Get(provider)
	if err != nil {
		finishStream(err, llmgateway.TokenUsage{}, false, provider, model)
		return nil, g.classify(streamCtx, err)
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
	retryCtx, retrySpan := g.startSpan(streamCtx, telemetry.OpD3_S3_LLM_Retry, tracer.SpanKindInternal,
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
		_, adSpan := g.startSpan(callCtx, telemetry.OpD3_S3_LLM_Adapter_Stream, tracer.SpanKindClient,
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
		finishStream(err, llmgateway.TokenUsage{}, false, provider, model)
		return nil, g.classify(streamCtx, err)
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
		var usageReceived bool
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
					usageReceived = true
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
			// DM-20260617-002 W1: classify + shortstack wrapper on stream errors.
			// 流错误通过 finishStream(span RecordError + RecordLLMSpanPayload) 落库，
			// out channel 正常关闭让 consumer 拿到 io.EOF 风格的语义。
			classified := g.classify(streamCtx, streamErr)
			finishStream(classified, usage, usageReceived, provider, model)
			return
		}
		g.breaker.RecordSuccess(provider)
		g.recordSuccess(provider, primaryModel, start)
		finishStream(nil, usage, usageReceived, provider, model)
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
	info, toolNames := buildStreamRequestInfo(req)
	bz, _ := json.Marshal(info)
	incident.RecordLLMSpanPayload(
		span, "", "request", "llm.request", "llm.request_json", string(bz),
		0, req.Model,
		tracer.Attribute{Key: "llm.messages_count", Value: len(req.Messages)},
		tracer.Attribute{Key: "llm.tools_count", Value: len(req.Tools)},
		tracer.Attribute{Key: "llm.tools_names", Value: toolNames},
	)
}

// buildStreamRequestInfo assembles the JSON payload + compact tool-name
// summary that recordStreamRequest attaches to the LLM stream span.
//
// The payload mirrors the `tools_json` shape sent to the provider: a
// `tools` array with name + description always present, and the JSON
// Schema `parameters` block included only when the operator enabled
// `observability.llm.log_content` (otherwise each tool's schema can
// be 5KB+ and would bloat every span).
//
// The compact `toolNames` returned separately is a comma-separated
// list of tool names for fast span-attribute queries (e.g. Jaeger
// filters `llm.tools_names=bash,read`). The full `tools` array is
// available in the JSON payload for detailed inspection.
func buildStreamRequestInfo(req *llmgateway.Request) (info map[string]interface{}, toolNames string) {
	full := incident.LLMLogContentEnabled()
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

	tools := summarizeToolsForTrace(req.Tools, full)
	names := make([]string, 0, len(req.Tools))
	for _, t := range req.Tools {
		if t.Name == "" {
			continue
		}
		names = append(names, t.Name)
	}
	toolNames = strings.Join(names, ",")

	info = map[string]interface{}{
		"model":                req.Model,
		"message_count":        len(req.Messages),
		"tool_count":           len(req.Tools),
		"tool_names":           names,
		"system_prompt_length": len(req.SystemPrompt),
		"messages":             msgs,
		"tools":                tools,
	}
	if full {
		info["system_prompt"] = req.SystemPrompt
	}
	return info, toolNames
}

// summarizeToolsForTrace turns the LLM-side `req.Tools` into the same
// shape sent in the provider `tools_json` body, minus the wire
// `type:"function"` wrapper which is implicit. Name and description
// are always included; the JSON Schema `parameters` are only included
// when `full` is true (governed by observability.llm.log_content).
//
// Descriptions are truncated to 200 chars in the summary-only path
// to keep span payloads compact when many tools are offered.
func summarizeToolsForTrace(tools []llmgateway.ToolSchema, full bool) []map[string]interface{} {
	if len(tools) == 0 {
		return []map[string]interface{}{}
	}
	out := make([]map[string]interface{}, 0, len(tools))
	for _, t := range tools {
		entry := map[string]interface{}{
			"name": t.Name,
		}
		desc := strings.TrimSpace(t.Description)
		if !full && len(desc) > 200 {
			desc = desc[:200] + "..."
		}
		if desc != "" {
			entry["description"] = desc
		}
		if full {
			if params, ok := parseToolParametersJSON(t.Parameters); ok {
				entry["parameters"] = params
			} else if t.Parameters != "" {
				// Schema didn't parse — keep the raw string so the
				// trace still has the original payload the provider
				// was given (or would have been given, if it failed
				// at adapter time).
				entry["parameters_raw"] = t.Parameters
			}
		}
		out = append(out, entry)
	}
	return out
}

func parseToolParametersJSON(raw string) (any, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, false
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil, false
	}
	return v, true
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
	incident.RecordLLMSpanPayload(
		span, "", "response", "llm.response", "llm.response_json", string(bz),
		0, model,
		tracer.Attribute{Key: "llm.provider", Value: provider},
	)
}

var _ llmgateway.IGateway = (*Gateway)(nil)
