package taskprogress

import (
	"github.com/devrix/devrix/internal/layers/communication/kernel"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// EmitToolCall maps S15-A01 EmitToolProgress (tool_call) → OutboundMessage.
func EmitToolCall(session *types.Session, event *contracts.EngineEvent, sig contracts.IMOutboundSignal, hasSig bool, emit kernel.Emitter) {
	if session == nil || event == nil || emit == nil {
		return
	}
	toolName := event.ToolName
	if toolName == "" && event.Metadata != nil {
		toolName = event.Metadata["tool_name"]
	}
	meta := kernel.EnrichMetadata(map[string]string{
		"event_type": "tool_call",
		"tool_name":  toolName,
		"input":      kernel.MetaField(event.Metadata, "input"),
	}, kernel.SigOrEmpty(hasSig, sig))
	emit.OnMessage(&types.OutboundMessage{
		MessageID:  kernel.NewMessageID(),
		SessionID:  session.SessionID,
		ChatID:     session.ChatID,
		Content:    toolName,
		IsComplete: false,
		Role:       types.MessageRoleAssistant,
		Metadata:   meta,
	})
}

// EmitToolResult maps S15-A01 EmitToolProgress (tool_result) → OutboundMessage.
func EmitToolResult(session *types.Session, event *contracts.EngineEvent, sig contracts.IMOutboundSignal, hasSig bool, emit kernel.Emitter) {
	if session == nil || event == nil || emit == nil {
		return
	}
	toolName := event.ToolName
	if toolName == "" && event.Metadata != nil {
		toolName = event.Metadata["tool_name"]
	}
	meta := kernel.EnrichMetadata(map[string]string{
		"event_type": "tool_result",
		"tool_name":  toolName,
	}, kernel.SigOrEmpty(hasSig, sig))
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

// EmitMilestoneProgress maps S15-A01-F03 EmitMilestoneCardProgress → OutboundMessage.
func EmitMilestoneProgress(session *types.Session, event *contracts.EngineEvent, sig contracts.IMOutboundSignal, hasSig bool, emit kernel.Emitter) {
	if session == nil || event == nil || emit == nil {
		return
	}
	meta := map[string]string{
		"event_type":         "milestone_progress",
		"progress":           kernel.MetaField(event.Metadata, "progress"),
		"task":               kernel.MetaField(event.Metadata, "task"),
		"milestone_id":       kernel.MetaField(event.Metadata, "milestone_id"),
		"render":             "milestone",
		"milestone_name":     kernel.MetaField(event.Metadata, "task"),
		"milestone_progress": kernel.MetaField(event.Metadata, "progress"),
		"milestone_status":   string(types.MilestoneStatusInProgress),
	}
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

// EmitWorkerProgress maps S15-A02 EmitWorkerProgress → OutboundMessage.
func EmitWorkerProgress(session *types.Session, event *contracts.EngineEvent, sig contracts.IMOutboundSignal, hasSig bool, emit kernel.Emitter) {
	if session == nil || event == nil || emit == nil {
		return
	}
	meta := map[string]string{"event_type": "worker_progress"}
	if event.Metadata != nil {
		for k, v := range event.Metadata {
			meta[k] = v
		}
	}
	if meta["render"] == "" {
		meta["render"] = "worker_tree"
	}
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
