package sessionorchestrator

import (
	"context"
	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// T: D7-S2-A01-T05 — OrchestratePath 走 SynthesizeTaskGraph + WaveScheduler。
// 验证 event sequence: plan_formed → wave_started → text → complete
// 且 fakeWaveScheduler.Start/WaitForCompletion 各被调用 1 次。
func TestOrchestratePath_Run_PipelineSequence(t *testing.T) {
	decomp := decisionplanning.NewTaskDecomposer()
	sched := &fakeWaveScheduler{artifacts: nil}
	op := NewOrchestratePath(decomp, sched, nil)

	ch, err := op.Run(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-orch",
		Message:   "fix bug in auth.go",
	}, orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate})
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}

	var types []string
	for ev := range ch {
		types = append(types, ev.Type)
	}
	// Empty artifacts → error (no worker output), not a misleading text reply.
	want := []string{"plan_formed", "wave_started", "error"}
	if !sequenceContains(types, want) {
		t.Fatalf("event sequence %v must contain %v in order", types, want)
	}

	// Scheduler was actually invoked.
	sched.mu.Lock()
	defer sched.mu.Unlock()
	if sched.starts != 1 {
		t.Fatalf("WaveScheduler.Start should be called once, got %d", sched.starts)
	}
	if sched.waits != 1 {
		t.Fatalf("WaveScheduler.WaitForCompletion should be called once, got %d", sched.waits)
	}
}

// T: D7-S2-A01-T05 — OrchestratePath nil guard。
func TestOrchestratePath_Run_NilDecomposer(t *testing.T) {
	sched := &fakeWaveScheduler{}
	op := &OrchestratePath{scheduler: sched} // nil decomposer
	_, err := op.Run(context.Background(), orchtypes.ProcessRequest{
		SessionID: "s",
		Message:   "x",
	}, orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate})
	if err == nil {
		t.Fatalf("expected error for nil decomposer")
	}
	if !strings.Contains(err.Error(), "decomposer") {
		t.Fatalf("err should mention decomposer, got %q", err.Error())
	}
}

func TestOrchestratePath_Run_NilScheduler(t *testing.T) {
	decomp := decisionplanning.NewTaskDecomposer()
	op := &OrchestratePath{decomposer: decomp} // nil scheduler
	_, err := op.Run(context.Background(), orchtypes.ProcessRequest{
		SessionID: "s",
		Message:   "x",
	}, orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate})
	if err == nil {
		t.Fatalf("expected error for nil scheduler")
	}
	if !strings.Contains(err.Error(), "scheduler") {
		t.Fatalf("err should mention scheduler, got %q", err.Error())
	}
}

// T: D7-S2-A01-T05 — OrchestratePath 在 Wave 失败时 emit error 事件。
func TestOrchestratePath_Run_WaveStartFails(t *testing.T) {
	decomp := decisionplanning.NewTaskDecomposer()
	sched := &failingWaveScheduler{}
	op := NewOrchestratePath(decomp, sched, nil)

	ch, err := op.Run(context.Background(), orchtypes.ProcessRequest{
		SessionID: "s",
		Message:   "x",
	}, orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate})
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	var types []string
	for ev := range ch {
		types = append(types, ev.Type)
	}
	// Should emit plan_formed (decomp succeeds) then error.
	if !contains(types, "error") {
		t.Fatalf("want error event, got %v", types)
	}
}

// T: D7-S2-A01-T05 — summarizeArtifacts 短文本输出，无 panic。
func TestOrchestratePath_SummarizeArtifacts(t *testing.T) {
	artifacts := []wavescheduler.Artifact{
		{
			TaskID:    "t1",
			Summary:   "fixed auth bug",
			ExitCode:  0,
			StartedAt: time.Now().Add(-2 * time.Second),
			EndedAt:   time.Now(),
		},
	}
	got := summarizeArtifacts(artifacts)
	if got != "fixed auth bug" {
		t.Fatalf("summary should be task output, got %q", got)
	}
}

func TestOrchestratePath_SummarizeEmpty(t *testing.T) {
	got := summarizeArtifacts(nil)
	if got != "(no artifacts)" {
		t.Fatalf("want '(no artifacts)', got %q", got)
	}
}

// sequenceContains reports whether `sub` appears in `all` preserving order.
// Subset check; allows extra events between subsequence elements.
func sequenceContains(all, sub []string) bool {
	i, j := 0, 0
	for i < len(all) && j < len(sub) {
		if all[i] == sub[j] {
			j++
		}
		i++
	}
	return j == len(sub)
}

// failingWaveScheduler returns an error from Start; used to verify the
// error branch of OrchestratePath.Run.
type failingWaveScheduler struct{}

func (failingWaveScheduler) Start(_ context.Context, _ string, _ *wavescheduler.TaskGraph) error {
	return context.DeadlineExceeded
}
func (failingWaveScheduler) WaitForCompletion(_ context.Context, _ string) ([]wavescheduler.Artifact, error) {
	return nil, nil
}

// Ensure the contracts package is referenced (for downstream consumers
// reading the file).
var _ = contracts.EngineEvent{}

// T: regression — DM-20260626-002. OrchestratePath.Run must NOT emit a
// consolidated "text" event after the wave finishes; the worker's text
// events (streamed through workerEventToEngine) are the only source of
// user-facing text. Emitting a second text event from summarizeArtifacts
// produced a 61207 vs 61212 byte duplicate in the feishu card (the
// extra 5 bytes came from outputParts joining the worker's "result"
// with the runner's "done" complete-event content).
func TestOrchestratePath_NoDuplicateTextOnCompletion(t *testing.T) {
	decomp := decisionplanning.NewTaskDecomposer()
	sched := &fakeWaveScheduler{artifacts: []wavescheduler.Artifact{{
		TaskID:    "t1",
		Summary:   "the result",
		ExitCode:  0,
		StartedAt: time.Now().Add(-2 * time.Second),
		EndedAt:   time.Now(),
	}}}
	op := NewOrchestratePath(decomp, sched, nil)

	ch, err := op.Run(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-no-dup",
		Message:   "do something",
	}, orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate})
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}

	var textCount int
	var textContents []string
	for ev := range ch {
		if ev.Type == "text" {
			textCount++
			textContents = append(textContents, ev.Content)
		}
	}
	if textCount != 0 {
		t.Fatalf("OrchestratePath must NOT emit text events (the worker streams them); got %d: %v", textCount, textContents)
	}
}

// T: regression — workerEventToEngine must pass through "text" events so
// worker output (bash results, LLM text) reaches the Feishu card. Before
// the fix, only thinking/tool_use/error were mapped; text was silently
// dropped, leaving the user staring at a stale "started" card.
func TestWorkerEventToEngine_PassesText(t *testing.T) {
	cases := []struct {
		name     string
		inType   string
		wantType string
		wantNil  bool
	}{
		{"text passes through", "text", "text", false},
		{"thinking passes through", "thinking", "thinking", false},
		// 2026-06-26 hotfix: worker "tool_use" must surface as
		// EngineEvent "tool_call" so the D1 feishu task-card surface
		// and SignalRouter (which only recognise tool_call/tool_result)
		// actually render the entry. Otherwise the event reaches
		// handleEngineEvent with Type="tool_use", falls into the router
		// default branch, and is silently dropped — task card stays
		// empty during subagent tool calls.
		{"tool_use maps to tool_call", "tool_use", "tool_call", false},
		{"error passes through", "error", "error", false},
		{"complete filtered", "complete", "", true},
		{"cancelled filtered", "cancelled", "", true},
		{"unknown filtered", "unknown_type", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := workerEventToEngine("sess-x", "task-y",
				wavescheduler.WorkerEvent{Type: tc.inType, Content: "hello"})
			if tc.wantNil {
				if ev != nil {
					t.Fatalf("type=%q should map to nil, got %+v", tc.inType, ev)
				}
				return
			}
			if ev == nil {
				t.Fatalf("type=%q should produce an EngineEvent, got nil", tc.inType)
			}
			if ev.Type != tc.wantType {
				t.Errorf("Type=%q, want %q", ev.Type, tc.wantType)
			}
			if ev.Content != "hello" {
				t.Errorf("Content=%q, want hello", ev.Content)
			}
			if ev.SessionID != "sess-x" {
				t.Errorf("SessionID=%q, want sess-x", ev.SessionID)
			}
			if ev.Metadata["wave_task_id"] != "task-y" {
				t.Errorf("metadata.wave_task_id=%q, want task-y", ev.Metadata["wave_task_id"])
			}
		})
	}
}

// TestWorkerEventToEngine_ToolUseCarriesToolName (2026-06-26 hotfix): a
// worker `tool_use` event with ToolName/ToolInput must produce a
// `tool_call` EngineEvent that surfaces them. Otherwise the D1 feishu
// task card has no name to render and the user cannot tell which tool
// ran.
func TestWorkerEventToEngine_ToolUseCarriesToolName(t *testing.T) {
	ev := workerEventToEngine("sess-x", "task-y",
		wavescheduler.WorkerEvent{
			Type:      "tool_use",
			Content:   "running ls",
			ToolName:  "bash",
			ToolInput: `{"cmd":"ls -la"}`,
		})
	if ev == nil {
		t.Fatal("tool_use must produce an EngineEvent, got nil")
	}
	if ev.Type != "tool_call" {
		t.Errorf("Type=%q, want tool_call", ev.Type)
	}
	if ev.ToolName != "bash" {
		t.Errorf("ToolName=%q, want bash", ev.ToolName)
	}
	if ev.ToolInput != `{"cmd":"ls -la"}` {
		t.Errorf("ToolInput=%q, want %q", ev.ToolInput, `{"cmd":"ls -la"}`)
	}
	if ev.Metadata["tool_name"] != "bash" {
		t.Errorf("metadata.tool_name=%q, want bash", ev.Metadata["tool_name"])
	}
}
