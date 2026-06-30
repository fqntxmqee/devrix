package sessionorchestrator

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestTurnState_BeginEndWait_HappyPath covers the canonical lifecycle:
// BeginTurn → WaitTurn blocks → EndTurn releases → WaitTurn returns nil.
func TestTurnState_BeginEndWait_HappyPath(t *testing.T) {
	ts := NewTurnState()
	if err := ts.BeginTurn("sess_happy"); err != nil {
		t.Fatalf("BeginTurn: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ts.WaitTurn(ctx, "sess_happy")
	}()

	// Brief delay so WaitTurn is definitely parked before EndTurn.
	time.Sleep(5 * time.Millisecond)
	ts.EndTurn("sess_happy")

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("WaitTurn returned err = %v, want nil", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("WaitTurn did not return within 100ms after EndTurn")
	}
}

// TestTurnState_BeginTurn_DoubleBlocks_ReturnsInProgress verifies that a
// second BeginTurn for the same session while the first is open returns
// TurnInProgressError carrying the original turn's start time + turnNo.
func TestTurnState_BeginTurn_DoubleBlocks_ReturnsInProgress(t *testing.T) {
	ts := NewTurnState()
	if err := ts.BeginTurn("sess_dup"); err != nil {
		t.Fatalf("BeginTurn #1: %v", err)
	}
	started := time.Now()
	if got := ts.TurnNo("sess_dup"); got != 1 {
		t.Fatalf("TurnNo after BeginTurn #1 = %d, want 1", got)
	}

	err := ts.BeginTurn("sess_dup")
	if err == nil {
		t.Fatal("BeginTurn #2: expected TurnInProgressError, got nil")
	}
	var tip TurnInProgressError
	if !errors.As(err, &tip) {
		t.Fatalf("BeginTurn #2: err = %T (%v), want TurnInProgressError", err, err)
	}
	if tip.SessionID != "sess_dup" {
		t.Errorf("tip.SessionID = %q, want sess_dup", tip.SessionID)
	}
	if tip.TurnNo != 1 {
		t.Errorf("tip.TurnNo = %d, want 1", tip.TurnNo)
	}
	if tip.SinceStartedAt.Before(started.Add(-time.Second)) || tip.SinceStartedAt.After(started.Add(time.Second)) {
		t.Errorf("tip.SinceStartedAt = %v not within [%v, %v]", tip.SinceStartedAt, started.Add(-time.Second), started.Add(time.Second))
	}

	// errors.Is(target) on value sentinel must work too.
	if !errors.Is(err, TurnInProgressError{}) {
		t.Error("errors.Is(err, TurnInProgressError{}) = false, want true")
	}

	ts.EndTurn("sess_dup")
}

// TestTurnState_BeginTurn_AfterEnd_AdvancesTurnNo verifies that after
// EndTurn, BeginTurn succeeds and TurnNo returns to 1 (handle is purged
// post-End per RH-D7-07 DM-20260630-013 to bound map growth).
func TestTurnState_BeginTurn_AfterEnd_AdvancesTurnNo(t *testing.T) {
	ts := NewTurnState()
	if err := ts.BeginTurn("sess_adv"); err != nil {
		t.Fatalf("BeginTurn #1: %v", err)
	}
	ts.EndTurn("sess_adv")

	if err := ts.BeginTurn("sess_adv"); err != nil {
		t.Fatalf("BeginTurn #2 (after End): %v", err)
	}
	// RH-D7-07: EndTurn purges the handle, so the next BeginTurn starts a
	// fresh counter at 1 (no prev handle to advance from). This is the
	// trade-off for bounding long-lived map growth — turn identity is
	// tracked at the session layer (TranscriptReader / log scope) rather
	// than by an in-process counter that survives End.
	if got := ts.TurnNo("sess_adv"); got != 1 {
		t.Errorf("TurnNo after BeginTurn #2 = %d, want 1 (handle purged on End)", got)
	}
	ts.EndTurn("sess_adv")
}

// TestTurnState_EndTurn_PurgesHandle_MapBounded is the regression test
// for the unbounded-growth bug (RH-D7-07 DM-20260630-013). With the
// pre-fix code, 1000 BeginTurn/EndTurn cycles for distinct sessions
// would leave 1000 stale entries in handles. Post-fix, the map stays
// empty after each End.
func TestTurnState_EndTurn_PurgesHandle_MapBounded(t *testing.T) {
	ts := NewTurnState()
	for i := 0; i < 1000; i++ {
		sid := "sess_purge_" + strings.Repeat("x", 0) + string(rune('a'+i%26)) + string(rune('A'+(i/26)%26))
		if err := ts.BeginTurn(sid); err != nil {
			t.Fatalf("BeginTurn #%d: %v", i, err)
		}
		ts.EndTurn(sid)
	}
	ts.mu.RLock()
	n := len(ts.handles)
	ts.mu.RUnlock()
	if n != 0 {
		t.Errorf("handles map size after 1000 Begin/End cycles = %d, want 0 (unbounded growth regression)", n)
	}
}

// TestTurnState_BeginTurn_AfterEnd_StaleSlotReplaced verifies the cleanup
// path: after EndTurn, the stale handle is silently replaced (no error).
// This covers the crash-recovery path where EndTurn was never called.
func TestTurnState_BeginTurn_AfterEnd_StaleSlotReplaced(t *testing.T) {
	ts := NewTurnState()
	if err := ts.BeginTurn("sess_stale"); err != nil {
		t.Fatalf("BeginTurn #1: %v", err)
	}
	// simulate "stale slot" by manually closing the handle's done channel
	// WITHOUT calling EndTurn (so closeOnce is not consumed). This models
	// a crash where close(done) ran via some panic path but the orchestrator
	// never observed it. BeginTurn must replace it.
	ts.mu.Lock()
	close(ts.handles["sess_stale"].done)
	ts.mu.Unlock()

	if err := ts.BeginTurn("sess_stale"); err != nil {
		t.Fatalf("BeginTurn on stale slot: %v, want nil (silent replace)", err)
	}
	if got := ts.TurnNo("sess_stale"); got != 2 {
		t.Errorf("TurnNo after stale-slot BeginTurn = %d, want 2", got)
	}
}

// TestTurnState_EndTurn_Idempotent verifies that calling EndTurn twice
// does not panic (close-on-closed-channel panic).
func TestTurnState_EndTurn_Idempotent(t *testing.T) {
	ts := NewTurnState()
	if err := ts.BeginTurn("sess_idem"); err != nil {
		t.Fatalf("BeginTurn: %v", err)
	}
	ts.EndTurn("sess_idem")
	ts.EndTurn("sess_idem") // must not panic
	ts.EndTurn("sess_idem") // must not panic
}

// TestTurnState_EndTurn_NoHandle_IsNoop verifies that EndTurn on an
// unknown sessionID is silently ignored (e.g., when ProcessMessage
// never reached BeginTurn because of an earlier error).
func TestTurnState_EndTurn_NoHandle_IsNoop(t *testing.T) {
	ts := NewTurnState()
	ts.EndTurn("sess_never_began") // must not panic
}

// TestTurnState_WaitTurn_NoHandle_ReturnsNil covers the case where
// WaitTurn is called for a session that never had BeginTurn (the
// happy case for the very first turn of a session).
func TestTurnState_WaitTurn_NoHandle_ReturnsNil(t *testing.T) {
	ts := NewTurnState()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := ts.WaitTurn(ctx, "sess_never"); err != nil {
		t.Errorf("WaitTurn on no-handle session = %v, want nil", err)
	}
}

// TestTurnState_WaitTurn_CtxCancel verifies WaitTurn returns ctx.Err()
// when context is canceled before EndTurn fires.
func TestTurnState_WaitTurn_CtxCancel(t *testing.T) {
	ts := NewTurnState()
	if err := ts.BeginTurn("sess_cancel"); err != nil {
		t.Fatalf("BeginTurn: %v", err)
	}
	defer ts.EndTurn("sess_cancel")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- ts.WaitTurn(ctx, "sess_cancel") }()

	time.Sleep(5 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("WaitTurn err = %v, want context.Canceled", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("WaitTurn did not return within 100ms after ctx cancel")
	}
}

// TestTurnState_NilReceiver_AllAPIsNoop verifies that a nil *TurnState
// is safe to call (covers legacy/test paths where WithPriorContextRounds
// is not wired).
func TestTurnState_NilReceiver_AllAPIsNoop(t *testing.T) {
	var ts *TurnState
	if err := ts.BeginTurn("sess_nil"); err != nil {
		t.Errorf("nil.BeginTurn = %v, want nil", err)
	}
	ts.EndTurn("sess_nil")
	if err := ts.WaitTurn(context.Background(), "sess_nil"); err != nil {
		t.Errorf("nil.WaitTurn = %v, want nil", err)
	}
	if ts.IsTurnInProgress("sess_nil") {
		t.Error("nil.IsTurnInProgress = true, want false")
	}
	if got := ts.TurnNo("sess_nil"); got != 0 {
		t.Errorf("nil.TurnNo = %d, want 0", got)
	}
}

// TestTurnState_BeginTurn_EmptySessionID_ReturnsError covers the input
// validation boundary.
func TestTurnState_BeginTurn_EmptySessionID_ReturnsError(t *testing.T) {
	ts := NewTurnState()
	if err := ts.BeginTurn(""); err == nil {
		t.Error("BeginTurn(\"\") = nil, want error")
	}
}

// TestTurnState_TurnNo_NilAndEmpty covers the nil/empty inputs to
// TurnNo (defensive no-ops).
func TestTurnState_TurnNo_NilAndEmpty(t *testing.T) {
	var ts *TurnState
	if got := ts.TurnNo("sess_nil"); got != 0 {
		t.Errorf("nil.TurnNo = %d, want 0", got)
	}
	ts = NewTurnState()
	if got := ts.TurnNo(""); got != 0 {
		t.Errorf("TurnNo(\"\") = %d, want 0", got)
	}
}

// TestTurnState_IsTurnInProgress_NilAndEmpty covers the nil/empty inputs.
func TestTurnState_IsTurnInProgress_NilAndEmpty(t *testing.T) {
	var ts *TurnState
	if ts.IsTurnInProgress("sess_nil") {
		t.Error("nil.IsTurnInProgress = true, want false")
	}
	ts = NewTurnState()
	if ts.IsTurnInProgress("") {
		t.Error("IsTurnInProgress(\"\") = true, want false")
	}
	if ts.IsTurnInProgress("sess_never_began") {
		t.Error("IsTurnInProgress on never-began session = true, want false")
	}
}

// TestTurnState_IsTurnInProgress_AfterEndTurn_Flips covers the lifecycle
// transition: in-progress during the turn, not in-progress after End.
func TestTurnState_IsTurnInProgress_AfterEndTurn_Flips(t *testing.T) {
	ts := NewTurnState()
	if err := ts.BeginTurn("sess_flip"); err != nil {
		t.Fatalf("BeginTurn: %v", err)
	}
	if !ts.IsTurnInProgress("sess_flip") {
		t.Error("IsTurnInProgress after Begin = false, want true")
	}
	ts.EndTurn("sess_flip")
	if ts.IsTurnInProgress("sess_flip") {
		t.Error("IsTurnInProgress after End = true, want false")
	}
}

// TestTurnState_EndTurn_EmptySessionID_Noop covers the defensive boundary.
func TestTurnState_EndTurn_EmptySessionID_Noop(t *testing.T) {
	ts := NewTurnState()
	ts.EndTurn("") // must not panic
}

// TestTurnState_ConcurrentStress is the 1000-goroutine regression test
// that catches missing sync.Mutex usage in BeginTurn. The contract is
// that at most ONE goroutine's BeginTurn returns nil at any instant —
// the rest must see TurnInProgressError. After all goroutines return,
// the final handle must be closed (no leak).
func TestTurnState_ConcurrentStress(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test skipped in -short mode")
	}
	ts := NewTurnState()
	const n = 1000
	var wg sync.WaitGroup
	var succeeded int64
	var rejected int64
	var overlaps int64

	// Pre-allocate ordering channels so we can detect overlap: goroutine i
	// records "in" by atomic-add to inFlight, records "out" by decrement;
	// if inFlight > 1 we have a BeginTurn/EndTurn overlap (mutex broken).
	var inFlight int64
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if err := ts.BeginTurn("sess_stress"); err == nil {
				atomic.AddInt64(&succeeded, 1)
				cur := atomic.AddInt64(&inFlight, 1)
				if cur > 1 {
					atomic.AddInt64(&overlaps, 1)
				}
				// Tiny work to widen the window for the overlap detector.
				time.Sleep(time.Microsecond)
				atomic.AddInt64(&inFlight, -1)
				ts.EndTurn("sess_stress")
			} else {
				atomic.AddInt64(&rejected, 1)
			}
		}()
	}
	wg.Wait()

	// At least one BeginTurn succeeded (proves the API works).
	if got := atomic.LoadInt64(&succeeded); got < 1 {
		t.Errorf("successful BeginTurn count = %d, want ≥ 1", got)
	}
	// Total succeeded + rejected == n (every goroutine accounted for).
	if got := atomic.LoadInt64(&succeeded) + atomic.LoadInt64(&rejected); got != n {
		t.Errorf("succeeded(%d) + rejected(%d) = %d, want %d", atomic.LoadInt64(&succeeded), atomic.LoadInt64(&rejected), got, n)
	}
	// Mutex guarantee: no two BeginTurns ever had an open handle simultaneously
	// (i.e., during the sleep window).
	if got := atomic.LoadInt64(&overlaps); got != 0 {
		t.Errorf("overlaps = %d, want 0 (BeginTurn/EndTurn must be serialized)", got)
	}
	// After dust settles: no in-flight state.
	if ts.IsTurnInProgress("sess_stress") {
		t.Error("IsTurnInProgress after stress = true, want false (last EndTurn closed)")
	}
	if got := atomic.LoadInt64(&inFlight); got != 0 {
		t.Errorf("inFlight counter = %d after wg.Wait(), want 0", got)
	}
}

// TestTurnInProgressError_Is_PatternMatch verifies that the cross-package
// errors.Is(err, TurnInProgressError{}) idiom works as advertised. This
// is what D1 feishu adapter will rely on.
func TestTurnInProgressError_Is_PatternMatch(t *testing.T) {
	var (
		err = TurnInProgressError{
			SessionID:      "sess_x",
			SinceStartedAt: time.Now(),
			TurnNo:         7,
		}
		wrapped error = err
	)

	if !errors.Is(wrapped, TurnInProgressError{}) {
		t.Error("errors.Is(wrapped, TurnInProgressError{}) = false, want true")
	}
	if errors.Is(wrapped, errors.New("other")) {
		t.Error("errors.Is(wrapped, unrelated err) = true, want false")
	}
}

// TestTurnInProgressError_ErrorMessage verifies the error string format
// (used for slog/observability audit).
func TestTurnInProgressError_ErrorMessage(t *testing.T) {
	start := time.Date(2026, 6, 28, 17, 31, 4, 0, time.UTC)
	err := TurnInProgressError{SessionID: "sess_1782638991113_5000", SinceStartedAt: start, TurnNo: 1}
	got := err.Error()
	wantSubstr := []string{"sess_1782638991113_5000", "1", "2026-06-28"}
	for _, s := range wantSubstr {
		if !strings.Contains(got, s) {
			t.Errorf("Error() = %q, missing substring %q", got, s)
		}
	}
}