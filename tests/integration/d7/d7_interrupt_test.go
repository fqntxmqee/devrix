//go:build integration && d7

package d7integration

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/tests/testutil"
)

// T: D7-S2-T04, D7-S2-A03-T01
func TestIntegration_D7Interrupt_StopDuringSlowLLM(t *testing.T) {
	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{
		LLMStub: &testutil.D7LLMStub{
			Response: "should not arrive",
			Delay:    2 * time.Second,
		},
	})
	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = stack.Gateway.RouteInbound(context.Background(), &types.InboundMessage{
			SessionID: session.SessionID,
			ChatID:    "chat-stop",
			MessageID: "msg-stop",
			Content:   "long running",
			UserID:    "user-d7",
		})
	}()

	time.Sleep(100 * time.Millisecond)
	if err := stack.Gateway.StopProcess(session.SessionID); err != nil {
		t.Fatalf("StopProcess: %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RouteInbound did not return after StopProcess")
	}
	stack.Gateway.WaitForProcesses()
}
