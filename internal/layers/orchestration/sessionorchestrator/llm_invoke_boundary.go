package sessionorchestrator

import (
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/usercontext"
	"github.com/devrix/devrix/internal/shared/types"
)

// messagesForLLMInvoke applies runtime user-context prepend at the D7→D3 API
// boundary (AGENTS.md when user_context.mode=prepend|both). Prepend is not
// persisted in snapshot Messages.
//
// All LLM InvokeStream call sites in this package must route through this
// helper so prepend policy stays single-sourced:
//   - DefaultWorkItemExecutor (Feishu / MUPS main path)
//   - DefaultOrchestrator.runLLMStream (sub-agent RunTurn path)
//
// HasMetaUserContext inside usercontext.MessagesWithUserContext is an
// idempotency guard for materialized/history messages that already carry a
// meta prepend block — not dedup between the two executors above.
func messagesForLLMInvoke(msgs []types.Message, userContextPrepend map[string]string) []types.Message {
	msgs = conversation.RepairToolMessageChain(msgs)
	return usercontext.MessagesWithUserContext(msgs, userContextPrepend)
}
