package legacy

import (
	stderrors "errors"
	"log/slog"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

func infoEvent(sessionID, content string) *contracts.EngineEvent {
	return &contracts.EngineEvent{
		Type:      "info",
		Content:   content,
		SessionID: sessionID,
		Metadata:  map[string]string{"category": "context"},
	}
}

func errorEvent(sessionID string, err *errors.SentinelError, recoverable bool) *contracts.EngineEvent {
	rec := "false"
	if recoverable {
		rec = "true"
	}
	return &contracts.EngineEvent{
		Type:      "error",
		Content:   err.Error(),
		SessionID: sessionID,
		Metadata: map[string]string{
			"code":        err.Code,
			"recoverable": rec,
		},
	}
}

func lastAssistantContent(msgs []types.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == types.MessageRoleAssistant && msgs[i].Content != "" {
			return msgs[i].Content
		}
	}
	return ""
}

func stripSystemMessage(msgs []types.Message) []types.Message {
	if len(msgs) == 0 {
		return msgs
	}
	if msgs[0].Role == types.MessageRoleSystem {
		return append([]types.Message(nil), msgs[1:]...)
	}
	return msgs
}

func mapProcessError(sessionID string, err error) *contracts.EngineEvent {
	if err == nil {
		return nil
	}
	var se *errors.SentinelError
	if stderrors.As(err, &se) {
		return errorEvent(sessionID, se, false)
	}
	msg := errors.FormatLLMError(err)
	if msg == "" {
		msg = err.Error()
	}
	slog.Warn("contextengine: process failed", "sessionID", sessionID, "error", msg)
	return errorEvent(sessionID, errors.WithCode("CTX_PROCESS_FAILED", msg, err), false)
}
