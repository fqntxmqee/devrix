//go:build integration && d7

package d7integration

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/tests/testutil"
)

func routeAndWait(t *testing.T, stack *testutil.D7TestStack, sessionID, content string) {
	t.Helper()
	if err := stack.Gateway.RouteInbound(context.Background(), &types.InboundMessage{
		SessionID: sessionID,
		ChatID:    "chat-d7",
		MessageID: "msg-d7",
		Content:   content,
		UserID:    "user-d7",
	}); err != nil {
		t.Fatalf("RouteInbound: %v", err)
	}
	stack.Gateway.WaitForProcesses()
	if !stack.Handler.WaitForMessages(1, 5*time.Second) {
		t.Fatal("expected outbound messages from D7 path")
	}
}
