// Package sessionorchestrator — DM-20260707-001 PR-C tests.
//
// Coverage targets (consensus packet §6.2):
//
//   - emit_dedup_test.go:    MarkAndCheck / Reset / Len / nil-receiver safety /
//                            concurrent safety (race detector).
//   - streaming_key_test.go: NewPartialIdempotencyKey / NewRollupIdempotencyKey
//                            keyshape matches adapters package (cross-package
//                            dedup drift prevention).
//   - execute_plan_dag_test.go: smoke test — happy path produces a rollup
//                            round with the right VerdictKind, dedup, and
//                            per-segment Learn attribution.
//
// The deeper integration tests (PlanDAG construction, real wavescheduler
// fan-out) live in wavescheduler/dag_executor_test.go — PR-C only adds the
// sessionorchestrator-side consumer; the scheduler-side path is already
// covered by PR-B.
package sessionorchestrator

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	ifaces "github.com/devrix/devrix/internal/layers/orchestration/interfaces"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// =====================================================================
// emit_dedup_test.go — D7-S15-A?? (PR-C T01: 2-layer dedup)
// =====================================================================

func TestEmitDedup_MarkAndCheck_FirstHitReturnsTrue(t *testing.T) {
	d := NewEmitDedup()
	if !d.MarkAndCheck("sess:seg:1") {
		t.Fatal("first MarkAndCheck must return true")
	}
}

func TestEmitDedup_MarkAndCheck_DuplicateReturnsFalse(t *testing.T) {
	d := NewEmitDedup()
	if !d.MarkAndCheck("sess:seg:1") {
		t.Fatal("first call must miss")
	}
	if d.MarkAndCheck("sess:seg:1") {
		t.Fatal("second call must hit (dedup)")
	}
}

func TestEmitDedup_HitMissCounters(t *testing.T) {
	d := NewEmitDedup()
	d.MarkAndCheck("k1") // miss
	d.MarkAndCheck("k1") // hit
	d.MarkAndCheck("k2") // miss
	d.MarkAndCheck("k1") // hit
	if got := d.MissCount(); got != 2 {
		t.Errorf("MissCount = %d, want 2", got)
	}
	if got := d.HitCount(); got != 2 {
		t.Errorf("HitCount = %d, want 2", got)
	}
	if got := d.Len(); got != 2 {
		t.Errorf("Len = %d, want 2", got)
	}
}

func TestEmitDedup_Reset(t *testing.T) {
	d := NewEmitDedup()
	d.MarkAndCheck("k1")
	d.MarkAndCheck("k1")
	d.Reset()
	if !d.MarkAndCheck("k1") {
		t.Fatal("post-Reset first call must miss")
	}
	if d.Len() != 1 {
		t.Errorf("post-Reset Len = %d, want 1", d.Len())
	}
}

func TestEmitDedup_NilReceiverSafe(t *testing.T) {
	var d *EmitDedup // nil
	if !d.MarkAndCheck("anything") {
		t.Fatal("nil-receiver MarkAndCheck must return true (treat as miss)")
	}
	if d.Len() != 0 {
		t.Errorf("nil-receiver Len = %d, want 0", d.Len())
	}
	if d.HitCount() != 0 || d.MissCount() != 0 {
		t.Errorf("nil-receiver counters must be 0")
	}
	// Reset on nil must not panic.
	d.Reset()
}

func TestEmitDedup_ConcurrentSafety(t *testing.T) {
	d := NewEmitDedup()
	const goroutines = 16
	const keysPerG = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			for i := 0; i < keysPerG; i++ {
				d.MarkAndCheck("k")
			}
		}(g)
	}
	wg.Wait()
	if d.Len() != 1 {
		t.Errorf("after concurrent inserts on same key, Len = %d, want 1", d.Len())
	}
	// Exactly ONE miss (the first inserter), the rest are hits.
	if got := d.MissCount() + d.HitCount(); got != int64(goroutines*keysPerG) {
		t.Errorf("counter sum = %d, want %d", got, goroutines*keysPerG)
	}
	if d.MissCount() != 1 {
		t.Errorf("MissCount = %d, want 1 (sync.Map.LoadOrStore first inserter wins)", d.MissCount())
	}
}

// =====================================================================
// streaming_key_test.go — D7-S15-A?? (PR-C keyshape parity with adapters)
// =====================================================================

func TestStreamingKey_FormatMatchesAdaptersShape(t *testing.T) {
	// The keyshape "{sessionID}:seg:{segmentID}" is the single shared
	// keyspace between sessionorchestrator and adapters.FeishuAdapter.
	// If the shape drifts, the IM-side dedup table misses the dedup.
	got := NewPartialIdempotencyKey("sess_abc", "seg_42")
	want := "sess_abc:seg:seg_42"
	if got != want {
		t.Errorf("NewPartialIdempotencyKey = %q, want %q", got, want)
	}
}

func TestStreamingKey_RollupKeyPrefixAvoidsCollision(t *testing.T) {
	// "rollup:" prefix must not collide with "seg:" partial keys.
	partial := NewPartialIdempotencyKey("sess", "abc")
	rollup := NewRollupIdempotencyKey("sess", "abc")
	if partial == rollup {
		t.Fatal("partial and rollup keys must be distinct for the same parentID")
	}
	if !strings.Contains(rollup, ":rollup:") {
		t.Errorf("rollup key missing :rollup: prefix: %q", rollup)
	}
	if !strings.Contains(partial, ":seg:") {
		t.Errorf("partial key missing :seg: prefix: %q", partial)
	}
}

func TestStreamingKey_EmptyInputsFormatSafely(t *testing.T) {
	// The formatters don't validate; the dedup layer catches empty-key
	// misuse at the API boundary. We just confirm the format is stable.
	got := NewPartialIdempotencyKey("", "")
	if got != ":seg:" {
		t.Errorf("empty inputs format = %q, want \":seg:\"", got)
	}
}

// =====================================================================
// execute_plan_dag_test.go — happy path smoke + dedup + rollup verdict
// =====================================================================

// stubDAGExecutor is a controllable DAGExecutor for executePlanDAG tests.
// It feeds a pre-canned list of SegmentEmits down the channel then closes.
type stubDAGExecutor struct {
	emits []wavescheduler.SegmentEmit
	// err is returned synchronously (mirrors RunPlanDAG conversion errors).
	err error
}

func (s *stubDAGExecutor) RunPlanDAG(
	_ context.Context,
	_, _ string,
	_ *plan.PlanDAG,
	_ *ifaces.IntentSegmentSet,
) (<-chan wavescheduler.SegmentEmit, error) {
	if s.err != nil {
		return nil, s.err
	}
	ch := make(chan wavescheduler.SegmentEmit, len(s.emits))
	for _, e := range s.emits {
		ch <- e
	}
	close(ch)
	return ch, nil
}

// stubStreamingEmitter is a recording StreamingEmitter for the DAG path.
// It captures partial/final emits so tests can assert content + keyshape.
type stubStreamingEmitter struct {
	mu       sync.Mutex
	partials []stubEmitterCall
	finals   []stubEmitterCall
	partialErr error // if non-nil, EmitPartialCard returns this
	finalErr   error // if non-nil, EmitFinalCard returns this
}

type stubEmitterCall struct {
	ChatID    string
	IdemKey   string
	Content   string
}

func (s *stubStreamingEmitter) EmitPartialCard(
	_ context.Context, chatID, idemKey, content string,
) (*FeishuEmitPartialResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.partials = append(s.partials, stubEmitterCall{chatID, idemKey, content})
	if s.partialErr != nil {
		return nil, s.partialErr
	}
	return &FeishuEmitPartialResult{CardID: "card-partial-" + idemKey, Sequence: 1}, nil
}

func (s *stubStreamingEmitter) EmitFinalCard(
	_ context.Context, chatID, idemKey, content string,
) (*FeishuEmitPartialResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finals = append(s.finals, stubEmitterCall{chatID, idemKey, content})
	if s.finalErr != nil {
		return nil, s.finalErr
	}
	return &FeishuEmitPartialResult{CardID: "card-final-" + idemKey, Sequence: 1}, nil
}

func (s *stubStreamingEmitter) partialCalls() []stubEmitterCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]stubEmitterCall(nil), s.partials...)
}

func (s *stubStreamingEmitter) finalCalls() []stubEmitterCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]stubEmitterCall(nil), s.finals...)
}

// stubSessionChatID verifies executePlanDAG passes sessionID as chatID.
// (Verified implicitly via the stub call records above.)

func dagTestPlan(segIDs ...string) *plan.Plan {
	p := plan.NewPlan("plan_dag_test", "sess_dag_test", plan.CommitmentPlan, []string{}, []plan.Step{}, 0.95)
	p.DAG = &plan.PlanDAG{Nodes: make([]plan.PlanNode, 0, len(segIDs))}
	segs := make([]ifaces.IntentSegment, 0, len(segIDs))
	for _, id := range segIDs {
		p.DAG.Nodes = append(p.DAG.Nodes, plan.PlanNode{ID: id, SegmentID: id})
		segs = append(segs, ifaces.NewIntentSegment(id, "text-"+id, ifaces.IntentSegmentKindDeterministic))
	}
	p.IntentSegmentSet = &ifaces.IntentSegmentSet{
		Segments:        segs,
		SourceDirective: "test directive",
		DetectedAt:      time.Now(),
	}
	return p
}

func dagTestItem(t *testing.T, tm *workmodel.TaskManager, sessionID, directive string) *workmodel.WorkItem {
	t.Helper()
	item, err := tm.EnsureGoal(sessionID, directive)
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}
	return item
}

func TestExecutePlanDAG_HappyPath_ThreePassChildren_OneFinal(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)

	executor := &stubDAGExecutor{emits: []wavescheduler.SegmentEmit{
		{SessionID: "sess_dag_test", SegmentID: "seg_a", WorkerType: wavescheduler.WorkerSubAgent,
			Summary: "answer-a", ExitCode: 0, StartedAt: time.Now(), EndedAt: time.Now()},
		{SessionID: "sess_dag_test", SegmentID: "seg_b", WorkerType: wavescheduler.WorkerSubAgent,
			Summary: "answer-b", ExitCode: 0, StartedAt: time.Now(), EndedAt: time.Now()},
		{SessionID: "sess_dag_test", SegmentID: "seg_c", WorkerType: wavescheduler.WorkerSubAgent,
			Summary: "answer-c", ExitCode: 0, StartedAt: time.Now(), EndedAt: time.Now(), IsFinal: true},
	}}
	emitter := &stubStreamingEmitter{}
	runner.DAGExecutor = executor
	runner.StreamingEmitter = emitter

	pl := dagTestPlan("seg_a", "seg_b", "seg_c")
	item := dagTestItem(t, tm, "sess_dag_test", "multi-intent query")

	round, err := runner.executePlanDAG(
		context.Background(), "sess_dag_test", "u_test",
		item, pl, "multi-intent query", 1, "test_trigger",
		time.Now(), false, nil)
	if err != nil {
		t.Fatalf("executePlanDAG: %v", err)
	}

	// Rollup verdict: all 3 children Pass → VerdictPass.
	if round.VerdictKind != types.VerdictPass {
		t.Errorf("rollup VerdictKind = %v, want VerdictPass", round.VerdictKind)
	}
	if round.ArtifactSummary != "answer-c" {
		t.Errorf("ArtifactSummary = %q, want IsFinal child text", round.ArtifactSummary)
	}
	if round.ExitReason != "dag_rollup" {
		t.Errorf("ExitReason = %q, want dag_rollup", round.ExitReason)
	}
	// EmitPartialCard: 3 children, but the IsFinal one is suppressed
	// (the rollup's EmitFinalCard overrides it).
	if got := len(emitter.partialCalls()); got != 2 {
		t.Errorf("partial emit count = %d, want 2 (IsFinal suppressed)", got)
	}
	if got := len(emitter.finalCalls()); got != 1 {
		t.Errorf("final emit count = %d, want 1", got)
	}
	// Final key uses rollup shape.
	final := emitter.finalCalls()[0]
	if final.IdemKey != "sess_dag_test:rollup:"+item.ID {
		t.Errorf("final idemKey = %q, want sess_dag_test:rollup:%s", final.IdemKey, item.ID)
	}
	// Partial keys use seg shape.
	for _, p := range emitter.partialCalls() {
		if !strings.Contains(p.IdemKey, ":seg:") {
			t.Errorf("partial idemKey missing :seg: prefix: %q", p.IdemKey)
		}
	}
}

func TestExecutePlanDAG_ChildFailure_FlipsRollupToFail(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	executor := &stubDAGExecutor{emits: []wavescheduler.SegmentEmit{
		{SessionID: "sess_fail", SegmentID: "seg_a", WorkerType: wavescheduler.WorkerSubAgent,
			Summary: "answer-a", ExitCode: 0, StartedAt: time.Now(), EndedAt: time.Now()},
		{SessionID: "sess_fail", SegmentID: "seg_b", WorkerType: wavescheduler.WorkerSubAgent,
			Summary: "", ExitCode: -1, Error: "boom", StartedAt: time.Now(), EndedAt: time.Now(), IsFinal: true},
	}}
	emitter := &stubStreamingEmitter{}
	runner.DAGExecutor = executor
	runner.StreamingEmitter = emitter

	pl := dagTestPlan("seg_a", "seg_b")
	item := dagTestItem(t, tm, "sess_fail", "failing directive")

	round, err := runner.executePlanDAG(
		context.Background(), "sess_fail", "u",
		item, pl, "failing directive", 1, "trigger", time.Now(), false, nil)
	if err != nil {
		t.Fatalf("executePlanDAG: %v", err)
	}
	if round.VerdictKind != types.VerdictFail {
		t.Errorf("rollup VerdictKind = %v, want VerdictFail (child failed)", round.VerdictKind)
	}
}

func TestExecutePlanDAG_Dedup_DropsDuplicateEmits(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	executor := &stubDAGExecutor{emits: []wavescheduler.SegmentEmit{
		// Same SegmentID twice — second must be dropped by EmitDedup.
		{SessionID: "sess_dup", SegmentID: "seg_a", WorkerType: wavescheduler.WorkerSubAgent,
			Summary: "first", ExitCode: 0, StartedAt: time.Now(), EndedAt: time.Now()},
		{SessionID: "sess_dup", SegmentID: "seg_a", WorkerType: wavescheduler.WorkerSubAgent,
			Summary: "dup", ExitCode: 0, StartedAt: time.Now(), EndedAt: time.Now()},
		{SessionID: "sess_dup", SegmentID: "seg_b", WorkerType: wavescheduler.WorkerSubAgent,
			Summary: "answer-b", ExitCode: 0, StartedAt: time.Now(), EndedAt: time.Now(), IsFinal: true},
	}}
	emitter := &stubStreamingEmitter{}
	runner.DAGExecutor = executor
	runner.StreamingEmitter = emitter

	pl := dagTestPlan("seg_a", "seg_b")
	item := dagTestItem(t, tm, "sess_dup", "dup query")

	round, err := runner.executePlanDAG(
		context.Background(), "sess_dup", "u",
		item, pl, "dup query", 1, "trigger", time.Now(), false, nil)
	if err != nil {
		t.Fatalf("executePlanDAG: %v", err)
	}
	if round.VerdictKind != types.VerdictPass {
		t.Errorf("rollup VerdictKind = %v, want VerdictPass", round.VerdictKind)
	}
	// Only 2 partials (seg_a + seg_b's IsFinal suppressed). The duplicate
	// seg_a is dropped by dedup.
	if got := len(emitter.partialCalls()); got != 1 {
		t.Errorf("partial emits = %d, want 1 (dedup dropped the duplicate)", got)
	}
	if emitter.partialCalls()[0].Content != "first" {
		t.Errorf("partial content = %q, want \"first\" (the first observation wins)",
			emitter.partialCalls()[0].Content)
	}
}

func TestExecutePlanDAG_NilStreamingEmitter_StillProducesRound(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	executor := &stubDAGExecutor{emits: []wavescheduler.SegmentEmit{
		{SessionID: "sess_noemit", SegmentID: "seg_a", WorkerType: wavescheduler.WorkerSubAgent,
			Summary: "answer", ExitCode: 0, StartedAt: time.Now(), EndedAt: time.Now(), IsFinal: true},
	}}
	// StreamingEmitter intentionally nil — exercises the no-op fallback.
	runner.DAGExecutor = executor
	// runner.StreamingEmitter = nil (default)

	pl := dagTestPlan("seg_a")
	item := dagTestItem(t, tm, "sess_noemit", "no-emit query")

	round, err := runner.executePlanDAG(
		context.Background(), "sess_noemit", "u",
		item, pl, "no-emit query", 1, "trigger", time.Now(), false, nil)
	if err != nil {
		t.Fatalf("executePlanDAG: %v", err)
	}
	if round == nil {
		t.Fatal("round must not be nil even when StreamingEmitter is nil")
	}
	if round.VerdictKind != types.VerdictPass {
		t.Errorf("VerdictKind = %v, want VerdictPass", round.VerdictKind)
	}
}

func TestExecutePlanDAG_NilRunnerReturnsError(t *testing.T) {
	var r *ItemPipelineRunner
	_, err := r.executePlanDAG(
		context.Background(), "s", "u", nil, nil, "", 1, "t",
		time.Now(), false, nil)
	if err == nil {
		t.Fatal("nil-runner executePlanDAG must return error")
	}
}

func TestExecutePlanDAG_MissingDAGReturnsError(t *testing.T) {
	runner, _, _ := newItemPipelineTestRunner(t)
	runner.DAGExecutor = &stubDAGExecutor{}
	// Plan with nil DAG — executePlanDAG must reject.
	pl := plan.NewPlan("p", "sess", plan.CommitmentPlan, nil, nil, 0.5)
	_, err := runner.executePlanDAG(
		context.Background(), "sess", "u", nil, pl, "", 1, "t",
		time.Now(), false, nil)
	if err == nil {
		t.Fatal("executePlanDAG with nil DAG must return error")
	}
	if !strings.Contains(err.Error(), "DAG") {
		t.Errorf("error %q should mention DAG", err.Error())
	}
}

func TestExecutePlanDAG_RunPlanDAGErrorBubblesUp(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	runner.DAGExecutor = &stubDAGExecutor{err: context.DeadlineExceeded}
	pl := dagTestPlan("seg_a")
	item := dagTestItem(t, tm, "sess_deadline", "deadline query")
	_, err := runner.executePlanDAG(
		context.Background(), "sess_deadline", "u",
		item, pl, "deadline query", 1, "t", time.Now(), false, nil)
	if err == nil {
		t.Fatal("RunPlanDAG error must bubble up")
	}
	if !strings.Contains(err.Error(), "RunPlanDAG") {
		t.Errorf("error %q should wrap RunPlanDAG", err.Error())
	}
}

func TestExecutePlanDAG_PerSegmentLearn_AttributionIncludesSegmentID(t *testing.T) {
	// Verifies the AssetBuilder wires SegmentID onto LearnRequest so
	// reputation records track which segments contributed. We capture the
	// LearnRequest via a recording learner stub.
	runner, tm, _ := newItemPipelineTestRunner(t)
	recorded := &recordingLearner{inner: runner.Learner}
	runner.Learner = recorded

	executor := &stubDAGExecutor{emits: []wavescheduler.SegmentEmit{
		{SessionID: "sess_attr", SegmentID: "seg_a", WorkerType: wavescheduler.WorkerSubAgent,
			Summary: "ok-a", ExitCode: 0, StartedAt: time.Now(), EndedAt: time.Now()},
		{SessionID: "sess_attr", SegmentID: "seg_b", WorkerType: wavescheduler.WorkerSubAgent,
			Summary: "ok-b", ExitCode: 0, StartedAt: time.Now(), EndedAt: time.Now(), IsFinal: true},
	}}
	runner.DAGExecutor = executor

	pl := dagTestPlan("seg_a", "seg_b")
	item := dagTestItem(t, tm, "sess_attr", "attr query")
	_, err := runner.executePlanDAG(
		context.Background(), "sess_attr", "u",
		item, pl, "attr query", 1, "t", time.Now(), false, nil)
	if err != nil {
		t.Fatalf("executePlanDAG: %v", err)
	}

	// Expect 2 per-segment + 1 rollup Learn calls.
	if got := len(recorded.calls); got != 3 {
		t.Fatalf("Learn call count = %d, want 3 (2 per-segment + 1 rollup)", got)
	}
	// Per-segment calls: SegmentID set, IsRollup=false.
	for i, c := range recorded.calls[:2] {
		if c.req.IsRollup {
			t.Errorf("call[%d] IsRollup = true, want false (per-segment)", i)
		}
		if c.req.SegmentID == "" {
			t.Errorf("call[%d] SegmentID is empty", i)
		}
		if c.req.Evidence != nil {
			t.Errorf("call[%d] Evidence must be nil for per-segment", i)
		}
	}
	// Rollup call: IsRollup=true, Evidence non-nil with both segment IDs.
	rollup := recorded.calls[2]
	if !rollup.req.IsRollup {
		t.Error("rollup Learn call: IsRollup must be true")
	}
	if rollup.req.Evidence == nil {
		t.Fatal("rollup Learn call: Evidence must be non-nil")
	}
	if got := len(rollup.req.Evidence.SegmentIDs); got != 2 {
		t.Errorf("rollup Evidence.SegmentIDs = %v, want 2", rollup.req.Evidence.SegmentIDs)
	}
}

// recordingLearner wraps a real learn.Learner and captures every LearnRequest
// passed through. Lets tests assert per-segment attribution without mocking
// the whole LP-1 pipeline.
type recordingLearner struct {
	inner learn.Learner
	mu    sync.Mutex
	calls []recordingLearnCall
}

type recordingLearnCall struct {
	req learn.LearnRequest
}

func (r *recordingLearner) Learn(ctx context.Context, req learn.LearnRequest) ([]*learn.LearningAsset, error) {
	r.mu.Lock()
	r.calls = append(r.calls, recordingLearnCall{req: req})
	r.mu.Unlock()
	return r.inner.Learn(ctx, req)
}

func (r *recordingLearner) Inject(ctx context.Context, sessionID, trackModeHint string) (*learn.AdaptivePrior, error) {
	return r.inner.Inject(ctx, sessionID, trackModeHint)
}

func (r *recordingLearner) ScheduledTick(ctx context.Context) error {
	return r.inner.ScheduledTick(ctx)
}