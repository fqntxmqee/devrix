package enforce_test

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestRunBackground_should_enqueue_notification_on_complete(t *testing.T) {
	llm := &query.SequentialLLM{Responses: []query.LLMScript{{Content: "bg done"}}}
	loop := &query.Loop{LLM: llm, Permission: query.AllowPermission{}}
	reg := enforce.NewBackgroundRegistry()
	sq := newTestSessionQueue()
	parent := &types.SessionContext{SessionID: "sess_bg", Model: "test"}

	taskID, err := enforce.RunBackground(context.Background(), enforce.LoopDeps{Loop: loop}, enforce.SubQueryParams{
		ParentSC:       parent,
		AgentID:        "explore_bg",
		AgentName:      "Explore",
		SystemPrompt:   "bg",
		PromptMessages: []types.Message{{Role: types.MessageRoleUser, Content: "work"}},
		MaxTurns:       2,
	}, reg, sq)
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if drained := sq.Drain("sess_bg", "explore_bg", false); len(drained) > 0 {
			if task, ok := reg.Get(taskID); !ok || task.Status != "completed" {
				t.Fatalf("task should be completed: %+v", task)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected task-notification to be enqueued")
}
