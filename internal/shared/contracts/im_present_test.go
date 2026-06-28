package contracts

import "testing"

func TestWorkerKind_Valid(t *testing.T) {
	if !WorkerKindSubAgent.Valid() {
		t.Fatal("subagent should be valid")
	}
	if WorkerKind("unknown").Valid() {
		t.Fatal("unknown kind should be invalid")
	}
}

func TestWorkerStreamEvent_Fields(t *testing.T) {
	ev := WorkerStreamEvent{Type: "thinking", Content: "plan"}
	if ev.Type != "thinking" || ev.Content != "plan" {
		t.Fatalf("unexpected event: %+v", ev)
	}
}
