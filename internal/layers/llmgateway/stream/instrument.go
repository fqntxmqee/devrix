package stream

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/observability/diagnose/incident"
	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
)

// startSpan creates a child span if observability is configured.
//
// DM-20260629-003 PR-2: extracted from gateway.go (originally 12 LOC).
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

func (g *Gateway) recordStreamResponse(span tracer.Span, err error, usage llmgateway.TokenUsage, provider, model string, capture *streamResponseCapture) {
	info := buildStreamResponseInfo(err, usage, provider, model, capture)
	bz, _ := json.Marshal(info)
	extra := []tracer.Attribute{
		{Key: "llm.provider", Value: provider},
	}
	if v, ok := info["content_len"]; ok {
		extra = append(extra, tracer.Attribute{Key: "llm.response.content_len", Value: v})
	}
	if v, ok := info["content_hash"]; ok {
		extra = append(extra, tracer.Attribute{Key: "llm.response.content_hash", Value: v})
	}
	if v, ok := info["content_preview"]; ok {
		extra = append(extra, tracer.Attribute{Key: "llm.response.content_preview", Value: v})
	}
	if incident.LLMLogContentEnabled() {
		if v, ok := info["content"]; ok {
			extra = append(extra, tracer.Attribute{Key: "llm.response.content", Value: v})
		}
	}
	incident.RecordLLMSpanPayload(
		span, "", "response", "llm.response", "llm.response_json", string(bz),
		0, model,
		extra...,
	)
}

// buildStreamRequestInfo assembles the JSON payload + compact tool-name
// summary that recordStreamRequest attaches to the LLM stream span.
//
// DM-20260629-003 PR-2: extracted from gateway.go.
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
