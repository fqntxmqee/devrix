//go:build integration && d7

package d7integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/tests/testutil"
)

func TestIntegration_DeliverableConvergence_CompleteNotTransition(t *testing.T) {
	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{WorkItemPipeline: true})
	entry, ok := stack.Gateway.OrchestrationEntry().(*sessionorchestrator.Entry)
	if !ok {
		t.Fatalf("expected *sessionorchestrator.Entry, got %T", stack.Gateway.OrchestrationEntry())
	}
	_ = entry

	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	ctx := context.Background()
	if err := stack.Gateway.RouteInbound(ctx, &types.InboundMessage{
		SessionID: session.SessionID,
		ChatID:    "chat-deliverable",
		MessageID: "msg-deliverable",
		Content:   "review internal/layers/contextengine/kernel/ for P0/P1 issues with file:line citations",
		UserID:    "integration-user",
	}); err != nil {
		t.Fatalf("RouteInbound: %v", err)
	}
	stack.Gateway.WaitForProcesses()
	if !stack.Handler.WaitForMessages(1, 15*time.Second) {
		t.Fatal("expected outbound messages")
	}

	var completeContent string
	for _, m := range stack.Handler.OutboundMessages() {
		if m == nil || m.Metadata["event_type"] != "complete" {
			continue
		}
		completeContent = m.Content
	}
	if completeContent == "" {
		t.Fatal("missing complete message")
	}
	lower := strings.ToLower(completeContent)
	for _, bad := range []string{"let me continue", "继续探索", "任务未能完成"} {
		if strings.Contains(lower, bad) {
			t.Fatalf("complete should not contain %q: %s", bad, completeContent)
		}
	}
}
