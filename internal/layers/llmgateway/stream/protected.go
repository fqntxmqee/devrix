package stream

import (
	"context"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/configure"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
)

// retryEnvelope holds the retry span lifecycle returned from startRetrySpan.
type retryEnvelope struct {
	ctx      context.Context
	span     tracer.Span
	fallback string
}

// startRetrySpan opens a `D3_LLM_Retry` span and stamps the primary +
// fallback model attributes. Caller must call span.End() exactly once.
//
// DM-20260629-003 PR-2: extracted from gateway.go::Stream() to separate
// the retry/adapter span lifecycle from the routing phase.
func (g *Gateway) startRetrySpan(streamCtx context.Context, provider, model string, fallback string) retryEnvelope {
	ctx, span := g.startSpan(streamCtx, telemetry.OpD3_S3_LLM_Retry, tracer.SpanKindInternal,
		tracer.Attribute{Key: "llm.provider", Value: provider},
		tracer.Attribute{Key: "llm.model_primary", Value: model},
	)
	if span != nil && fallback != "" && fallback != model {
		span.SetAttributes(tracer.Attribute{Key: "llm.model_fallback", Value: fallback})
	}
	return retryEnvelope{ctx: ctx, span: span, fallback: fallback}
}

// openAdapterCall builds the closure passed to g.retry.Stream: each
// attempt opens a `D3_LLM_Adapter_Stream` span and dispatches the call
// to the registered adapter.
//
// DM-20260629-003 PR-2: extracted from gateway.go::Stream()'s streamCall
// closure (~22 LOC).
func (g *Gateway) openAdapterCall(ad llmgateway.IAdapter, provider string, providerCfg configure.LLMProviderRuntimeConfig, primaryReq *llmgateway.Request) func(callCtx context.Context, callModel string) (<-chan *llmgateway.AdapterChunk, error) {
	return func(callCtx context.Context, callModel string) (<-chan *llmgateway.AdapterChunk, error) {
		_, adSpan := g.startSpan(callCtx, telemetry.OpD3_S3_LLM_Adapter_Stream, tracer.SpanKindClient,
			tracer.Attribute{Key: "llm.provider", Value: provider},
			tracer.Attribute{Key: "llm.model", Value: callModel},
		)

		callReq := *primaryReq
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
}
