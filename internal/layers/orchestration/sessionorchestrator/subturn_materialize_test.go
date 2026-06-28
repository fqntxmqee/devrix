package sessionorchestrator

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/materialize"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestSubTurnRunner_MaterializePath_Brief(t *testing.T) {
	llm, runner := buildSubTurnFixture(t)
	runner.Materializer = materialize.NewDefaultMaterializer(nil)

	_, err := runner.RunSubTurn(context.Background(), contracts.SubTurnRequest{
		SessionID: "s1",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "do work"}},
		Mode:      contracts.SubAgentModeBrief,
	})
	if err != nil {
		t.Fatal(err)
	}
	msgs := llm.lastMessages()
	if len(msgs) != 1 || msgs[0].Content != "do work" {
		t.Fatalf("LLM messages = %+v, want single user do work", msgs)
	}
}

func TestBuildSubTurnMaterializeRequest_AgentPartition(t *testing.T) {
	req := contracts.SubTurnRequest{
		SessionID:    "sess",
		AgentID:      "explore-1",
		SystemPrompt: "sys",
		Messages:     []types.Message{{Role: types.MessageRoleUser, Content: "x"}},
	}
	matReq := BuildSubTurnMaterializeRequest(req, "fork", 12000)
	if matReq.Partition.Kind != materialize.PartitionAgent {
		t.Fatalf("kind = %q", matReq.Partition.Kind)
	}
	if matReq.Partition.AgentID != "explore-1" {
		t.Fatalf("agent id = %q", matReq.Partition.AgentID)
	}
	if matReq.Policy.Mode != materialize.ModeFork {
		t.Fatalf("mode = %q", matReq.Policy.Mode)
	}
}
