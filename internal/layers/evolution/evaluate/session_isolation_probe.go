package evaluate

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/multiagent/observability"
)

func init() {
	RegisterProbe(&SessionIsolationProbe{})
}

// SessionIsolationProbe measures the SessionView COW invariants in
// the multi-agent layer. It is the D6 (evolution/eval) counterpart
// of the D4-S3-A01-T02 (metadata isolation) and D4-S3-A01-T03 (concurrent
// Fork + Join) test points.
//
// Inputs expected on EvalItem.Input:
//   - fork_count  (int):   how many children were forked
//   - join_count  (int):   how many children were joined
//   - metadata_writes (int): how many SetMetadata calls ran
//   - isolation_violations (int): how many reads of the parent
//     session saw a child-written key (must be 0 for a perfect score)
//
// The probe is deterministic: it does not call the Judge LLM; it
// computes the score from the inputs so a CI gate can use it
// without paying LLM cost.
type SessionIsolationProbe struct{}

func (p *SessionIsolationProbe) ID() string { return "session_isolation" }

func (p *SessionIsolationProbe) Run(ctx context.Context, item EvalItem, jm Judge) (*DomainScore, error) {
	_ = ctx
	_ = jm

	forkCount := intFromInput(item.Input, "fork_count")
	joinCount := intFromInput(item.Input, "join_count")
	writes := intFromInput(item.Input, "metadata_writes")
	violations := intFromInput(item.Input, "isolation_violations")

	// Isolation rate: 1.0 when no violations, 0.0 when all writes
	// leaked to the parent. With 0 writes the rate is vacuously 1.0.
	isolation := 1.0
	if writes > 0 {
		isolation = 1.0 - float64(violations)/float64(writes)
		if isolation < 0 {
			isolation = 0
		}
	}

	// Join consistency: forks and joins must be in 1:1 correspondence
	// to count as consistent. Off-by-one is treated as 0.0 because
	// it indicates dropped or duplicated children.
	joinConsistency := 0.0
	if forkCount == joinCount && forkCount > 0 {
		joinConsistency = 1.0
	} else if forkCount == 0 && joinCount == 0 {
		joinConsistency = 1.0
	}

	// D5 counter cross-check: the local counter must reflect at least
	// the fork_count, otherwise the metric path is broken.
	policySnap := observability.Snapshot()
	metricTotal := int64(0)
	for _, v := range policySnap {
		metricTotal += v
	}
	metricOK := 1.0
	if int64(forkCount) > metricTotal {
		metricOK = 0
	}

	score := (isolation + joinConsistency + metricOK) / 3.0

	details := map[string]float64{
		"isolation_rate":   isolation,
		"join_consistency": joinConsistency,
		"metric_ok":        metricOK,
		"fork_count":       float64(forkCount),
		"join_count":       float64(joinCount),
		"metadata_writes":  float64(writes),
		"violations":       float64(violations),
		"metric_cow_count": float64(policySnap[observability.PolicyCow]),
		"metric_total":     float64(metricTotal),
	}

	buckets := map[string]float64{}
	if item.Bucket != "" {
		buckets[item.Bucket] = score
	}

	return &DomainScore{
		Domain:     "d6",
		Dimension:  p.ID(),
		Score:      score,
		Confidence: 1.0, // deterministic probe
		Buckets:    buckets,
		Details:    details,
	}, nil
}

func intFromInput(input map[string]any, key string) int {
	if input == nil {
		return 0
	}
	v, ok := input[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		_ = fmt.Sprintf("unexpected type for %s: %T", key, v)
		return 0
	}
}
