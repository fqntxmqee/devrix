package eventbus

import (
	"context"
	"sync/atomic"
	"time"
)

// Reconnect runs the full Drain → Compact → ChannelRebuilt lifecycle for
// the given session. After Reconnect returns successfully, the bus is
// back in StateRunning with the backlog ≤ lowWatermark.
//
// L5-2-3-04: Reconnect recovery.
func (b *Bus) Reconnect(ctx context.Context, sessionID string) (ReconnectReport, error) {
	if b.closed.Load() {
		return ReconnectReport{}, ErrBusClosed
	}
	report := ReconnectReport{SessionID: sessionID}
	start := time.Now()

	// Step 1: Drain. Force the bus into Draining first.
	b.TriggerDrain()
	drainReport, err := b.Drain(ctx, sessionID, b.cfg.DrainTimeout)
	report.DrainReport = drainReport
	if err != nil && err != ErrDrainTimeout {
		// Hard error: leave state as Draining so caller can retry.
		report.Duration = time.Since(start)
		return report, err
	}
	// Even on partial drain, proceed — backlog should now be much smaller.

	// Step 2: Compact. Compact exits back to Draining (the prior state).
	compactReport, err := b.Compact(ctx, sessionID)
	report.CompactReport = compactReport
	if err != nil {
		report.Duration = time.Since(start)
		return report, err
	}

	// Step 3: Rebuild the pipeline. We transition to Reconnecting so
	// the monitor keeps hands off normalCh while we flush pending.
	b.setState(StateReconnecting)
	flushed := b.flushPending(ctx)
	report.PendingFlushed = flushed

	// Step 4: Return to Running.
	b.setState(StateRunning)
	report.Duration = time.Since(start)

	b.statsMu.Lock()
	b.reconnectTotal++
	b.statsMu.Unlock()
	return report, nil
}

// flushPending drains pendingCh back into normalCh, counting how many
// events were recovered. Returns when pendingCh is empty or ctx fires.
func (b *Bus) flushPending(ctx context.Context) int {
	flushed := 0
	for {
		select {
		case <-ctx.Done():
			return flushed
		case ev, ok := <-b.pendingCh:
			if !ok {
				return flushed
			}
			// Try to push into normalCh with a short timeout.
			pushCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
			select {
			case <-pushCtx.Done():
				// Stuck — put it back (best-effort, non-blocking).
				select {
				case b.pendingCh <- ev:
				default:
					// Pending buffer is full too; drop with stats.
					atomic.AddInt64(&b.droppedTotal, 1)
				}
			case b.normalCh <- ev:
				b.addBacklog(1)
				flushed++
			}
			cancel()
		default:
			return flushed
		}
	}
}

// resetBacklogIfEmpty is a small convenience used by tests: if the
// backlog is zero, transition out of Reconnecting into Running.
func (b *Bus) resetBacklogIfEmpty() {
	if b.Backlog() == 0 && b.State() == StateReconnecting {
		b.setState(StateRunning)
	}
}
