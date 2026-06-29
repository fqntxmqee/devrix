package sessionorchestrator

import (
	"context"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/materialize"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// BuildSubTurnMaterializeRequest maps a SubTurn request to D2 Materialize input (D7-S16-A65).
func BuildSubTurnMaterializeRequest(req contracts.SubTurnRequest, mode string, tokenBudget int) materialize.Request {
	agentID := subTurnAgentID(req)
	return materialize.Request{
		Partition: materialize.Partition{
			SessionID: req.SessionID,
			Kind:      materialize.PartitionAgent,
			AgentID:   agentID,
		},
		Policy:        materialize.PolicyFromSubTurnMode(mode, tokenBudget),
		SystemPrompt:  req.SystemPrompt,
		SubTurnParent: append([]types.Message(nil), req.Messages...),
	}
}

func subTurnAgentID(req contracts.SubTurnRequest) string {
	if strings.TrimSpace(req.AgentID) != "" {
		return req.AgentID
	}
	return "subturn:" + req.SessionID
}

func (r *SubTurnRunner) materializeSubTurnContext(ctx context.Context, req contracts.SubTurnRequest, mode string, tokenBudget int) (systemPrompt string, preloaded []types.Message, lastUser types.Message, ok bool) {
	if r == nil || r.Materializer == nil {
		return "", nil, types.Message{}, false
	}
	matReq := BuildSubTurnMaterializeRequest(req, mode, tokenBudget)
	res, err := r.Materializer.Materialize(ctx, matReq)
	if err != nil {
		return "", nil, types.Message{}, false
	}
	preloaded, lastUser = splitMaterializedSubTurn(res.Messages)
	return res.SystemPrompt, preloaded, lastUser, true
}

func splitMaterializedSubTurn(msgs []types.Message) (preloaded []types.Message, lastUser types.Message) {
	if len(msgs) == 0 {
		return nil, types.Message{}
	}
	lastUser = lastUserMessage(msgs)
	preloaded = messagesWithoutLastUser(msgs)
	return preloaded, lastUser
}
