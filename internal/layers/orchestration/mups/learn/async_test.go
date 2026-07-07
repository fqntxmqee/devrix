// Package learn: AsyncLearner tests (DM-20260707-001 PR-E, T58).
//
// 6-dimension coverage matrix for AsyncLearner (T65 stress):
//
//   1. Enqueue happy path          — single Enqueue succeeds, inner called
//   2. Enqueue latency < 1ms       — production timing target (T58)
//   3. Queue overflow → ErrAsyncQueueFull + dropped metric increments
//   4. Drain blocks until empty    — queued items all processed
//   5. Concurrent Enqueue (stress) — 100 producers + queueSize=100, no panics, all-or-nothing
//   6. Shutdown graceful drain     — after Shutdown, workers exit + queue empty
package learn

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// mockLearner is a minimal Learner implementation for async tests. It records
// every Learn call and counts successes / failures. Inject a delay via opts
// to simulate slow Learn paths.
type mockLearner struct {
	mu        sync.Mutex
	calls     []LearnRequest
	delay     time.Duration
	failEvery int // fail every Nth call (0 = never fail)
	callCount atomic.Int64
}

func (m *mockLearner) Learn(ctx context.Context, req LearnRequest) ([]*LearningAsset, error) {
	m.mu.Lock()
	m.calls = append(m.calls, req)
	delay := m.delay
	failEvery := m.failEvery
	m.mu.Unlock()

	m.callCount.Add(1)
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if failEvery > 0 {
		n := m.callCount.Load()
		if int(n)%failEvery == 0 {
			return nil, errors.New("mock: forced failure")
		}
	}
	return []*LearningAsset{}, nil
}

func (m *mockLearner) Inject(ctx context.Context, sessionID, trackModeHint string) (*AdaptivePrior, error) {
	return nil, nil
}

func (m *mockLearner) ScheduledTick(ctx context.Context) error {
	return nil
}

func (m *mockLearner) CallCount() int { return int(m.callCount.Load()) }

// TestAsyncLearner_EnqueueHappyPath: single Enqueue succeeds; inner called once.
func TestAsyncLearner_EnqueueHappyPath(t *testing.T) {
	t.Parallel()
	mock := &mockLearner{}
	al := NewAsyncLearner(mock, AsyncLearnerOptions{
		QueueSize:    10,
		WorkerCount:  1,
		EnqueueTimeout: 100 * time.Millisecond,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	defer al.Shutdown(context.Background())

	req := LearnRequest{
		SessionID: "test-session",
		Verdict:   workmodel.Verdict{Kind: 1, Confidence: 0.8},
	}
	if err := al.Enqueue(context.Background(), req); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	// Wait for the worker to process it.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := al.Drain(ctx); err != nil {
		t.Fatalf("Drain failed: %v", err)
	}
	if mock.CallCount() != 1 {
		t.Errorf("expected 1 inner Learn call, got %d", mock.CallCount())
	}
	metrics := al.Metrics()
	if metrics.Processed != 1 {
		t.Errorf("metrics.Processed = %d, want 1", metrics.Processed)
	}
	if metrics.Dropped != 0 {
		t.Errorf("metrics.Dropped = %d, want 0", metrics.Dropped)
	}
}

// TestAsyncLearner_EnqueueLatency asserts the production timing target:
// Enqueue returns within 1ms even under nominal load.
func TestAsyncLearner_EnqueueLatency(t *testing.T) {
	t.Parallel()
	mock := &mockLearner{}
	al := NewAsyncLearner(mock, AsyncLearnerOptions{
		QueueSize:    100,
		WorkerCount:  2,
		EnqueueTimeout: 1 * time.Millisecond,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	defer al.Shutdown(context.Background())

	// Warm up.
	al.Enqueue(context.Background(), LearnRequest{SessionID: "warmup", Verdict: workmodel.Verdict{Kind: 1}})
	time.Sleep(5 * time.Millisecond)

	// Measure 10 sequential Enqueue calls.
	start := time.Now()
	for i := 0; i < 10; i++ {
		req := LearnRequest{SessionID: "sess", Verdict: workmodel.Verdict{Kind: 1, Confidence: 0.5}}
		if err := al.Enqueue(context.Background(), req); err != nil {
			t.Fatalf("Enqueue[%d] failed: %v", i, err)
		}
	}
	elapsed := time.Since(start)
	avgPerOp := elapsed / 10
	if avgPerOp > 1*time.Millisecond {
		t.Errorf("avg Enqueue latency %v exceeds 1ms target", avgPerOp)
	}
}

// TestAsyncLearner_QueueOverflow verifies that a full queue returns
// ErrAsyncQueueFull and increments the Dropped metric.
//
// Layout reasoning: queueSize=2 means 2 items fit in the queue; one more
// is being processed by the worker. So the first 3 Enqueues succeed, the
// 4th overflows and must be dropped with ErrAsyncQueueFull.
func TestAsyncLearner_QueueOverflow(t *testing.T) {
	t.Parallel()
	mock := &mockLearner{delay: 50 * time.Millisecond} // slow worker → queue fills up
	al := NewAsyncLearner(mock, AsyncLearnerOptions{
		QueueSize:    2, // tiny queue to force overflow fast
		WorkerCount:  1,
		EnqueueTimeout: 5 * time.Millisecond, // tight timeout to fail fast
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	defer al.Shutdown(context.Background())

	// First 3 Enqueues fill the worker + queue; 4th must drop.
	for i := 0; i < 3; i++ {
		if err := al.Enqueue(context.Background(), LearnRequest{SessionID: "fill", Verdict: workmodel.Verdict{Kind: 1}}); err != nil {
			t.Fatalf("fill Enqueue[%d] unexpectedly dropped: %v", i, err)
		}
	}
	err := al.Enqueue(context.Background(), LearnRequest{SessionID: "overflow", Verdict: workmodel.Verdict{Kind: 1}})
	if !errors.Is(err, ErrAsyncQueueFull) {
		t.Errorf("expected ErrAsyncQueueFull, got %v", err)
	}
	metrics := al.Metrics()
	if metrics.Dropped != 1 {
		t.Errorf("metrics.Dropped = %d, want 1", metrics.Dropped)
	}
}

// TestAsyncLearner_DrainBlocksUntilEmpty: enqueue 10, drain returns only after
// all 10 are processed.
func TestAsyncLearner_DrainBlocksUntilEmpty(t *testing.T) {
	t.Parallel()
	mock := &mockLearner{delay: 10 * time.Millisecond}
	al := NewAsyncLearner(mock, AsyncLearnerOptions{
		QueueSize:    20,
		WorkerCount:  2,
		EnqueueTimeout: 100 * time.Millisecond,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	defer al.Shutdown(context.Background())

	for i := 0; i < 10; i++ {
		al.Enqueue(context.Background(), LearnRequest{SessionID: "drain", Verdict: workmodel.Verdict{Kind: 1}})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := al.Drain(ctx); err != nil {
		t.Fatalf("Drain failed: %v", err)
	}
	if mock.CallCount() != 10 {
		t.Errorf("expected 10 inner calls, got %d", mock.CallCount())
	}
}

// TestAsyncLearner_ConcurrentStress: 100 producers enqueue concurrently,
// no panics, all-or-nothing semantics (enqueued + dropped == total attempts).
func TestAsyncLearner_ConcurrentStress(t *testing.T) {
	t.Parallel()
	mock := &mockLearner{delay: 1 * time.Millisecond}
	al := NewAsyncLearner(mock, AsyncLearnerOptions{
		QueueSize:    100,
		WorkerCount:  2,
		EnqueueTimeout: 1 * time.Millisecond,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	defer al.Shutdown(context.Background())

	const producerCount = 20
	const enqueuesPerProducer = 50
	var wg sync.WaitGroup
	var successCount, dropCount atomic.Int64
	for p := 0; p < producerCount; p++ {
		wg.Add(1)
		go func(pid int) {
			defer wg.Done()
			for i := 0; i < enqueuesPerProducer; i++ {
				err := al.Enqueue(context.Background(), LearnRequest{SessionID: "stress", Verdict: workmodel.Verdict{Kind: 1}})
				if err == nil {
					successCount.Add(1)
				} else if errors.Is(err, ErrAsyncQueueFull) {
					dropCount.Add(1)
				} else {
					t.Errorf("unexpected Enqueue error: %v", err)
				}
			}
		}(p)
	}
	wg.Wait()

	total := producerCount * enqueuesPerProducer
	if successCount.Load()+dropCount.Load() != int64(total) {
		t.Errorf("total = %d, want %d", successCount.Load()+dropCount.Load(), total)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	al.Drain(ctx)

	metrics := al.Metrics()
	if metrics.Enqueued != successCount.Load() {
		t.Errorf("metrics.Enqueued = %d, want %d", metrics.Enqueued, successCount.Load())
	}
	if metrics.Dropped != dropCount.Load() {
		t.Errorf("metrics.Dropped = %d, want %d", metrics.Dropped, dropCount.Load())
	}
	// processed + failed = enqueued (no Learn work lost in counting).
	if metrics.Processed+metrics.Failed != metrics.Enqueued {
		t.Errorf("processed(%d) + failed(%d) != enqueued(%d)",
			metrics.Processed, metrics.Failed, metrics.Enqueued)
	}
	// Submitted/Completed accounting must balance at Drain time.
	if metrics.Submitted != metrics.Completed {
		t.Errorf("submitted(%d) != completed(%d) at Drain", metrics.Submitted, metrics.Completed)
	}
}

// TestAsyncLearner_ShutdownGraceful: after Shutdown, workers exit and the
// queue is drained.
func TestAsyncLearner_ShutdownGraceful(t *testing.T) {
	t.Parallel()
	mock := &mockLearner{delay: 1 * time.Millisecond}
	al := NewAsyncLearner(mock, AsyncLearnerOptions{
		QueueSize:    50,
		WorkerCount:  2,
		EnqueueTimeout: 100 * time.Millisecond,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	for i := 0; i < 20; i++ {
		al.Enqueue(context.Background(), LearnRequest{SessionID: "shutdown", Verdict: workmodel.Verdict{Kind: 1}})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := al.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	metrics := al.Metrics()
	if metrics.QueueDepth != 0 {
		t.Errorf("post-Shutdown queue depth = %d, want 0", metrics.QueueDepth)
	}

	// Enqueue after Shutdown returns ErrAsyncQueueFull.
	err := al.Enqueue(context.Background(), LearnRequest{SessionID: "post", Verdict: workmodel.Verdict{Kind: 1}})
	if !errors.Is(err, ErrAsyncQueueFull) {
		t.Errorf("post-Shutdown Enqueue = %v, want ErrAsyncQueueFull", err)
	}

	// Idempotent Shutdown.
	if err := al.Shutdown(context.Background()); err != nil {
		t.Errorf("second Shutdown failed: %v", err)
	}
}