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

// TaskIncompleteMessage is rendered on the IM "任务总结" card when D7
// flagged BOTH summary AND final Content as bad (transitional / planning
// text leaked to the user — e.g. sess_1782826968112_7000 emitted "Now let
// me look at the cross-package contracts…" as its 82-char "conclusion").
// The message is intentionally short and bilingual-friendly so it works
// across IM adapters without needing per-locale wiring.
const TaskIncompleteMessage = "（任务未能完成，AI 未产生有效结论。请重新发起。）"

// CompleteEventSourceObservationalAnswerFastPath mirrors the D7
// sessionorchestrator completeEventSourceObservationalAnswerFastPath
// constant. D1 conclusion.EmitComplete checks this source on
// event.Metadata["source"] to suppress the task_incomplete override when
// the answer came from the DM-20260706-011 observational_answer fast-path.
// The fast-path answer is structurally pre-validated by
// pickHighStrengthBusinessFact (strength ≥ 0.9, CatBusiness ObsFact, no
// ObsUncertainty, VerdictPass) and intentionally short — the
// 100-rune too_short threshold does not apply. The string value MUST stay
// in sync with the D7 side; if either side changes, both must change
// together (no compile-time check across the D1↔D7 boundary).
const CompleteEventSourceObservationalAnswerFastPath = "observational_answer_fastpath"

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
//
// DM-20260630-011 follow-up: when summary_quality AND final_quality are
// BOTH bad (LLM ended mid-tool-call with transitional text — see
// sess_1782826968112_7000), the original fallback chain forwards the
// transitional phrase to Feishu, leaving the user with a meaningless
// fragment instead of an answer. Detect this case and replace the
// fallback Content with a clear "task incomplete" message so the user
// knows to retry, and surface task_incomplete on the metadata so D6
// Evolution / dashboards can alert on the pattern.
func EmitComplete(session *types.Session, event *contracts.EngineEvent, sig contracts.IMOutboundSignal, hasSig bool, emit kernel.Emitter) {
	if session == nil || event == nil || emit == nil {
		return
	}
	usage := kernel.MetaField(event.Metadata, "usage")
	duration := kernel.MetaField(event.Metadata, "duration")
	model := kernel.MetaField(event.Metadata, "model")
	ctxPct := kernel.MetaField(event.Metadata, "ctx_pct")
	summaryQuality := kernel.MetaField(event.Metadata, "summary_quality")
	finalQuality := kernel.MetaField(event.Metadata, "final_quality")
	// DM-20260708-002: read the D7-set source field so the
	// task_incomplete override at the bottom of this function can be
	// suppressed for the observational_answer fast-path. The source is
	// a contract between D7 buildSessionCompleteEvent and D1
	// EmitComplete — the string value must match
	// CompleteEventSourceObservationalAnswerFastPath.
	source := kernel.MetaField(event.Metadata, "source")
	stats := BuildCompletionSummary(duration, usage, model, ctxPct)
	summary := strings.TrimSpace(kernel.MetaField(event.Metadata, "summary"))
	content := summary
	fallbackSource := ""
	taskIncomplete := false
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
	// DM-20260630-011 follow-up: when BOTH summary and Content classify
	// as bad (transitional text leaked to the user), replace with a
	// clear "task incomplete" message. We classify Content using the
	// final_quality signal D7 propagated; that's cheaper than re-running
	// the structural classifier here and avoids re-importing
	// orchestration/ into D1 (forbidden by scripts/lint-d1-imports.sh).
	//
	// DM-20260708-002 fast-path bypass: when event.Metadata["source"] is
	// the observational_answer fast-path, the answer is structurally
	// pre-validated (CatBusiness ObsFact, strength ≥ 0.9, no ObsUncertainty,
	// VerdictPass) and the short final is the actual answer (e.g. "2×3=6",
	// "巴黎是法国首都"). The 100-rune too_short threshold is for LLM
	// transitional text, not fast-path answers — replacing it with
	// TaskIncompleteMessage would mask a correct answer as a failure
	// (screenshot 2026-07-08, sess_1783502345285_0). Source is a
	// machine-set field on the D7 buildSessionCompleteEvent contract; the
	// D1↔D7 string value is the same.
	if summaryQuality == "too_short" || summaryQuality == "inconclusive" {
		if finalQuality == "too_short" || finalQuality == "inconclusive" {
			if source == CompleteEventSourceObservationalAnswerFastPath {
				// Fast-path: keep the answer (already in `content` after
				// the summary→Content fallback above). Mark
				// task_incomplete=false explicitly even if it was
				// hypothetically set earlier (defensive — current code
				// doesn't set it).
				taskIncomplete = false
				fallbackSource = "fastpath_answer_preserved"
			} else {
				content = TaskIncompleteMessage
				taskIncomplete = true
				if fallbackSource == "" {
					fallbackSource = "task_incomplete"
				} else {
					fallbackSource = fallbackSource + "+task_incomplete"
				}
			}
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
	if taskIncomplete {
		meta["task_incomplete"] = "true"
	}
	if finalQuality != "" {
		meta["final_quality"] = finalQuality
	}
	// DM-20260708-002: propagate the D7-set source to the outbound
	// message metadata so D6 Evolution / dashboards can count fast-path
	// traffic independently of summary_quality. Mirrors the propagation
	// of summary_quality / final_quality above.
	if source != "" {
		meta["source"] = source
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
