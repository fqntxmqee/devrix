// Package learn: UserActionListener tests (DM-20260707-001 PR-E, T62).
//
// Coverage matrix:
//
//   1. TestParseUserAction_Canonical       — "cancel"/"accept"/"modify" → enum
//   2. TestParseUserAction_CaseInsensitive  — "CANCEL"/"Accept"/"modify "
//   3. TestParseUserAction_UnknownReturnsError
//   4. TestHandleUserAction_HappyPath_Cancel
//   5. TestHandleUserAction_HappyPath_Accept
//   6. TestHandleUserAction_HappyPath_Modify
//   7. TestHandleUserAction_MissingSessionID_Rejected
//   8. TestHandleUserAction_MissingRoundNo_Rejected
//   9. TestHandleUserAction_UnknownAction_Rejected
//  10. TestHandleUserAction_FallsBackToPlainLearner
//  11. TestHandleUserAction_NoLearner_ReturnsError
//  12. TestHandleUserAction_VerdictResolverUsed
package learn

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// stubResolver returns a fixed Verdict (or error) for any (sessionID, roundNo).
type stubResolver struct {
	mu      sync.Mutex
	calls   int
	verdict workmodel.Verdict
	err     error
}

func (s *stubResolver) ResolveVerdict(ctx context.Context, sessionID string, roundNo int) (workmodel.Verdict, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return s.verdict, s.err
}

// TestParseUserAction_Canonical.
func TestParseUserAction_Canonical(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want UserAction
	}{
		{"cancel", UserActionCancel},
		{"accept", UserActionAccept},
		{"modify", UserActionModify},
	}
	for _, tc := range cases {
		got, err := parseUserAction(tc.in)
		if err != nil {
			t.Errorf("parseUserAction(%q) returned error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("parseUserAction(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

// TestParseUserAction_CaseInsensitive.
func TestParseUserAction_CaseInsensitive(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want UserAction
	}{
		{"CANCEL", UserActionCancel},
		{"Accept", UserActionAccept},
		{"  modify  ", UserActionModify},
		{"MoDiFy", UserActionModify},
	}
	for _, tc := range cases {
		got, err := parseUserAction(tc.in)
		if err != nil {
			t.Errorf("parseUserAction(%q) returned error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("parseUserAction(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

// TestParseUserAction_UnknownReturnsError.
func TestParseUserAction_UnknownReturnsError(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"", "pause", "reject", "CANCELED"} {
		_, err := parseUserAction(in)
		if !errors.Is(err, ErrUnknownUserAction) {
			t.Errorf("parseUserAction(%q) err = %v, want ErrUnknownUserAction", in, err)
		}
	}
}

// makeListener constructs a listener backed by an AsyncLearner with the mock
// inner Learner, plus the supplied resolver. Returns the listener + the mock
// for assertion.
func makeListener(t *testing.T, resolver VerdictResolver) (*DefaultUserActionListener, *mockLearner, *AsyncLearner) {
	t.Helper()
	mock := &mockLearner{}
	async := NewAsyncLearner(mock, AsyncLearnerOptions{
		QueueSize:      10,
		WorkerCount:    1,
		EnqueueTimeout: 100 * 1_000_000, // 100ms
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	t.Cleanup(func() { async.Shutdown(context.Background()) })

	listener := NewDefaultUserActionListener(async, nil, resolver, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return listener, mock, async
}

// TestHandleUserAction_HappyPath_Cancel.
func TestHandleUserAction_HappyPath_Cancel(t *testing.T) {
	t.Parallel()
	resolver := &stubResolver{verdict: workmodel.Verdict{Kind: types.VerdictPass, Confidence: 0.8}}
	listener, mock, _ := makeListener(t, resolver)

	evt := UserActionEvent{SessionID: "sess_cancel", RoundNo: 1, Action: "cancel", UserID: "user_1"}
	req, err := listener.HandleUserAction(context.Background(), evt)
	if err != nil {
		t.Fatalf("HandleUserAction returned error: %v", err)
	}
	if req.SessionID != "sess_cancel" {
		t.Errorf("req.SessionID = %s, want sess_cancel", req.SessionID)
	}

	// Drain + assert.
	ctx, cancel := context.WithTimeout(context.Background(), 500*1_000_000)
	defer cancel()
	if err := mock_sync_drain(mock, ctx); err != nil {
		t.Fatalf("drain failed: %v", err)
	}
	if mock.CallCount() != 1 {
		t.Errorf("mock called %d times, want 1", mock.CallCount())
	}

	metrics := listener.Metrics()
	if metrics.Cancel.Load() != 1 {
		t.Errorf("Cancel counter = %d, want 1", metrics.Cancel.Load())
	}
}

// TestHandleUserAction_HappyPath_Accept.
func TestHandleUserAction_HappyPath_Accept(t *testing.T) {
	t.Parallel()
	resolver := &stubResolver{verdict: workmodel.Verdict{Kind: types.VerdictPass, Confidence: 0.85}}
	listener, mock, _ := makeListener(t, resolver)

	evt := UserActionEvent{SessionID: "sess_accept", RoundNo: 2, Action: "accept", UserID: "user_2"}
	if _, err := listener.HandleUserAction(context.Background(), evt); err != nil {
		t.Fatalf("HandleUserAction returned error: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*1_000_000)
	defer cancel()
	if err := mock_sync_drain(mock, ctx); err != nil {
		t.Fatalf("drain failed: %v", err)
	}
	if listener.Metrics().Accept.Load() != 1 {
		t.Errorf("Accept counter = %d, want 1", listener.Metrics().Accept.Load())
	}
}

// TestHandleUserAction_HappyPath_Modify.
func TestHandleUserAction_HappyPath_Modify(t *testing.T) {
	t.Parallel()
	resolver := &stubResolver{verdict: workmodel.Verdict{Kind: types.VerdictPartial, Confidence: 0.5}}
	listener, mock, _ := makeListener(t, resolver)

	evt := UserActionEvent{SessionID: "sess_modify", RoundNo: 3, Action: "modify", UserID: "user_3"}
	if _, err := listener.HandleUserAction(context.Background(), evt); err != nil {
		t.Fatalf("HandleUserAction returned error: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*1_000_000)
	defer cancel()
	if err := mock_sync_drain(mock, ctx); err != nil {
		t.Fatalf("drain failed: %v", err)
	}
	if listener.Metrics().Modify.Load() != 1 {
		t.Errorf("Modify counter = %d, want 1", listener.Metrics().Modify.Load())
	}
}

// TestHandleUserAction_MissingSessionID_Rejected.
func TestHandleUserAction_MissingSessionID_Rejected(t *testing.T) {
	t.Parallel()
	resolver := &stubResolver{verdict: workmodel.Verdict{Kind: types.VerdictPass}}
	listener, _, _ := makeListener(t, resolver)

	evt := UserActionEvent{SessionID: "", RoundNo: 1, Action: "cancel"}
	_, err := listener.HandleUserAction(context.Background(), evt)
	if err == nil {
		t.Errorf("expected error for empty sessionID, got nil")
	}
	if listener.Metrics().Rejected.Load() != 1 {
		t.Errorf("Rejected counter = %d, want 1", listener.Metrics().Rejected.Load())
	}
}

// TestHandleUserAction_MissingRoundNo_Rejected.
func TestHandleUserAction_MissingRoundNo_Rejected(t *testing.T) {
	t.Parallel()
	resolver := &stubResolver{verdict: workmodel.Verdict{Kind: types.VerdictPass}}
	listener, _, _ := makeListener(t, resolver)

	evt := UserActionEvent{SessionID: "sess", RoundNo: 0, Action: "accept"}
	_, err := listener.HandleUserAction(context.Background(), evt)
	if err == nil {
		t.Errorf("expected error for roundNo=0, got nil")
	}
}

// TestHandleUserAction_UnknownAction_Rejected.
func TestHandleUserAction_UnknownAction_Rejected(t *testing.T) {
	t.Parallel()
	resolver := &stubResolver{verdict: workmodel.Verdict{Kind: types.VerdictPass}}
	listener, _, _ := makeListener(t, resolver)

	evt := UserActionEvent{SessionID: "sess", RoundNo: 1, Action: "pause"}
	_, err := listener.HandleUserAction(context.Background(), evt)
	if !errors.Is(err, ErrUnknownUserAction) {
		t.Errorf("expected ErrUnknownUserAction, got %v", err)
	}
	if listener.Metrics().Rejected.Load() != 1 {
		t.Errorf("Rejected counter = %d, want 1", listener.Metrics().Rejected.Load())
	}
}

// TestHandleUserAction_FallsBackToPlainLearner: when asyncLearner is nil but
// plainLearner is set, the listener uses the sync path.
func TestHandleUserAction_FallsBackToPlainLearner(t *testing.T) {
	t.Parallel()
	mock := &mockLearner{}
	resolver := &stubResolver{verdict: workmodel.Verdict{Kind: types.VerdictPass}}
	listener := NewDefaultUserActionListener(nil, mock, resolver, slog.New(slog.NewTextHandler(io.Discard, nil)))

	evt := UserActionEvent{SessionID: "sess_sync", RoundNo: 1, Action: "accept"}
	if _, err := listener.HandleUserAction(context.Background(), evt); err != nil {
		t.Fatalf("HandleUserAction returned error: %v", err)
	}
	if mock.CallCount() != 1 {
		t.Errorf("plain mock called %d times, want 1", mock.CallCount())
	}
}

// TestHandleUserAction_NoLearner_ReturnsError.
func TestHandleUserAction_NoLearner_ReturnsError(t *testing.T) {
	t.Parallel()
	listener := NewDefaultUserActionListener(nil, nil, &stubResolver{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	evt := UserActionEvent{SessionID: "sess", RoundNo: 1, Action: "accept"}
	_, err := listener.HandleUserAction(context.Background(), evt)
	if err == nil {
		t.Errorf("expected error when no learner wired, got nil")
	}
}

// TestHandleUserAction_VerdictResolverUsed: resolver is called with correct args.
func TestHandleUserAction_VerdictResolverUsed(t *testing.T) {
	t.Parallel()
	resolver := &stubResolver{verdict: workmodel.Verdict{Kind: types.VerdictPass, SourceID: "v_stub"}}
	listener, mock, _ := makeListener(t, resolver)

	evt := UserActionEvent{SessionID: "sess_v", RoundNo: 7, Action: "accept"}
	if _, err := listener.HandleUserAction(context.Background(), evt); err != nil {
		t.Fatalf("HandleUserAction returned error: %v", err)
	}
	if resolver.calls != 1 {
		t.Errorf("resolver calls = %d, want 1", resolver.calls)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*1_000_000)
	defer cancel()
	if err := mock_sync_drain(mock, ctx); err != nil {
		t.Fatalf("drain failed: %v", err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("mock.calls len = %d, want 1", len(mock.calls))
	}
	if mock.calls[0].Verdict.SourceID != "v_stub" {
		t.Errorf("mock.calls[0].Verdict.SourceID = %s, want v_stub", mock.calls[0].Verdict.SourceID)
	}
}

// mock_sync_drain waits for the mock's call count to reach at least 1 OR
// the context to time out. Helper used in listener tests because the mock's
// async submission may not finish by the time we assert. Polls every 5ms.
func mock_sync_drain(m *mockLearner, ctx context.Context) error {
	for {
		if m.CallCount() > 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Millisecond):
			// Yield to scheduler; loop will re-check.
		}
	}
}