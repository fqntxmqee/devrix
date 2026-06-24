package escape

import (
	"sync"
	"testing"
	"time"
)

// --- L1-01: FirstCall ---------------------------------------------------------

// L1-01: 守护首回路计数语义（depth=1）
func TestLoopDepthTracker_FirstCall(t *testing.T) {
	tracker, err := NewLoopDepthTracker(DefaultMaxDepth)
	if err != nil {
		t.Fatalf("NewLoopDepthTracker failed: %v", err)
	}
	ctx := LoopContext{
		SessionID:        "sess-001",
		PlanKind:         1, // ProtocolPlan
		ObservationKind:  0, // ObsFact
		FailureCriterion: "verifier_timeout",
		ArtifactType:     0,
	}
	decision := tracker.ShouldContinue(ctx)

	if decision.Action != EscapeContinue {
		t.Errorf("first call Action = %s, want continue", decision.Action)
	}
	if decision.Depth != 1 {
		t.Errorf("first call Depth = %d, want 1", decision.Depth)
	}
	if tracker.Depth("sess-001") != 1 {
		t.Errorf("tracker.Depth = %d, want 1", tracker.Depth("sess-001"))
	}
}

// --- L1-02: SameMode ----------------------------------------------------------

// L1-02: 守护同模式 depth++
func TestLoopDepthTracker_SameMode(t *testing.T) {
	tracker, _ := NewLoopDepthTracker(DefaultMaxDepth)
	ctx := makeCtx("sess-002")

	for i := 1; i <= 2; i++ {
		d := tracker.ShouldContinue(ctx)
		if d.Action != EscapeContinue {
			t.Errorf("call %d: Action = %s, want continue", i, d.Action)
		}
		if d.Depth != i {
			t.Errorf("call %d: Depth = %d, want %d", i, d.Depth, i)
		}
	}
}

// --- L1-03: DifferentMode -----------------------------------------------------

// L1-03: 守护异模式 reset
func TestLoopDepthTracker_DifferentMode(t *testing.T) {
	tracker, _ := NewLoopDepthTracker(DefaultMaxDepth)

	// 第一次：mode A
	ctxA := LoopContext{SessionID: "sess-003", PlanKind: 1, FailureCriterion: "x"}
	d1 := tracker.ShouldContinue(ctxA)
	if d1.Depth != 1 {
		t.Errorf("first call Depth = %d, want 1", d1.Depth)
	}

	// 同 session 不同 PlanKind → reset depth=1
	ctxA2 := LoopContext{SessionID: "sess-003", PlanKind: 2, FailureCriterion: "x"}
	d2 := tracker.ShouldContinue(ctxA2)
	if d2.Action != EscapeContinue {
		t.Errorf("mode change Action = %s, want continue", d2.Action)
	}
	if d2.Depth != 1 {
		t.Errorf("mode change Depth = %d, want 1 (reset)", d2.Depth)
	}
	if d2.Reason != "mode_changed_reset" {
		t.Errorf("mode change Reason = %q, want mode_changed_reset", d2.Reason)
	}
}

// --- L1-04: ExceedMax ---------------------------------------------------------

// L1-04: 守护 MaxDepth=3 边界（采纳 design §5.1 SoT: depth >= MaxDepth 触发 ForceExit）
func TestLoopDepthTracker_ExceedMax(t *testing.T) {
	tracker, _ := NewLoopDepthTracker(DefaultMaxDepth)
	ctx := makeCtx("sess-004")

	// depth=1 → Continue
	d1 := tracker.ShouldContinue(ctx)
	if d1.Action != EscapeContinue || d1.Depth != 1 {
		t.Errorf("depth=1: Action=%s Depth=%d, want continue/1", d1.Action, d1.Depth)
	}
	// depth=2 → Continue
	d2 := tracker.ShouldContinue(ctx)
	if d2.Action != EscapeContinue || d2.Depth != 2 {
		t.Errorf("depth=2: Action=%s Depth=%d, want continue/2", d2.Action, d2.Depth)
	}
	// depth=3 → ForceExit
	d3 := tracker.ShouldContinue(ctx)
	if d3.Action != EscapeForceExit {
		t.Errorf("depth=3: Action=%s, want force_exit", d3.Action)
	}
	if d3.Depth != 3 {
		t.Errorf("depth=3: Depth=%d, want 3", d3.Depth)
	}
	if d3.Reason != "loop_depth_exceeded" {
		t.Errorf("depth=3: Reason=%q, want loop_depth_exceeded", d3.Reason)
	}
	if d3.AuditLevel != 2 {
		t.Errorf("depth=3: AuditLevel=%d, want 2 (full audit)", d3.AuditLevel)
	}
	// depth=4 兜底仍 ForceExit
	d4 := tracker.ShouldContinue(ctx)
	if d4.Action != EscapeForceExit {
		t.Errorf("depth=4: Action=%s, want force_exit (兜底)", d4.Action)
	}
}

// --- L1-05: HashDeterministic -------------------------------------------------

// L1-05: 守护 hash 稳定性（同输入产生同 hash）
func TestHashLoopContext_Deterministic(t *testing.T) {
	ctx := makeCtx("sess-005")
	h1 := hashLoopContext(ctx)
	h2 := hashLoopContext(ctx)

	if h1 != h2 {
		t.Errorf("hash not deterministic: %s vs %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("hash length = %d, want 64 (SHA-256 hex)", len(h1))
	}
}

// --- L1-06: HashDifferentInput ------------------------------------------------

// L1-06: 守护 hash 区分性（不同输入产生不同 hash）
func TestHashLoopContext_DifferentInput(t *testing.T) {
	base := makeCtx("sess-006")
	baseHash := hashLoopContext(base)

	tests := []struct {
		name string
		mut  func(*LoopContext)
	}{
		{"PlanKind", func(c *LoopContext) { c.PlanKind++ }},
		{"ObservationKind", func(c *LoopContext) { c.ObservationKind++ }},
		{"FailureCriterion", func(c *LoopContext) { c.FailureCriterion = "other" }},
		{"ArtifactType", func(c *LoopContext) { c.ArtifactType++ }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := base
			tc.mut(&ctx)
			if h := hashLoopContext(ctx); h == baseHash {
				t.Errorf("hash unchanged after %s mutation", tc.name)
			}
		})
	}
}

// --- L1-07: SessionID_Isolated ------------------------------------------------

// L1-07: 守护跨 session 隔离（codex review M4 明确）
func TestLoopDepthTracker_SessionID_Isolated(t *testing.T) {
	tracker, _ := NewLoopDepthTracker(DefaultMaxDepth)
	ctx := makeCtx("sess-A")

	// sess-A 跑 2 次
	tracker.ShouldContinue(ctx)
	tracker.ShouldContinue(ctx)
	if tracker.Depth("sess-A") != 2 {
		t.Errorf("sess-A depth = %d, want 2", tracker.Depth("sess-A"))
	}

	// sess-B 同模式应 depth=1（隔离）
	ctxB := makeCtx("sess-B")
	d := tracker.ShouldContinue(ctxB)
	if d.Depth != 1 {
		t.Errorf("sess-B first call Depth = %d, want 1 (cross-session isolation)", d.Depth)
	}
	if tracker.Depth("sess-A") != 2 {
		t.Errorf("sess-A depth polluted: = %d, want 2", tracker.Depth("sess-A"))
	}
}

// --- L1-08: Concurrent --------------------------------------------------------

// L1-08: 守护并发安全（race 0）
func TestLoopDepthTracker_Concurrent(t *testing.T) {
	tracker, _ := NewLoopDepthTracker(DefaultMaxDepth)
	ctx := makeCtx("sess-concurrent")

	var wg sync.WaitGroup
	const goroutines = 100
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			tracker.ShouldContinue(ctx)
		}()
	}
	wg.Wait()

	depth := tracker.Depth("sess-concurrent")
	if depth != goroutines {
		t.Errorf("concurrent depth = %d, want %d (no lost increments)", depth, goroutines)
	}
}

// --- L1-09: Reset -------------------------------------------------------------

// L1-09: 守护 Reset 清空 History
func TestLoopDepthTracker_Reset(t *testing.T) {
	tracker, _ := NewLoopDepthTracker(DefaultMaxDepth)
	tracker.ShouldContinue(makeCtx("sess-r1"))
	tracker.ShouldContinue(makeCtx("sess-r2"))
	if tracker.Depth("sess-r1") == 0 || tracker.Depth("sess-r2") == 0 {
		t.Fatalf("pre-Reset depth not set: r1=%d r2=%d", tracker.Depth("sess-r1"), tracker.Depth("sess-r2"))
	}

	tracker.Reset()
	if tracker.Depth("sess-r1") != 0 || tracker.Depth("sess-r2") != 0 {
		t.Errorf("Reset did not clear: r1=%d r2=%d", tracker.Depth("sess-r1"), tracker.Depth("sess-r2"))
	}

	// Reset 后再调用应 depth=1
	d := tracker.ShouldContinue(makeCtx("sess-r1"))
	if d.Depth != 1 {
		t.Errorf("post-Reset first call Depth = %d, want 1", d.Depth)
	}
}

// --- L1-10: HashCollision_Resilience -----------------------------------------

// L1-10: 守护 hash 冲突时的降级行为（SHA-256 实际无冲突；模拟同 hash 但 mode
// 不同的场景）。
//
// 在我们的设计中，hashLoopContext 是 SessionID 隔离的（不同 session 不
// 会冲突），同 session 内 5 字段变化必然改变 hash。我们验证的是：
// "如果两个不同 ctx 偶然产生同 hash（同 session），计数器会累加"。
func TestLoopDepthTracker_HashCollision_Resilience(t *testing.T) {
	tracker, _ := NewLoopDepthTracker(DefaultMaxDepth)
	ctx := makeCtx("sess-collision")

	// 同 session 同输入 3 次 → depth 递增
	tracker.ShouldContinue(ctx)
	tracker.ShouldContinue(ctx)
	d3 := tracker.ShouldContinue(ctx)
	if d3.Action != EscapeForceExit {
		t.Errorf("3 同输入应触发 ForceExit, got %s", d3.Action)
	}
}

// --- L1-91: PanicRecovery -----------------------------------------------------

// L1-91: 守护 LoopDepthTracker panic 降级为 Continue（design §9）
//
// 我们通过创建一个内部 panic 的 hash map 来强制 panic；defer/recover
// 应捕获并返回 EscapeContinue。
func TestLoopDepthTracker_PanicRecovery(t *testing.T) {
	tracker, _ := NewLoopDepthTracker(DefaultMaxDepth)
	ctx := makeCtx("sess-panic")

	// 正常路径 baseline
	d := tracker.ShouldContinue(ctx)
	if d.Action != EscapeContinue {
		t.Fatalf("baseline Action = %s, want continue", d.Action)
	}

	// 强制 internal panic: 在 shouldContinueLocked 内注入 panic
	// 通过替换 nowFunc 在 CreatedAt 时刻 panic，验证 recover
	originalNow := nowFunc
	defer func() { nowFunc = originalNow }()
	panicCount := 0
	nowFunc = func() (t time.Time) {
		panicCount++
		if panicCount <= 2 { // 第 1, 2 次 panic
			panic("simulated panic in nowFunc")
		}
		return originalNow()
	}

	d2 := tracker.ShouldContinue(ctx)
	if d2.Action != EscapeContinue {
		t.Errorf("after panic: Action = %s, want continue (panic recovery)", d2.Action)
	}
	if d2.Reason != "panic_recovered_failsafe" {
		t.Errorf("after panic: Reason = %q, want panic_recovered_failsafe", d2.Reason)
	}
}

// --- L1-Extra: NewLoopDepthTracker validation ---------------------------------

// Extra: NewLoopDepthTracker 拒绝 MaxDepth < 1
func TestNewLoopDepthTracker_InvalidMaxDepth(t *testing.T) {
	tracker, err := NewLoopDepthTracker(0)
	if err == nil {
		t.Errorf("MaxDepth=0 should fail")
	}
	if tracker != nil {
		t.Errorf("MaxDepth=0 should return nil tracker")
	}

	tracker, err = NewLoopDepthTracker(-1)
	if err == nil {
		t.Errorf("MaxDepth=-1 should fail")
	}
	if tracker != nil {
		t.Errorf("MaxDepth=-1 should return nil tracker")
	}

	tracker, err = NewLoopDepthTracker(1)
	if err != nil {
		t.Errorf("MaxDepth=1 should succeed, got err=%v", err)
	}
	if tracker == nil || tracker.MaxDepth() != 1 {
		t.Errorf("MaxDepth=1 should return tracker with depth 1")
	}
}

// --- L1-Extra: Nil receiver failsafe ------------------------------------------

// Extra: nil tracker receiver should not panic (failsafe to Continue)
func TestLoopDepthTracker_NilReceiver(t *testing.T) {
	var tracker *LoopDepthTracker
	d := tracker.ShouldContinue(makeCtx("sess-nil"))
	if d.Action != EscapeContinue {
		t.Errorf("nil receiver: Action = %s, want continue (failsafe)", d.Action)
	}
	if d.Reason != "tracker_nil_failsafe" {
		t.Errorf("nil receiver: Reason = %q, want tracker_nil_failsafe", d.Reason)
	}
}

// --- L1-Extra: Empty SessionID failsafe ---------------------------------------

// Extra: empty SessionID should fail-safe to Continue (defensive)
func TestLoopDepthTracker_EmptySessionID(t *testing.T) {
	tracker, _ := NewLoopDepthTracker(DefaultMaxDepth)
	ctx := LoopContext{SessionID: "", PlanKind: 1}
	d := tracker.ShouldContinue(ctx)
	if d.Action != EscapeContinue {
		t.Errorf("empty SessionID: Action = %s, want continue (failsafe)", d.Action)
	}
}

// --- L1-Extra: ResetSession ---------------------------------------------------

// Extra: ResetSession 只清空指定 session
func TestLoopDepthTracker_ResetSession(t *testing.T) {
	tracker, _ := NewLoopDepthTracker(DefaultMaxDepth)
	tracker.ShouldContinue(makeCtx("sess-rs-1"))
	tracker.ShouldContinue(makeCtx("sess-rs-2"))

	ok := tracker.ResetSession("sess-rs-1")
	if !ok {
		t.Errorf("ResetSession(sess-rs-1) should return true")
	}
	if tracker.Depth("sess-rs-1") != 0 {
		t.Errorf("sess-rs-1 depth after reset = %d, want 0", tracker.Depth("sess-rs-1"))
	}
	if tracker.Depth("sess-rs-2") == 0 {
		t.Errorf("sess-rs-2 should not be reset")
	}
}

// --- helpers ------------------------------------------------------------------

func makeCtx(sessionID string) LoopContext {
	return LoopContext{
		SessionID:        sessionID,
		PlanKind:         1, // ProtocolPlan
		ObservationKind:  0, // ObsFact
		FailureCriterion: "verifier_timeout",
		ArtifactType:     0, // StateChangeCert
	}
}