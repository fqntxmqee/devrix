package signal

import (
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/contracts"
)

func TestTurnTracker_Next_chainIntegrity(t *testing.T) {
	tr := NewTurnTracker()
	tr.BeginTurn("sess-1", "turn-1", time.Now())

	evThinking := &contracts.EngineEvent{Type: "thinking", Content: "hmm", SessionID: "sess-1"}
	sig1, r1, ok := tr.Next("sess-1", evThinking)
	if !ok || sig1.Kind != contracts.SignalThinking || !r1.Intact {
		t.Fatalf("thinking: ok=%v kind=%q intact=%v", ok, sig1.Kind, r1.Intact)
	}
	if sig1.SourceEventID == "" || sig1.ElapsedMs < 0 {
		t.Fatalf("expected objective anchors, got id=%q elapsed=%d", sig1.SourceEventID, sig1.ElapsedMs)
	}

	evTool := &contracts.EngineEvent{Type: "tool_call", SessionID: "sess-1", ToolName: "grep"}
	_, r2, ok := tr.Next("sess-1", evTool)
	if !ok || !r2.Intact || !r2.SawTask {
		t.Fatalf("task: ok=%v intact=%v sawTask=%v", ok, r2.Intact, r2.SawTask)
	}

	evComplete := &contracts.EngineEvent{Type: "complete", SessionID: "sess-1"}
	sig3, r3, ok := tr.Next("sess-1", evComplete)
	if !ok || !sig3.IsTerminal || sig3.Kind != contracts.SignalConclusion {
		t.Fatalf("complete: ok=%v terminal=%v kind=%q", ok, sig3.IsTerminal, sig3.Kind)
	}
	if !r3.SawConclusion {
		t.Fatal("expected sawConclusion")
	}
}

func TestTurnTracker_regKindBreak(t *testing.T) {
	tr := NewTurnTracker()
	tr.BeginTurn("s", "t", time.Now())
	tr.Next("s", &contracts.EngineEvent{Type: "complete", SessionID: "s"})
	_, r, ok := tr.Next("s", &contracts.EngineEvent{Type: "thinking", SessionID: "s"})
	if !ok || r.Intact {
		t.Fatalf("expected chain break after conclusion, intact=%v break=%q", r.Intact, r.BreakAt)
	}
}
