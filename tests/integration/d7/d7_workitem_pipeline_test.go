//go:build integration && d7

package d7integration

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/tests/testutil"
)

func stackWaitTimeout() time.Duration {
	return 10 * time.Second
}

func TestIntegration_WorkItemPipeline_BootstrapWired(t *testing.T) {
	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{WorkItemPipeline: true})
	entry, ok := stack.Gateway.OrchestrationEntry().(*sessionorchestrator.Entry)
	if !ok {
		t.Fatalf("expected *sessionorchestrator.Entry, got %T", stack.Gateway.OrchestrationEntry())
	}
	tm := entry.TaskManager()
	if tm == nil {
		t.Fatal("orchestrator TaskManager not wired")
	}

	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	ctx := context.Background()
	if err := stack.Gateway.RouteInbound(ctx, &types.InboundMessage{
		SessionID: session.SessionID,
		ChatID:    "chat-wip",
		MessageID: "msg-wip",
		Content:   "implement lightweight cache for session metadata",
		UserID:    "integration-user",
	}); err != nil {
		t.Fatalf("RouteInbound: %v", err)
	}
	stack.Gateway.WaitForProcesses()
	if !stack.Handler.WaitForMessages(1, stackWaitTimeout()) {
		t.Fatal("expected outbound messages from work item pipeline path")
	}

	foundGoal := false
	for _, item := range tm.Tree().List(session.SessionID) {
		if item != nil && item.Kind == workmodel.WorkKindGoal {
			foundGoal = true
			if item.LastRound == nil {
				t.Log("goal has no LastRound yet (may be ok for short-circuit path)")
			}
		}
	}
	if !foundGoal {
		t.Fatal("expected goal work item in orchestrator TaskManager")
	}
}
