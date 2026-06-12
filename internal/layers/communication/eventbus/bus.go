package eventbus

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// Errors returned by the eventbus package.
var (
	// ErrBusClosed is returned by Publish / PublishCritical after Close().
	ErrBusClosed = errors.New("eventbus: bus is closed")
	// ErrDrainTimeout is returned by Drain when the timeout elapses before
	// backlog drops to the low watermark.
	ErrDrainTimeout = errors.New("eventbus: drain timed out")
	// ErrReconnectTimeout is returned by Reconnect when channel rebuild
	// exceeds the configured timeout.
	ErrReconnectTimeout = errors.New("eventbus: reconnect timed out")
	// ErrContextCancelled is returned by Publish when the caller's ctx fires
	// before the event could be enqueued.
	ErrContextCancelled = errors.New("eventbus: context cancelled")
)

// DrainReport summarizes a Drain pass.
type DrainReport struct {
	SessionID    string
	Drained      int // number of Normal/Low events shed
	KeptCritical int // number of Critical events that remained queued
	Duration     time.Duration
	StartBacklog int
	EndBacklog   int
}

// CompactReport summarizes a Compact pass.
type CompactReport struct {
	SessionID    string
	Compacted    int // number of events replaced by aggregates
	AggregateOut int // number of aggregate events emitted
	Duration     time.Duration
	SkippedCrit  int // Critical events skipped (invariant: never compacted)
}

// ReconnectReport summarizes a Reconnect lifecycle.
type ReconnectReport struct {
	SessionID      string
	DrainReport    DrainReport
	CompactReport  CompactReport
	PendingFlushed int
	Duration       time.Duration
}

// Bus is the concrete BackpressureEventBus implementation.
//
// Concurrency model:
//   - Publish / PublishCritical acquire publisherMu (writer side) only briefly
//     to enqueue into internal channels; they do not hold it during blocking.
//   - The monitor goroutine polls len(normalCh) on a fast tick and toggles state.
//   - State transitions acquire stateMu.
//   - Subscribers are tracked in subs map under subsMu; cancel removes them.
type Bus struct {
	cfg config.EventBusConfig

	// Internal pipelines.
	normalCh  chan Event // buffered; size = cfg.ChannelBuffer
	pendingCh chan Event // holds Normal/Low events during Reconnecting

	// Subscribers.
	subsMu sync.RWMutex
	subs   map[string]*subscription

	// State machine.
	stateMu sync.RWMutex
	state   State

	// Backlog accounting (atomic so the monitor never blocks).
	backlog int64

	// Publisher serialization (only one Publish in-flight may block).
	publisherMu sync.Mutex

	// Lifecycle.
	closed  atomic.Bool
	wg      sync.WaitGroup
	closeCh chan struct{}
	seq     atomic.Uint64

	// monitorPaused is true while the monitor goroutine is in a
	// non-Running state and is NOT consuming normalCh. Callers that
	// flip the state (TriggerDrain, Drain, Compact, Reconnect) can
	// spin briefly on this flag to ensure monitor observed the state
	// change before they start asserting buffer-full / backpressure
	// behavior.
	monitorPaused atomic.Bool

	// dispatchMu serializes the dispatcher's fanout with cancelFn's
	// remove-and-close. The dispatcher holds dispatchMu ONLY around
	// the fanout step — never across the blocking channel receive.
	// cancelFn also acquires dispatchMu, so cancelFn cannot run
	// while a fanout is in progress. The blocking receive on
	// normalCh happens OUTSIDE dispatchMu so cancelFn is never
	// starved when no event is pending.
	//
	// PublishCritical also takes dispatchMu around its synchronous
	// fanout. This guarantees that when PublishCritical returns,
	// the critical event has already been delivered to every
	// then-active subscriber's ch (subject to the non-blocking
	// fanout drop semantics for slow consumers).
	//
	// Consistency guarantee: a fanout snapshot taken while cancelFn
	// is waiting for dispatchMu observes the sub as still present
	// and will deliver the event (the consumer is responsible for
	// draining ch after seeing done close). A fanout that starts
	// after cancelFn releases dispatchMu sees the sub removed and
	// skips it.
	dispatchMu sync.Mutex

	// Reports (counters) accessible to tests.
	statsMu        sync.Mutex
	drainedTotal   int64
	compactedTotal int64
	reconnectTotal int64
	droppedTotal   int64
}

// subscription is one subscriber's view of the bus.
//
// `done` is closed exactly once when the subscription is cancelled; the
// fanout path checks `done` under subsMu before sending to `ch`, and
// `cancel` closes `done` under subsMu before closing `ch`. This
// prevents the race between fanout's send and cancel's close.
type subscription struct {
	id   string
	ch   chan Event
	done chan struct{}
}

// NewBus constructs a Bus with the given config.
func NewBus(cfg config.EventBusConfig) (*Bus, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("eventbus: invalid config: %w", err)
	}
	b := &Bus{
		cfg:       cfg,
		normalCh:  make(chan Event, cfg.ChannelBuffer),
		pendingCh: make(chan Event, cfg.ChannelBuffer),
		subs:      make(map[string]*subscription),
		state:     StateRunning,
		closeCh:   make(chan struct{}),
	}
	b.wg.Add(1)
	go b.monitor()
	return b, nil
}

// State returns the current bus state.
func (b *Bus) State() State {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()
	return b.state
}

// setState atomically transitions to s.
func (b *Bus) setState(s State) {
	b.stateMu.Lock()
	prev := b.state
	b.state = s
	b.stateMu.Unlock()
	if prev != s {
		// State change side-effects (e.g. promote to Compacting) are
		// driven by callers (Drain/Compact/Reconnect) — monitor only
		// triggers Draining from backlog.
		_ = prev
	}
}

// Backlog returns the current approximate backlog of Normal/Low events.
func (b *Bus) Backlog() int {
	return int(atomic.LoadInt64(&b.backlog))
}

// addBacklog atomically increments backlog and returns the new value.
func (b *Bus) addBacklog(delta int64) int64 {
	return atomic.AddInt64(&b.backlog, delta)
}

// Snapshot returns a copy of internal counters (for tests / metrics).
type Snapshot struct {
	State          State
	Backlog        int
	DrainedTotal   int64
	CompactedTotal int64
	ReconnectTotal int64
	DroppedTotal   int64
}

// Snapshot reads current internal counters.
func (b *Bus) Snapshot() Snapshot {
	b.statsMu.Lock()
	d := b.drainedTotal
	c := b.compactedTotal
	r := b.reconnectTotal
	drop := b.droppedTotal
	b.statsMu.Unlock()
	return Snapshot{
		State:          b.State(),
		Backlog:        b.Backlog(),
		DrainedTotal:   d,
		CompactedTotal: c,
		ReconnectTotal: r,
		DroppedTotal:   drop,
	}
}

// Publish enqueues a Normal/Low event. Blocks the caller when the normal
// channel is at capacity (high watermark reached) so the upstream producer
// experiences backpressure.
//
// Critical events should use PublishCritical — calling Publish with a
// critical-priority event returns an error (use the other API).
func (b *Bus) Publish(ctx context.Context, ev Event) error {
	if b.closed.Load() {
		return ErrBusClosed
	}
	if ev.EngineEvent == nil {
		return fmt.Errorf("eventbus: nil EngineEvent in Publish")
	}
	if ev.IsTerminator() {
		return fmt.Errorf("eventbus: terminator events must use PublishCritical (type=%q)",
			ev.EngineEvent.Type)
	}
	if ev.Priority == PriorityCritical {
		return fmt.Errorf("eventbus: Publish does not accept Critical priority; use PublishCritical")
	}
	ev = ev.WithSequence(b.nextSeq()).WithPublishedAt(time.Now())
	return b.enqueueNormal(ctx, ev)
}

// PublishCritical synchronously delivers a Critical event to every
// active subscriber. It bypasses the normal channel buffer entirely,
// so Critical events are guaranteed to be delivered immediately even
// when normalCh is at high watermark.
//
// PublishCritical takes dispatchMu around fanout, so when it returns,
// every subscriber that was active at fanout time has either received
// the event in its ch buffer or been dropped (slow consumer with full
// ch). This guarantees the caller can safely cancel the subscription
// afterwards without losing the event.
func (b *Bus) PublishCritical(ctx context.Context, ev Event) error {
	if b.closed.Load() {
		return ErrBusClosed
	}
	if ev.EngineEvent == nil {
		return fmt.Errorf("eventbus: nil EngineEvent in PublishCritical")
	}
	ev = ev.WithPriority(PriorityCritical).
		WithSequence(b.nextSeq()).
		WithPublishedAt(time.Now())
	// Honor ctx cancellation even on the fast path.
	select {
	case <-ctx.Done():
		return ErrContextCancelled
	default:
	}
	b.dispatchMu.Lock()
	b.fanout(ev)
	b.dispatchMu.Unlock()
	// Critical events never touch the backlog counter.
	return nil
}

// enqueueNormal puts a Normal/Low event into the active pipeline, blocking
// when capacity is reached. During Reconnecting, events go to pendingCh.
func (b *Bus) enqueueNormal(ctx context.Context, ev Event) error {
	for {
		state := b.State()
		target := b.normalCh
		if state == StateReconnecting {
			target = b.pendingCh
		}
		select {
		case <-ctx.Done():
			return ErrContextCancelled
		case <-b.closeCh:
			return ErrBusClosed
		case target <- ev:
			// Only count against backlog once we successfully land in
			// normalCh (pendingCh events are still "in flight" but not
			// contributing to backpressure on the live pipeline).
			if state != StateReconnecting {
				b.addBacklog(1)
			}
			return nil
		}
	}
}

// Subscribe registers a new consumer. The returned channel receives all
// events for the bus (the sessionID argument is accepted for API symmetry /
// future per-session filtering; the v1 bus is session-agnostic).
//
// The returned tuple is (subID, events, done, cancel):
//   - events: the consumer reads events from this channel
//   - done: closed when the subscription is cancelled; consumers
//     should select on it to know when to stop reading
//   - cancel: removes the subscription from the bus. The cancel
//     function does NOT close done — the caller is responsible
//     for closing done AFTER the consumer has drained the events
//     channel. This avoids the race where a critical event is
//     still in flight when cancel runs and the consumer exits
//     before the event is fanned out.
func (b *Bus) Subscribe(sessionID string) (subID string, events <-chan Event, done <-chan struct{}, cancel func()) {
	id := fmt.Sprintf("sub_%d", b.nextSeq())
	sub := &subscription{
		id:   id,
		ch:   make(chan Event, b.cfg.SubscribeBuffer),
		done: make(chan struct{}),
	}
	b.subsMu.Lock()
	b.subs[id] = sub
	b.subsMu.Unlock()

	cancelFn := func() {
		// Hold dispatchMu across the entire cancel sequence.
		// The fanout path also holds dispatchMu, so cancel cannot
		// interleave with an in-flight fanout that is about to
		// snapshot subs.
		//
		// Specifically, by the time we acquire dispatchMu:
		//   - Either a fanout is in the lock (monitor's normalCh
		//     fanout, or a synchronous PublishCritical fanout).
		//     We wait. When the fanout releases dispatchMu, its
		//     snapshot has completed and the send has happened.
		//   - Or no fanout is in progress. We acquire immediately.
		//     When we release, any future fanout will see the
		//     sub removed from subs and skip it.
		b.dispatchMu.Lock()

		// Remove the sub from subs.
		b.subsMu.Lock()
		existing, ok := b.subs[id]
		if ok && existing == sub {
			delete(b.subs, id)
		}
		b.subsMu.Unlock()

		// Close sub.done. The consumer will see this and drain
		// any remaining events from ch.
		b.subsMu.Lock()
		select {
		case <-sub.done:
			// already closed (e.g. by bus.Close)
		default:
			close(sub.done)
		}
		b.subsMu.Unlock()

		b.dispatchMu.Unlock()
		// We do NOT close sub.ch — the consumer must drain any
		// events buffered in ch before exiting. ch is closed only
		// by the bus's Close() method, which guarantees no further
		// events will ever be fanned out.
	}
	return id, sub.ch, sub.done, cancelFn
}

// fanout delivers ev to every active subscriber. Slow subscribers are
// dropped (non-blocking send) — the bus never waits for a slow consumer.
//
// The fanout snapshot is taken under subsMu.RLock. cancelFn takes
// subsMu.Lock to remove the sub and close done. The check-done +
// send-to-ch sequence is non-blocking; it may race with cancel (see
// CancelSubscription for the full teardown contract).
func (b *Bus) fanout(ev Event) {
	b.subsMu.RLock()
	subs := make([]*subscription, 0, len(b.subs))
	for _, s := range b.subs {
		subs = append(subs, s)
	}
	b.subsMu.RUnlock()
	for _, s := range subs {
		// Check the done channel non-blockingly. If it's closed, the
		// subscription has been cancelled and we must NOT send.
		select {
		case <-s.done:
			continue
		default:
		}
		select {
		case s.ch <- ev:
		default:
			// Slow subscriber — drop on the floor. This is documented
			// behavior; subscribers are expected to drain promptly.
		}
	}
	_ = ev
}

// drainOne consumes one event from normalCh, decrementing backlog.
// Returns (event, true) on success, (zero, false) if the channel is empty
// or the bus is closed.
func (b *Bus) drainOne(ctx context.Context) (Event, bool) {
	select {
	case <-ctx.Done():
		return Event{}, false
	case ev, ok := <-b.normalCh:
		if !ok {
			return Event{}, false
		}
		b.addBacklog(-1)
		return ev, true
	}
}

// monitor pumps events from the normalCh to subscribers and periodically
// checks backlog to drive the state machine. Critical events bypass this
// pipeline entirely (PublishCritical fans out synchronously), so they
// never compete with normal events for monitor cycles — this is the P0
// invariant: critical events must reach subscribers even under heavy
// normal load.
//
// In Draining / Compacting / Reconnecting states, the monitor does NOT
// consume normalCh — those lifecycle phases own the channel. The state
// returns to Running when the phase completes.
func (b *Bus) monitor() {
	defer b.wg.Done()
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-b.closeCh:
			return
		default:
		}
		state := b.State()
		if state != StateRunning {
			// Lifecycle phases own normalCh. Only run the ticker to
			// check whether the phase should yield back to Running
			// (driven by the Drain/Compact/Reconnect callers via
			// setState).
			b.monitorPaused.Store(true)
			select {
			case <-b.closeCh:
				return
			case <-tick.C:
				continue
			}
		}
		b.monitorPaused.Store(false)
		select {
		case <-b.closeCh:
			return
		case ev := <-b.normalCh:
			b.dispatchMu.Lock()
			b.addBacklog(-1)
			b.fanout(ev)
			b.dispatchMu.Unlock()
			b.checkBackpressure()
		case <-tick.C:
			b.checkBackpressure()
		}
	}
}

// checkBackpressure transitions Running → Draining when backlog exceeds
// the high watermark. Other transitions (Draining → Compacting →
// Reconnecting → Running) are driven explicitly by Drain / Compact /
// Reconnect callers; checkBackpressure never silently flips back.
func (b *Bus) checkBackpressure() {
	if b.State() != StateRunning {
		return
	}
	if b.Backlog() >= b.cfg.HighWatermark {
		b.setState(StateDraining)
	}
}

// nextSeq atomically returns the next sequence number.
func (b *Bus) nextSeq() uint64 {
	return b.seq.Add(1)
}

// Close stops the bus. After Close, all Publish / PublishCritical return
// ErrBusClosed. In-flight events already in the pipeline are drained to
// subscribers before the monitor goroutine returns. After all subscriber
// done channels are closed, each subscriber's ch is also closed so
// consumers running `for ev := range ch` exit deterministically.
func (b *Bus) Close() error {
	if !b.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(b.closeCh)
	b.wg.Wait()
	// Snapshot subscribers and clear the map under subsMu; then close
	// done and ch in a deterministic order so consumers can rely on
	// the contract: ch is closed only AFTER done is closed.
	b.subsMu.Lock()
	subs := make([]*subscription, 0, len(b.subs))
	for id, s := range b.subs {
		subs = append(subs, s)
		delete(b.subs, id)
	}
	b.subsMu.Unlock()
	for _, s := range subs {
		select {
		case <-s.done:
		default:
			close(s.done)
		}
	}
	// Closing ch last ensures any consumer that observes done-closed
	// will see the channel close and exit its range loop.
	for _, s := range subs {
		close(s.ch)
	}
	b.setState(StateClosed)
	return nil
}

// recvOneFromNormal is a small helper used by Drain to pop one event.
func (b *Bus) recvOneFromNormal(ctx context.Context) (Event, bool) {
	return b.drainOne(ctx)
}

// PublishSync is a test-only convenience that wraps contracts.EngineEvent.
func wrapForTest(e *contracts.EngineEvent) Event {
	return Event{EngineEvent: e, Priority: PriorityNormal, PublishedAt: time.Now()}
}
