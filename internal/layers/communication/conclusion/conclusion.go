package conclusion

import (
	"github.com/devrix/devrix/internal/layers/communication/kernel"
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
func EmitComplete(session *types.Session, event *contracts.EngineEvent, sig contracts.IMOutboundSignal, hasSig bool, emit kernel.Emitter) {
	if session == nil || event == nil || emit == nil {
		return
	}
	usage := kernel.MetaField(event.Metadata, "usage")
	duration := kernel.MetaField(event.Metadata, "duration")
	model := kernel.MetaField(event.Metadata, "model")
	ctxPct := kernel.MetaField(event.Metadata, "ctx_pct")
	summary := BuildCompletionSummary(duration, usage, model, ctxPct)
	meta := kernel.EnrichMetadata(map[string]string{"event_type": "complete"}, kernel.SigOrEmpty(hasSig, sig))
	emit.OnMessage(&types.OutboundMessage{
		MessageID:  kernel.NewMessageID(),
		SessionID:  session.SessionID,
		ChatID:     session.ChatID,
		Content:    summary,
		IsComplete: true,
		Role:       types.MessageRoleAssistant,
		Metadata:   meta,
	})
	emit.OnStatus(session.SessionID, types.SessionStateCompleted)
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
