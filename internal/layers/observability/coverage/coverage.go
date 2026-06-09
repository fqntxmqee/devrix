package coverage

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

const unknownOperation = "__unknown__"

// ZeroHitEntry describes an instrumented operation with no runtime hits.
type ZeroHitEntry struct {
	Operation    string `json:"operation"`
	Layer        string `json:"layer"`
	Component    string `json:"component"`
	SinceVersion string `json:"since_version"`
}

// Report is the coverage reconciliation output.
type Report struct {
	Since              time.Time         `json:"since"`
	OperationsTotal    int               `json:"operations_total"`
	OperationsHit      int               `json:"operations_hit"`
	OperationsZeroHit  []ZeroHitEntry    `json:"operations_zero_hit"`
	CoverageRatio      float64           `json:"coverage_ratio"`
	Hits               map[string]uint64 `json:"hits,omitempty"`
	UnknownHits        uint64            `json:"unknown_hits,omitempty"`
}

// Counter tracks per-operation hit counts for the process lifetime.
type Counter struct {
	since time.Time
	mu    sync.RWMutex
	hits  map[string]*atomic.Uint64
}

// NewCounter creates a counter seeded with instrumented operation names.
func NewCounter(ops []OperationMeta) *Counter {
	hits := make(map[string]*atomic.Uint64, len(ops)+1)
	for _, op := range ops {
		if !op.Instrumented {
			continue
		}
		hits[op.Name] = &atomic.Uint64{}
	}
	hits[unknownOperation] = &atomic.Uint64{}
	return &Counter{
		since: time.Now().UTC(),
		hits:  hits,
	}
}

// RecordHit increments the hit count for an operation name.
func (c *Counter) RecordHit(operation string) {
	if c == nil || operation == "" {
		return
	}
	c.mu.RLock()
	counter, ok := c.hits[operation]
	c.mu.RUnlock()
	if !ok {
		c.mu.Lock()
		counter, ok = c.hits[operation]
		if !ok {
			counter = &atomic.Uint64{}
			c.hits[operation] = counter
		}
		c.mu.Unlock()
	}
	counter.Add(1)
}

// RecordUnknown increments the unknown-operation bucket.
func (c *Counter) RecordUnknown() {
	c.RecordHit(unknownOperation)
}

// Report builds a reconciliation report against the operation registry.
func (c *Counter) Report(registry []OperationMeta, includeHits bool) Report {
	instrumented := make([]OperationMeta, 0, len(registry))
	for _, op := range registry {
		if op.Instrumented {
			instrumented = append(instrumented, op)
		}
	}

	hitCounts, unknownCount := c.snapshot()
	hit := 0
	zeroHit := make([]ZeroHitEntry, 0)
	for _, op := range instrumented {
		if hitCounts[op.Name] > 0 {
			hit++
			continue
		}
		zeroHit = append(zeroHit, ZeroHitEntry{
			Operation:    op.Name,
			Layer:        op.Layer,
			Component:    op.Component,
			SinceVersion: op.SinceVersion,
		})
	}

	total := len(instrumented)
	ratio := 0.0
	if total > 0 {
		ratio = math.Round((float64(hit)/float64(total))*1000) / 1000
	}

	report := Report{
		Since:              c.since,
		OperationsTotal:    total,
		OperationsHit:      hit,
		OperationsZeroHit:  zeroHit,
		CoverageRatio:      ratio,
		UnknownHits:        unknownCount,
	}
	if includeHits {
		report.Hits = hitCounts
	}
	return report
}

// Reset clears all hit counts (for tests).
func (c *Counter) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, counter := range c.hits {
		counter.Store(0)
	}
	c.since = time.Now().UTC()
}

func (c *Counter) snapshot() (map[string]uint64, uint64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]uint64, len(c.hits))
	var unknownCount uint64
	for name, counter := range c.hits {
		if name == unknownOperation {
			unknownCount = counter.Load()
			continue
		}
		out[name] = counter.Load()
	}
	return out, unknownCount
}

var (
	globalMu sync.RWMutex
	global   *Counter
)

// InitGlobal initializes the process-wide coverage counter.
func InitGlobal(ops []OperationMeta) {
	globalMu.Lock()
	defer globalMu.Unlock()
	global = NewCounter(ops)
}

// Global returns the process-wide counter (may be nil before InitGlobal).
func Global() *Counter {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return global
}

// RecordHit records on the global counter when initialized.
func RecordHit(operation string) {
	if c := Global(); c != nil {
		c.RecordHit(operation)
	}
}

// RecordUnknown increments the unknown-operation bucket on the global counter.
func RecordUnknown() {
	if c := Global(); c != nil {
		c.RecordUnknown()
	}
}
