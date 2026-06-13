package eventbus

import (
	"context"
	"sync/atomic"
	"time"
)

// compactGroup holds one batch of same-type events being merged.
type compactGroup struct {
	eventType string
	priority  Priority
	first     Event
	count     int
	extras    []Event
}

// Compact merges adjacent same-type Normal/Low events into a single
// aggregate event whose Metadata["compacted_count"] holds the merged count.
// Critical events are never compacted.
//
// The algorithm:
//  1. Force state into Compacting so the monitor stops consuming normalCh.
//  2. Pull up to CompactMaxBatch events from normalCh.
//  3. Group by Type.
//  4. For each group of size > 1, emit ONE aggregate event (fanout) and
//     count (size-1) toward "compacted".
//  5. Re-fanout ungrouped singletons as-is.
//  6. Return to prior state (Draining or Running) so the caller can
//     continue the lifecycle.
//
// D1-S9-A02-T03: Compact consecutive same-type events.
func (b *Bus) Compact(ctx context.Context, sessionID string) (CompactReport, error) {
	if b.closed.Load() {
		return CompactReport{}, ErrBusClosed
	}
	prev := b.State()
	if prev == StateRunning || prev == StateDraining {
		b.setState(StateCompacting)
	}
	defer func() {
		// Restore the prior state (Drain→Compact→Reconnect flow owns
		// the subsequent transitions; Compact itself returns to the
		// prior state unless Reconnect is in flight).
		if b.State() == StateCompacting {
			b.setState(prev)
		}
	}()

	report := CompactReport{SessionID: sessionID}
	start := time.Now()

	// Lock-step pull: we need to "re-enqueue" events that we couldn't
	// compact, so we use a temp buffer.
	batch := make([]Event, 0, b.cfg.CompactMaxBatch)
	for len(batch) < b.cfg.CompactMaxBatch {
		select {
		case <-ctx.Done():
			report.Duration = time.Since(start)
			return report, ctx.Err()
		default:
		}
		pullCtx, pullCancel := context.WithTimeout(ctx, 25*time.Millisecond)
		ev, ok := b.drainOne(pullCtx)
		pullCancel()
		if !ok {
			break
		}
		if ev.Priority == PriorityCritical {
			// Should not happen (Critical never enters normalCh), but be
			// defensive: count as skipped, re-fanout unchanged.
			report.SkippedCrit++
			b.fanout(ev)
			continue
		}
		batch = append(batch, ev)
	}

	// Group by Type.
	groupOrder := make([]string, 0)
	groups := make(map[string]*compactGroup)
	for _, ev := range batch {
		t := ""
		if ev.EngineEvent != nil {
			t = ev.EngineEvent.Type
		}
		g, ok := groups[t]
		if !ok {
			g = &compactGroup{
				eventType: t,
				priority:  ev.Priority,
				first:     ev,
			}
			groups[t] = g
			groupOrder = append(groupOrder, t)
		} else {
			g.extras = append(g.extras, ev)
		}
		g.count++
	}

	// Emit aggregates / singletons.
	for _, t := range groupOrder {
		g := groups[t]
		if g.count <= 1 {
			// Nothing to compact; re-fanout the single event as-is.
			b.fanout(g.first)
			continue
		}
		// Build an aggregate: copy of first event with compacted_count metadata.
		agg := g.first
		if agg.Metadata == nil {
			agg.Metadata = map[string]string{}
		} else {
			// Shallow copy metadata so we don't mutate the original.
			md := make(map[string]string, len(agg.Metadata)+1)
			for k, v := range agg.Metadata {
				md[k] = v
			}
			agg.Metadata = md
		}
		agg.Metadata["compacted_count"] = itoa(g.count)
		agg.Metadata["compacted_extras"] = itoa(len(g.extras))
		b.fanout(agg)
		report.Compacted += g.count - 1
		report.AggregateOut++
	}

	// Apply accounting atomically.
	compacted := int64(report.Compacted)
	b.statsMu.Lock()
	b.compactedTotal += compacted
	b.statsMu.Unlock()

	report.Duration = time.Since(start)
	_ = atomic.LoadInt64 // keep import for hot-path counters in callers
	return report, nil
}

// itoa avoids importing strconv for a single tiny conversion in the hot
// path; small int → string.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
