package sessionorchestrator

import (
	"context"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

const maxOutputTokenRecoveryAttempts = 3

// compressMessagesForRecovery applies a one-shot D7-side compress when LLM
// rejects the prompt for length. Uses the same runCompress degradation ladder.
func (o *DefaultOrchestrator) compressMessagesForRecovery(ctx context.Context, req TurnRequest, messages []types.Message) []types.Message {
	if o == nil || len(messages) == 0 {
		return messages
	}
	result := o.runCompress(ctx, req, &CompressHint{
		MessagesToSummarize: messages,
		TargetTokenBudget:   maxTruncatedMessages * 256,
	})
	if result.Summary == "" {
		return messages
	}
	return []types.Message{{
		SessionID: req.SessionID,
		Role:      types.MessageRoleSystem,
		Content:   result.Summary,
	}}
}

// partialStreamEmit tracks events already sent to the UI during a stream
// attempt that may be rolled back before recovery retry (TD-QL-06).
type partialStreamEmit struct {
	hadThinking bool
	hadText     bool
	toolCalls   []llmgateway.ToolCall
}

func emitStreamRecoveryTombstones(out chan<- *contracts.EngineEvent, sessionID string, partial partialStreamEmit) {
	if partial.hadThinking {
		out <- &contracts.EngineEvent{
			Type:      "tombstone",
			SessionID: sessionID,
			Metadata:  map[string]string{"rollback": "thinking"},
		}
	}
	if partial.hadText {
		out <- &contracts.EngineEvent{
			Type:      "tombstone",
			SessionID: sessionID,
			Metadata:  map[string]string{"rollback": "text"},
		}
	}
	for _, tc := range partial.toolCalls {
		out <- &contracts.EngineEvent{
			Type:      "tombstone",
			ToolName:  tc.Name,
			SessionID: sessionID,
			Metadata: map[string]string{
				"rollback":  "tool_call",
				"tool_name": tc.Name,
			},
		}
	}
}

func (o *DefaultOrchestrator) invokeStreamWithRecovery(
	ctx context.Context,
	req TurnRequest,
	invokeReq LLMInvokeRequest,
) (<-chan llmgateway.Chunk, error) {
	ch, err := o.llm.InvokeStream(ctx, invokeReq)
	if err == nil || !hardening.IsContextLengthError(err) {
		return ch, err
	}
	compressed := o.compressMessagesForRecovery(ctx, req, invokeReq.Messages)
	if len(compressed) == len(invokeReq.Messages) {
		return nil, err
	}
	invokeReq.Messages = compressed
	return o.llm.InvokeStream(ctx, invokeReq)
}