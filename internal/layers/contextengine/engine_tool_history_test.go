package contextengine

import (
	"encoding/json"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/snapshot"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestEngineToolHistorySync_should_not_duplicate_tool_messages_without_call_id(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	mgr := memory.NewManager(cfg, snapshot.NewStore(&cfg.Snapshot), nil)
	sc := &types.SessionContext{
		SessionID: "sess_tool_hist",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "first"}},
		TokenBudget: types.DefaultTokenBudget(),
	}

	callID := "call_cursor_0"
	tcJSON, _ := json.Marshal([]map[string]string{
		{
			"id":   callID,
			"type": "function",
		},
	})
	tcMsg := types.Message{
		Role:     types.MessageRoleAssistant,
		Metadata: map[string]string{"tool_calls": string(tcJSON)},
	}
	mgr.AppendFullMessage(sc, tcMsg)

	resultMsg := types.Message{
		Role:     types.MessageRoleTool,
		Content:  "ok",
		Metadata: map[string]string{"tool_call_id": callID},
	}
	mgr.AppendFullMessage(sc, resultMsg)

	if len(sc.Messages) != 3 {
		t.Fatalf("message count = %d, want 3", len(sc.Messages))
	}
	toolMsg := sc.Messages[2]
	if toolMsg.Role != types.MessageRoleTool {
		t.Fatalf("third message role = %s", toolMsg.Role)
	}
	if toolMsg.Metadata["tool_call_id"] != callID {
		t.Fatalf("tool_call_id = %q", toolMsg.Metadata["tool_call_id"])
	}
}
