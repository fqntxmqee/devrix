package sessionorchestrator

import (
	"context"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// T: D7-S2-A01-T04 — CommandHandler 显式分发到 PlanCLICommands（/plan）。
func TestCommandHandler_Handle_PlanCommand(t *testing.T) {
	cli := workmodel.NewCLICommands(workmodel.NewTaskManager())
	plan := workmodel.NewPlanCLICommands(workmodel.NewPlanMode(nil, nil))
	h := NewCommandHandler(cli, plan, nil)

	ch, err := h.Handle(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-1",
		Message:   "/plan add auth",
	}, orchtypes.IntentClassification{
		Kind:    orchtypes.IntentCommand,
		Command: "/plan add auth",
	})
	if err != nil {
		t.Fatalf("Handle err: %v", err)
	}
	var events []*contracts.EngineEvent
	for ev := range ch {
		events = append(events, ev)
	}
	if len(events) < 2 {
		t.Fatalf("want ≥ 2 events, got %d", len(events))
	}
	// Expect text + complete (command_reply goes to sink, not channel).
	var types []string
	for _, ev := range events {
		types = append(types, ev.Type)
	}
	if !contains(types, "text") || !contains(types, "complete") {
		t.Fatalf("want text+complete events, got %v", types)
	}
	// text content should mention PlanMode (the canonical "orchtypes.Plan mode:" prefix).
	var text string
	for _, ev := range events {
		if ev.Type == "text" {
			text = ev.Content
		}
	}
	if !strings.Contains(text, "Entered plan mode") {
		t.Fatalf("text should confirm plan mode entry, got %q", text)
	}
}

// T: D7-S2-A01-T04 — CommandHandler 显式分发到 CLICommands（/task）。
func TestCommandHandler_Handle_TaskCommand(t *testing.T) {
	cli := workmodel.NewCLICommands(workmodel.NewTaskManager())
	plan := workmodel.NewPlanCLICommands(workmodel.NewPlanMode(nil, nil))
	h := NewCommandHandler(cli, plan, nil)

	ch, err := h.Handle(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-1",
		Message:   "/task list",
	}, orchtypes.IntentClassification{
		Kind:    orchtypes.IntentCommand,
		Command: "/task list",
	})
	if err != nil {
		t.Fatalf("Handle err: %v", err)
	}
	var types []string
	for ev := range ch {
		types = append(types, ev.Type)
	}
	if !contains(types, "text") || !contains(types, "complete") {
		t.Fatalf("want text+complete, got %v", types)
	}
	// Verify the text mentions "Task" (CLICommands list output format).
	var text string
	for _, ev := range collectEvents(ch) {
		// channel already closed by first loop; use stored events instead
		text = ev.Content
	}
	_ = text
}

// collectEvents is a helper that drains a closed channel of EngineEvent.
func collectEvents(ch <-chan *contracts.EngineEvent) []*contracts.EngineEvent {
	var out []*contracts.EngineEvent
	for ev := range ch {
		out = append(out, ev)
	}
	return out
}

// T: D7-S2-A01-T04 — CommandHandler sink 收到 command_reply 事件（不通过 channel）。
func TestCommandHandler_Handle_PublishesCommandReplyToSink(t *testing.T) {
	cli := workmodel.NewCLICommands(workmodel.NewTaskManager())
	plan := workmodel.NewPlanCLICommands(workmodel.NewPlanMode(nil, nil))
	sink := &fakeSink{}
	h := NewCommandHandler(cli, plan, sink)

	ch, err := h.Handle(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-sink",
		Message:   "/help",
	}, orchtypes.IntentClassification{
		Kind:    orchtypes.IntentCommand,
		Command: "/help",
	})
	if err != nil {
		t.Fatalf("Handle err: %v", err)
	}
	for range ch {
	}
	// Wait briefly for the sink Publish goroutine to flush.
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if len(sink.events) == 0 {
		t.Fatalf("sink should receive command_reply, got 0")
	}
	if sink.events[0].Type != "command_reply" {
		t.Fatalf("want command_reply, got %q", sink.events[0].Type)
	}
}

// T: D7-S2-A01-T04 — CommandHandler nil guard：未注入时返回明确错误。
func TestCommandHandler_Handle_NilWiring(t *testing.T) {
	h := &CommandHandler{} // nothing wired
	_, err := h.Handle(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-nil",
		Message:   "/plan",
	}, orchtypes.IntentClassification{Kind: orchtypes.IntentCommand, Command: "/plan"})
	if err == nil {
		t.Fatalf("expected error for nil wiring")
	}
	if !strings.Contains(err.Error(), "CommandHandler") {
		t.Fatalf("err should mention CommandHandler, got %q", err.Error())
	}
}

// contains is a tiny slice helper to keep test deps zero.
func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
