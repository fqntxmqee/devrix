package conclusion

import (
	"context"
	"strings"

	"github.com/devrix/devrix/internal/layers/communication/kernel"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// d1ObsBridge is the package-level observability bridge for D1
// presentation layers (DM-20260630-011). Set via SetBridge at
// bootstrap. Nil means tracing is disabled and the fallback span
// is a no-op. Following the same package-level pattern as
// orchestration/hardening so callers in any D1 sub-package can emit
// spans without threading an obsBridge through every Emitter interface.
var d1ObsBridge *tracer.Tracer

// SetBridge wires the D1 observability bridge for span emission.
// Idempotent; safe to call multiple times. Called from bootstrap
// after observability is initialized.
func SetBridge(t *tracer.Tracer) {
	d1ObsBridge = t
}

// EmitText maps S16-A01 EmitSummaryChunk → OutboundMessage.
func EmitText(session *types.Session, event *contracts.EngineEvent, sig contracts.IMOutboundSignal, hasSig bool, emit kernel.Emitter) {
	if session == nil || event == nil || emit == nil {
		return
	}
	isComplete := event.Metadata != nil && event.Metadata["is_complete"] == "true"
	meta := kernel.OutboundMetadata("text", event.Metadata)
	if hasSig {
		meta = kernel.EnrichMetadata(meta, sig)
	}
	emit.OnMessage(&types.OutboundMessage{
		MessageID:  kernel.NewMessageID(),
		SessionID:  session.SessionID,
		ChatID:     session.ChatID,
		Content:    event.Content,
		IsComplete: isComplete,
		Role:       types.MessageRoleAssistant,
		Metadata:   meta,
	})
}

// EmitComplete maps S16-A02 FinalizeReply (complete) → OutboundMessage.
//
// Content prioritizes the LLM's brief conclusion over the full multi-turn
// transcript:
//   1. event.Metadata["summary"] — D7 orchestrator sets this to the LAST
//      turn's text (the LLM's final synthesis). This is what IM adapters
//      render on the standalone "任务总结" card. PR #137 made the full
//      Content accumulate across turns, which is correct for transcript
//      persistence but wrong for a brief IM card (75K chars of in-flight
//      tool-loop output that the user already saw via streaming).
//   2. event.Content (fallback) — used when D7 didn't populate summary
//      (single-turn replies, or providers that emit no text) OR when D7
//      classified summary as too_short / inconclusive (DM-20260630-011).
//   3. stats string (last resort) — ensures the message body is never empty.
//
// Stats (duration / usage / model / ctx_pct) are surfaced via
// metadata["stats"] so adapters that want a compact stats line (CLI
// capture, fallback cards) can still render one, without colliding with
// the LLM conclusion in metadata["summary"].
//
// DM-20260630-011 AC2: when a fallback path was taken (summary was
// blank or summary_quality ∈ {too_short, inconclusive}), emit the
// hardening EmitEmitCompleteFallback span so dashboards can alert on
// abnormal fallback rate.
func EmitComplete(session *types.Session, event *contracts.EngineEvent, sig contracts.IMOutboundSignal, hasSig bool, emit kernel.Emitter) {
	if session == nil || event == nil || emit == nil {
		return
	}
	usage := kernel.MetaField(event.Metadata, "usage")
	duration := kernel.MetaField(event.Metadata, "duration")
	model := kernel.MetaField(event.Metadata, "model")
	ctxPct := kernel.MetaField(event.Metadata, "ctx_pct")
	summaryQuality := kernel.MetaField(event.Metadata, "summary_quality")
	stats := BuildCompletionSummary(duration, usage, model, ctxPct)
	summary := strings.TrimSpace(kernel.MetaField(event.Metadata, "summary"))
	content := summary
	fallbackSource := ""
	// DM-20260630-011 AC1+AC2: when D7 marked summary as too_short /
	// inconclusive (LastTextQualityGate), treat the summary as empty
	// for IM card rendering purposes. The original summary is still
	// preserved on meta["summary"] for observability / CLI / transcript.
	if summaryQuality == "too_short" || summaryQuality == "inconclusive" {
		content = ""
	}
	if content == "" {
		content = strings.TrimSpace(event.Content)
		if content != "" && fallbackSource == "" {
			fallbackSource = "event.Content"
		}
	}
	if content == "" {
		content = stats
		if fallbackSource == "" {
			fallbackSource = "stats"
		}
	}
	if content == "" {
		fallbackSource = "event.Content_redacted"
	}
	meta := kernel.EnrichMetadata(map[string]string{
		"event_type":      "complete",
		"stats":           stats,
		"summary_quality": summaryQuality,
	}, kernel.SigOrEmpty(hasSig, sig))
	if summary != "" {
		meta["summary"] = summary
	}
	outbound := &types.OutboundMessage{
		MessageID:  kernel.NewMessageID(),
		SessionID:  session.SessionID,
		ChatID:     session.ChatID,
		Content:    content,
		IsComplete: true,
		Role:       types.MessageRoleAssistant,
		Metadata:   meta,
	}
	emit.OnMessage(outbound)
	emit.OnStatus(session.SessionID, types.SessionStateCompleted)
	// DM-20260630-011 AC2: emit the fallback span so Jaeger can
	// correlate. Only emitted when fallback actually occurred
	// (fallbackSource != ""). Uses background context so cancellation
	// of the inbound stream doesn't suppress the fallback span.
	if fallbackSource != "" {
		emitEmitCompleteFallback(
			context.Background(),
			session.SessionID,
			fallbackSource,
			summaryQuality,
			len([]rune(outbound.Content)),
		)
	}
}

// emitEmitCompleteFallback is the D1-side helper for the
// `D1_EmitComplete_Fallback` span. Mirrors hardening.EmitEmitCompleteFallback
// but lives in the communication package to avoid the D1↔orchestration
// import boundary forbidden by scripts/lint-d1-imports.sh (DM-20260628-003).
// No-ops when d1ObsBridge is nil.
func emitEmitCompleteFallback(ctx context.Context, sessionID, fallbackSource, summaryQuality string, contentLength int) {
	if d1ObsBridge == nil {
		return
	}
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "fallback.source", Value: fallbackSource},
		{Key: "fallback.content_length", Value: intToString(contentLength)},
		{Key: "summary_quality", Value: summaryQuality},
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1_S16_EmitComplete_Fallback, attrs...)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	if _, span := d1ObsBridge.Start(ctx, telemetry.OpD1_S16_EmitComplete_Fallback, opts...); span != nil {
		span.End()
	}
}

// intToString is a local copy of hardening.intToString (avoids the D1
// import boundary). Used only for span attribute formatting.
func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// EmitError maps S16-A02 FinalizeReply (error) → OutboundMessage.
func EmitError(session *types.Session, event *contracts.EngineEvent, sig contracts.IMOutboundSignal, hasSig bool, emit kernel.Emitter) {
	if session == nil || event == nil || emit == nil {
		return
	}
	meta := kernel.EnrichMetadata(map[string]string{"event_type": "error"}, kernel.SigOrEmpty(hasSig, sig))
	emit.OnMessage(&types.OutboundMessage{
		MessageID:  kernel.NewMessageID(),
		SessionID:  session.SessionID,
		ChatID:     session.ChatID,
		Content:    event.Content,
		IsComplete: true,
		Role:       types.MessageRoleAssistant,
		Metadata:   meta,
	})
}

// EmitInfo maps informational terminal messages (non-costly).
func EmitInfo(session *types.Session, event *contracts.EngineEvent, emit kernel.Emitter) {
	if session == nil || event == nil || emit == nil {
		return
	}
	emit.OnMessage(&types.OutboundMessage{
		MessageID:  kernel.NewMessageID(),
		SessionID:  session.SessionID,
		ChatID:     session.ChatID,
		Content:    event.Content,
		IsComplete: true,
		Role:       types.MessageRoleAssistant,
		Metadata:   kernel.OutboundMetadata("info", event.Metadata),
	})
}
