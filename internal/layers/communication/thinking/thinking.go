package thinking

import (
	"github.com/devrix/devrix/internal/layers/communication/kernel"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// EmitThinking maps S14-A01 EmitThinkingDelta → OutboundMessage.
//
// DSAFT: D1-S14 PresentThinking
func EmitThinking(session *types.Session, event *contracts.EngineEvent, sig contracts.IMOutboundSignal, hasSig bool, emit kernel.Emitter) {
	if session == nil || event == nil || emit == nil {
		return
	}
	meta := kernel.OutboundMetadata("thinking", event.Metadata)
	if hasSig {
		meta = kernel.EnrichMetadata(meta, sig)
	}
	emit.OnMessage(&types.OutboundMessage{
		MessageID:  kernel.NewMessageID(),
		SessionID:  session.SessionID,
		ChatID:     session.ChatID,
		Content:    event.Content,
		IsComplete: false,
		Role:       types.MessageRoleAssistant,
		Metadata:   meta,
	})
}
