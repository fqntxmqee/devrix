package wavescheduler

import "github.com/devrix/devrix/internal/shared/contracts"

// WorkerKindToPresentation maps a D7 worker type to the D1 presentation DTO.
func WorkerKindToPresentation(wt WorkerType) contracts.WorkerKind {
	return contracts.WorkerKind(wt)
}

// WorkerEventToStream maps a D7 worker event to the D1 presentation DTO.
func WorkerEventToStream(ev WorkerEvent) contracts.WorkerStreamEvent {
	return contracts.WorkerStreamEvent{
		Type:      ev.Type,
		Content:   ev.Content,
		ToolName:  ev.ToolName,
		ToolInput: ev.ToolInput,
	}
}

// WorkerCardOptsFromPresentation builds card opts from contracts DTO fields.
func WorkerCardOptsFromPresentation(sessionID, taskID, workerID string, kind contracts.WorkerKind, title string) contracts.WorkerCardOpts {
	return contracts.WorkerCardOpts{
		SessionID:  sessionID,
		TaskID:     taskID,
		WorkerID:   workerID,
		WorkerKind: kind,
		Title:      title,
	}
}
