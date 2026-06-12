package eventbus

import (
	"context"
	"sync/atomic"
	"time"
)

// Drain blocks until the backlog is at or below the low watermark OR the
// timeout/context expires. Normal/Low events that can be safely shed are
// discarded (counted in Drained). Critical events (which never enter the
// normal channel) are never affected.
//
// On entry, Drain forces the bus into StateDraining (if it was Running);
// the monitor goroutine will then stop consuming normalCh so this
// caller owns the channel. On exit (success or timeout), Drain
// transitions back to StateRunning so the monitor can resume.
//
// Pre-conditions: caller should be in StateDraining (use TriggerDrain to
// flip from Running). If the bus is in another state, Drain still works
// but is a no-op (returns zero report).
//
// L5-2-3-02: Backpressure triggers Drain.
func (b *Bus) Drain(ctx context.Context, sessionID string, timeout time.Duration) (DrainReport, error) {
	if b.closed.Load() {
		return DrainReport{}, ErrBusClosed
	}
	// If we're in Running, force into Draining.
	if b.State() == StateRunning {
		b.setState(StateDraining)
	}
	defer func() {
		// Always return to Running so the monitor resumes.
		if b.State() == StateDraining {
			b.setState(StateRunning)
		}
	}()

	deadline := time.Now().Add(timeout)
	if timeout <= 0 {
		deadline = time.Now().Add(b.cfg.DrainTimeout)
	}
	dctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	report := DrainReport{
		SessionID:    sessionID,
		StartBacklog: b.Backlog(),
	}
	start := time.Now()

	// Loop: pull events off normalCh, shed Low/Normal, count Critical
	// (Critical never enters normalCh, so it is implicitly kept — we still
	// count it as 0 in KeptCritical for honesty).
	for {
		if b.Backlog() <= b.cfg.LowWatermark {
			break
		}
		// Try to pull one event with a short wait so we can re-check backlog.
		pullCtx, pullCancel := context.WithTimeout(dctx, 50*time.Millisecond)
		ev, ok := b.drainOne(pullCtx)
		pullCancel()
		if !ok {
			// Timed out without pulling. Check ctx / deadline.
			select {
			case <-dctx.Done():
				report.EndBacklog = b.Backlog()
				report.Duration = time.Since(start)
				b.statsMu.Lock()
				b.drainedTotal += int64(report.Drained)
				b.statsMu.Unlock()
				if report.EndBacklog > b.cfg.LowWatermark {
					return report, ErrDrainTimeout
				}
				return report, nil
			default:
				continue
			}
		}
		// Critical events are never in normalCh; this branch is defensive.
		if ev.Priority == PriorityCritical {
			report.KeptCritical++
			b.fanout(ev)
			continue
		}
		// Shed Normal/Low.
		report.Drained++
		atomic.AddInt64(&b.droppedTotal, 1)
	}

	report.EndBacklog = b.Backlog()
	report.Duration = time.Since(start)
	b.statsMu.Lock()
	b.drainedTotal += int64(report.Drained)
	b.statsMu.Unlock()
	return report, nil
}

// TriggerDrain forces the bus into StateDraining. Idempotent if already
// draining. Used by the monitor and by tests.
//
// On success, this also waits briefly (up to ~50ms = ~2 monitor ticks)
// for the monitor goroutine to observe the state change and stop
// consuming from normalCh. This is necessary so callers that intend
// to demonstrate "Publish blocks at high watermark" can fill the
// channel buffer without the monitor draining it concurrently.
func (b *Bus) TriggerDrain() {
	if b.State() == StateRunning {
		b.setState(StateDraining)
	}
	if b.State() == StateDraining {
		// Spin briefly waiting for monitor to acknowledge the
		// state change. The monitor ticker fires every 20ms, so
		// 50ms covers two full ticks plus jitter.
		deadline := time.Now().Add(50 * time.Millisecond)
		for time.Now().Before(deadline) {
			if b.monitorPaused.Load() {
				return
			}
			time.Sleep(time.Millisecond)
		}
	}
}
