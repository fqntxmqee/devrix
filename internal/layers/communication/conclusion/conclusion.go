package conclusion

import (
	"context"
	"strings"

	"github.com/devrix/devrix/internal/layers/communication/kernel"
	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

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
		end := hardening.EmitEmitCompleteFallback(
			context.Background(),
			session.SessionID,
			fallbackSource,
			summaryQuality,
			len([]rune(outbound.Content)),
		)
		end(nil)
	}
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
