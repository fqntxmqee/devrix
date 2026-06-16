package coordinator

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/wave"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// T: D7-S2-A01-T05 — OrchestratePath 走 SynthesizeTaskGraph + WaveScheduler。
// 验证 event sequence: plan_formed → wave_started → text → complete
// 且 fakeWaveScheduler.Start/WaitForCompletion 各被调用 1 次。
func TestOrchestratePath_Run_PipelineSequence(t *testing.T) {
	decomp := NewTaskDecomposer()
	sched := &fakeWaveScheduler{artifacts: nil}
	op := NewOrchestratePath(decomp, sched, nil)

	ch, err := op.Run(context.Background(), ProcessRequest{
		SessionID: "sess-orch",
		Message:   "fix bug in auth.go",
	}, IntentClassification{Kind: IntentOrchestrate})
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
	_, err := op.Run(context.Background(), ProcessRequest{
		SessionID: "s",
		Message:   "x",
	}, IntentClassification{Kind: IntentOrchestrate})
	if err == nil {
		t.Fatalf("expected error for nil decomposer")
	}
	if !strings.Contains(err.Error(), "decomposer") {
		t.Fatalf("err should mention decomposer, got %q", err.Error())
	}
}

func TestOrchestratePath_Run_NilScheduler(t *testing.T) {
	decomp := NewTaskDecomposer()
	op := &OrchestratePath{decomposer: decomp} // nil scheduler
	_, err := op.Run(context.Background(), ProcessRequest{
		SessionID: "s",
		Message:   "x",
	}, IntentClassification{Kind: IntentOrchestrate})
	if err == nil {
		t.Fatalf("expected error for nil scheduler")
	}
	if !strings.Contains(err.Error(), "scheduler") {
		t.Fatalf("err should mention scheduler, got %q", err.Error())
	}
}

// T: D7-S2-A01-T05 — OrchestratePath 在 Wave 失败时 emit error 事件。
func TestOrchestratePath_Run_WaveStartFails(t *testing.T) {
	decomp := NewTaskDecomposer()
	sched := &failingWaveScheduler{}
	op := NewOrchestratePath(decomp, sched, nil)

	ch, err := op.Run(context.Background(), ProcessRequest{
		SessionID: "s",
		Message:   "x",
	}, IntentClassification{Kind: IntentOrchestrate})
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
	artifacts := []wave.Artifact{
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

func (failingWaveScheduler) Start(_ context.Context, _ string, _ *wave.TaskGraph) error {
	return context.DeadlineExceeded
}
func (failingWaveScheduler) WaitForCompletion(_ context.Context, _ string) ([]wave.Artifact, error) {
	return nil, nil
}

// Ensure the contracts package is referenced (for downstream consumers
// reading the file).
var _ = contracts.EngineEvent{}
