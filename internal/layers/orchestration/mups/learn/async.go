// Package learn: AsyncLearner — non-blocking Learn wrapper (DM-20260707-001 PR-E, T58).
//
// AsyncLearner is the production wrapper around DefaultLearner that lets
// callers (ItemPipelineRunner.Verify-complete → Learn transition) fire-and-forget
// Learn without blocking the channel close on slow BayesianUpdate / Memory.Store
// paths.
//
// Why a separate type (vs. inlining the goroutine inside DefaultLearner):
//   - DefaultLearner is synchronous by contract (Learn returns []*LearningAsset).
//     Callers that want synchronous Learn need the simple contract.
//   - AsyncLearner wraps a Learner (interface) so any Learner implementation
//     can be made async — production passes DefaultLearner, tests can pass a
//     mock with deterministic behavior.
//
// Design (DM-20260707-001 PR-E, T58):
//
//	queueSize = 100            — buffered channel size (≈ 2x typical burst rate)
//	workerCount = 2            — 2 background workers (sufficient for SOP write throughput)
//	Enqueue() < 1ms            — non-blocking; drops to FeedbackMemory + audit on full queue
//	Drain() blocks             — waits for in-flight + queued to complete
//	Shutdown(ctx) graceful     — closes the queue, waits for workers with timeout
//
// Why 100/2: empirical — Verify emits ~5 Learn/second under nominal load; 100
// gives 20s of headroom on a worker stall (rare; backed by CircuitBreaker L1).
// 2 workers saturate a single ReputationStore row (MemStore has internal mutex;
// SQLite has connection-level serialization).
//
// Why non-blocking Enqueue: Learn cannot back-pressure the channel close
// path (ItemPipelineRunner auto-close would otherwise hang waiting for Learn).
// Drops are recoverable on the next round (BayesianUpdate is monotonic; a
// dropped Pass means the next Pass gets a slightly delayed α++, never a miss).
package learn

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Default async-learn configuration constants. These are the values that
// production passes to NewAsyncLearner; tests may override per-instance.
const (
	// AsyncQueueSize is the buffered channel depth. Sized at 100 to absorb
	// bursty Verify-complete → Learn transitions without back-pressuring the
	// channel close path.
	AsyncQueueSize = 100

	// AsyncWorkerCount is the number of background goroutines that drain
	// the Learn queue. 2 workers saturate a single ReputationStore row
	// (the dominant serialization point) without contention overhead.
	AsyncWorkerCount = 2

	// AsyncEnqueueTimeout is the maximum time Enqueue waits when the queue
	// is at capacity. Spec target: < 1ms in production. This constant is
	// the upper bound used by the test suite to detect regressions.
	AsyncEnqueueTimeout = 1 * time.Millisecond
)

// ErrAsyncQueueFull is returned by Enqueue when the queue is at capacity
// and the configured EnqueueTimeout elapses without a slot opening up.
// The caller is expected to log + audit + skip (the dropped Learn will be
// re-emitted on the next round; monotonic BayesianUpdate is the safety net).
var ErrAsyncQueueFull = errors.New("learn: async queue full")

// AsyncLearner wraps a synchronous Learner with a non-blocking queue +
// background workers. Implements the Learner interface so callers can swap
// it in transparently.
type AsyncLearner struct {
	// inner is the synchronous Learner (typically *DefaultLearner).
	inner Learner

	// queue carries the in-flight LearnRequests waiting for a worker.
	// Size = AsyncQueueSize (100).
	queue chan LearnRequest

	// workers signals worker goroutine shutdown. Close to begin draining.
	workers sync.WaitGroup

	// shutdownOnce guards the idempotent Shutdown() call.
	shutdownOnce sync.Once

	// shutdownCh is closed by Shutdown() to tell workers to drain and exit.
	shutdownCh chan struct{}

	// closed is set to true by Shutdown() before the queue is closed.
	// Enqueue checks this flag and returns ErrAsyncQueueFull once set.
	closed atomic.Bool

	// metrics counters (atomic; safe for concurrent access).
	enqueued  atomic.Int64 // total Enqueue() successes
	dropped   atomic.Int64 // total Enqueue() drops (queue full)
	processed atomic.Int64 // total successful DefaultLearner.Learn() calls
	failed    atomic.Int64 // total DefaultLearner.Learn() failures

	// submitted counts every successful Enqueue; completed counts every
	// finished Learn (success or fail). Drain waits until submitted ==
	// completed so we never return prematurely while a worker is still
	// holding an item. (The earlier inFlight atomic was racy with the
	// channel receive: a worker could have already received from the
	// queue but not yet incremented inFlight when Drain observed both
	// queue and inFlight at zero.)
	submitted atomic.Int64
	completed atomic.Int64

	// enqueueTimeout is the maximum time Enqueue waits when the queue is
	// at capacity. Stored from opts so tests can override.
	enqueueTimeout time.Duration

	// logger is the structured logger; defaults to slog.Default().
	logger *slog.Logger
}

// AsyncLearnerOptions configures a new AsyncLearner. All fields are optional;
// zero values fall back to defaults.
type AsyncLearnerOptions struct {
	// QueueSize overrides AsyncQueueSize when non-zero. Used by tests that
	// want a tighter queue (e.g. size=2 to test overflow behavior).
	QueueSize int

	// WorkerCount overrides AsyncWorkerCount when non-zero. Used by tests.
	WorkerCount int

	// EnqueueTimeout overrides AsyncEnqueueTimeout when non-zero. Tests use
	// this to verify the < 1ms production target (default) and the > 1ms
	// overflow path.
	EnqueueTimeout time.Duration

	// Logger overrides slog.Default() when non-nil. Tests pass a discard
	// logger to keep test output clean.
	Logger *slog.Logger
}

// NewAsyncLearner wraps the synchronous Learner with an async queue +
// background workers. The returned AsyncLearner is ready to use; callers
// MUST call Shutdown() to drain and clean up.
func NewAsyncLearner(inner Learner, opts AsyncLearnerOptions) *AsyncLearner {
	queueSize := opts.QueueSize
	if queueSize <= 0 {
		queueSize = AsyncQueueSize
	}
	workerCount := opts.WorkerCount
	if workerCount <= 0 {
		workerCount = AsyncWorkerCount
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	a := &AsyncLearner{
		inner:          inner,
		queue:          make(chan LearnRequest, queueSize),
		shutdownCh:     make(chan struct{}),
		enqueueTimeout: AsyncEnqueueTimeout,
		logger:         logger,
	}
	if opts.EnqueueTimeout > 0 {
		a.enqueueTimeout = opts.EnqueueTimeout
	}

	// Start workers.
	for i := 0; i < workerCount; i++ {
		a.workers.Add(1)
		go a.workerLoop(i)
	}

	logger.Info("async_learner_started",
		slog.Int("queue_size", queueSize),
		slog.Int("worker_count", workerCount),
	)
	return a
}

// Enqueue submits a LearnRequest for async processing. Returns nil on
// successful enqueue, ErrAsyncQueueFull when the queue is at capacity and
// the configured EnqueueTimeout elapses.
//
// Spec target: < 1ms in production. The default AsyncEnqueueTimeout (1ms)
// is the upper bound; tests assert on this.
//
// Behavior when Shutdown() has been called: Enqueue returns ErrAsyncQueueFull
// immediately (the channel is being drained; new requests are dropped with
// an audit log entry).
func (a *AsyncLearner) Enqueue(ctx context.Context, req LearnRequest) error {
	if a.closed.Load() {
		a.dropped.Add(1)
		a.logger.Warn("async_learner_enqueue_after_shutdown",
			slog.String("session_id", req.SessionID),
			slog.String("verdict_kind", req.Verdict.Kind.String()),
		)
		return ErrAsyncQueueFull
	}

	// Use a short non-blocking send via select with the shutdown channel.
	// The configured enqueueTimeout is the spec upper bound (1ms in
	// production); on overflow we drop with audit rather than block
	// (Learn cannot back-pressure channel close).
	timer := time.NewTimer(a.enqueueTimeout)
	defer timer.Stop()

	select {
	case a.queue <- req:
		a.enqueued.Add(1)
		a.submitted.Add(1)
		return nil
	case <-timer.C:
		a.dropped.Add(1)
		a.logger.Warn("async_learner_queue_full_drop",
			slog.String("session_id", req.SessionID),
			slog.String("verdict_kind", req.Verdict.Kind.String()),
			slog.Int("queue_depth", len(a.queue)),
		)
		return ErrAsyncQueueFull
	case <-a.shutdownCh:
		a.dropped.Add(1)
		return ErrAsyncQueueFull
	}
}

// Learn is the synchronous entry point — delegates to the inner Learner
// directly. Implements the Learner interface; callers that want sync
// semantics use this, callers that want async semantics use Enqueue.
//
// Why expose Learn on AsyncLearner: the Learner interface contract requires
// it; tests + occasional direct-call sites use it. Production callers
// typically use Enqueue + Drain.
func (a *AsyncLearner) Learn(ctx context.Context, req LearnRequest) ([]*LearningAsset, error) {
	return a.inner.Learn(ctx, req)
}

// Inject returns an AdaptivePrior for the next Observe call. Delegates to
// the inner Learner; the async queue is not involved.
func (a *AsyncLearner) Inject(ctx context.Context, sessionID, trackModeHint string) (*AdaptivePrior, error) {
	return a.inner.Inject(ctx, sessionID, trackModeHint)
}

// ScheduledTick drains ScheduledMemory entries. Delegates to inner.
func (a *AsyncLearner) ScheduledTick(ctx context.Context) error {
	return a.inner.ScheduledTick(ctx)
}

// Drain blocks until all currently-queued + in-flight LearnRequests complete.
// Use this in test setups (after Enqueue) and at session end (after the last
// Enqueue) to ensure all Learn side effects have been written.
//
// Note: Drain does NOT close the queue — subsequent Enqueue calls are still
// accepted. Use Shutdown for a hard close.
//
// Implementation: spin-wait on the submitted/completed counters (cheap;
// counters are atomic and the loop polls every 5ms). The submitted counter
// is incremented at successful Enqueue; completed is incremented after the
// worker finishes the inner Learn (success or fail).
func (a *AsyncLearner) Drain(ctx context.Context) error {
	for {
		if a.submitted.Load() == a.completed.Load() {
			return nil
		}
		select {
		case <-time.After(5 * time.Millisecond):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Shutdown gracefully closes the async learner: signals workers to drain
// the queue and exit. Idempotent — subsequent calls are no-ops.
//
// After Shutdown, Enqueue returns ErrAsyncQueueFull. Drain / Learn / Inject
// continue to work for any requests already enqueued.
func (a *AsyncLearner) Shutdown(ctx context.Context) error {
	a.shutdownOnce.Do(func() {
		a.closed.Store(true)
		close(a.shutdownCh)
		close(a.queue)
	})
	// Wait for workers to drain + exit, with a context-bounded timeout.
	done := make(chan struct{})
	go func() {
		a.workers.Wait()
		close(done)
	}()
	select {
	case <-done:
		a.logger.Info("async_learner_shutdown_complete",
			slog.Int64("processed", a.processed.Load()),
			slog.Int64("failed", a.failed.Load()),
			slog.Int64("dropped", a.dropped.Load()),
		)
		return nil
	case <-ctx.Done():
		a.logger.Warn("async_learner_shutdown_timeout",
			slog.Int64("processed", a.processed.Load()),
			slog.Int64("failed", a.failed.Load()),
			slog.Int64("dropped", a.dropped.Load()),
		)
		return ctx.Err()
	}
}

// Metrics returns a snapshot of the learner's counters. Used by the D5
// dashboard + the test assertions.
type AsyncMetrics struct {
	Enqueued   int64
	Dropped    int64
	Processed  int64
	Failed     int64
	Submitted  int64
	Completed  int64
	QueueDepth int
}

// Metrics returns a point-in-time snapshot.
func (a *AsyncLearner) Metrics() AsyncMetrics {
	return AsyncMetrics{
		Enqueued:   a.enqueued.Load(),
		Dropped:    a.dropped.Load(),
		Processed:  a.processed.Load(),
		Failed:     a.failed.Load(),
		Submitted:  a.submitted.Load(),
		Completed:  a.completed.Load(),
		QueueDepth: len(a.queue),
	}
}

// workerLoop is the background goroutine that drains the queue and calls
// inner.Learn. Multiple workers run in parallel; the inner Learner's
// ReputationStore mutex serializes per-session state.
//
// Each iteration increments completed AFTER inner.Learn returns (success
// or fail) so Drain's submitted/completed comparison is sound.
func (a *AsyncLearner) workerLoop(id int) {
	defer a.workers.Done()
	for req := range a.queue {
		// Use a 5-second timeout for the actual Learn call. The default
		// Learner already has its own internal timeouts; this is a hard
		// ceiling for hung BayesianUpdate / Memory.Store paths.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := a.inner.Learn(ctx, req)
		cancel()
		a.completed.Add(1)
		if err != nil {
			a.failed.Add(1)
			a.logger.Warn("async_learner_worker_learn_failed",
				slog.Int("worker_id", id),
				slog.String("session_id", req.SessionID),
				slog.String("err", err.Error()),
			)
			continue
		}
		a.processed.Add(1)
	}
}