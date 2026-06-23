package execute

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/shared/types"
)

// -----------------------------------------------------------------------------
// ExplorationChannel — multi-agent free-fork with priority ordering
// -----------------------------------------------------------------------------
//
// Doc 44 §3.5: ExplorationChannel produces an ArtifactExperimentData
// from parallel exploratory runs. Unlike ScenarioChannel, Exploration
// tolerates partial failure (any non-zero success count is acceptable);
// the Artifact captures all results and the highest-priority outcome
// wins. The Channel is sandboxed — side effects are explicitly scoped
// to the Plan's PersistScope (PR-C2 reads p.BlastRadius.PersistScope).
//
// Side-effect classification:
//   - p.BlastRadius.PersistScope == PersistTransient → SideEffectNone
//   - p.BlastRadius.PersistScope == PersistSession   → SideEffectCommitted
//   - p.BlastRadius.PersistScope == PersistPermanent → SideEffectCommitted (audit warning)
//
// Priority order: results are ranked by (1) absence of error, (2) shorter
// duration, (3) Step.EstimatedTokens (lower is better).

// ExplorationChannelConfig tunes ExplorationChannel's behavior.
type ExplorationChannelConfig struct {
	// Timeout caps the entire exploration batch. Default 30s.
	Timeout time.Duration
	// MaxParallel bounds concurrent exploration goroutines. Default 3
	// (smaller than ScenarioChannel because exploration is heavier).
	MaxParallel int
}

// ExplorationChannel is the per-ExplorationPlan execution unit.
type ExplorationChannel struct {
	runner ToolRunner
	cfg    ExplorationChannelConfig
}

// NewExplorationChannel constructs an ExplorationChannel.
func NewExplorationChannel(runner ToolRunner, cfg ExplorationChannelConfig) (*ExplorationChannel, error) {
	if runner == nil {
		return nil, NewChannelToolRunnerNilError("exploration")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxParallel <= 0 {
		cfg.MaxParallel = 3
	}
	return &ExplorationChannel{runner: runner, cfg: cfg}, nil
}

// Name returns the stable channel identifier.
func (c *ExplorationChannel) Name() string { return "exploration" }

// Supports reports whether this Channel handles the given PlanKind.
func (c *ExplorationChannel) Supports(pk plan.PlanKind) bool {
	return pk == plan.ExplorationPlan
}

// runOut is the per-goroutine result envelope produced by the
// exploration workers. Defined at package scope so the channel type is
// referenceable from tests and helpers.
type runOut struct {
	idx    int
	step   plan.Step
	result ToolResult
	err    error
}

// Execute runs all explorations in parallel, collects ALL results (no
// early-exit), and ranks them by priority. The Artifact carries every
// result for downstream analysis (Phase 5 Learn can mine the
// ExperimentData to update ReputationEvidence).
func (c *ExplorationChannel) Execute(ctx context.Context, p *plan.Plan, req ChannelRequest) (*wavescheduler.Artifact, error) {
	if p == nil {
		return nil, fmt.Errorf("%w", ErrChannelPlanNil)
	}
	if len(p.Steps) == 0 {
		return nil, NewChannelStepCountMismatchError(c.Name(), 0, 1, 0)
	}

	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	now := time.Now()
	art := &wavescheduler.Artifact{
		TaskID:         req.SessionID + ":exploration",
		SessionID:      req.SessionID,
		WorkerType:     wavescheduler.WorkerSubAgent,
		SourcePlanID:   p.ID,
		AnomaliesCount: p.AnomaliesCount,
		Kind:           types.ArtifactExperimentData,
		StartedAt:      now,
		// Side-effect status depends on the Plan's PersistScope.
		SideEffectStatus: sideEffectForScope(p.BlastRadius.PersistScope),
	}

	// Concurrency control: spawn all goroutines immediately, gate
	// parallelism with a semaphore INSIDE each goroutine. This avoids
	// the deadlock pattern where the main goroutine blocks on
	// `sem <- struct{}{}` waiting for an in-flight goroutine to release
	// its slot — a scenario that arose in the previous implementation
	// when MaxParallel < len(p.Steps). sync.WaitGroup guarantees we
	// wait for every worker; the buffered out channel guarantees no
	// worker blocks on send.
	sem := make(chan struct{}, c.cfg.MaxParallel)
	out := make(chan runOut, len(p.Steps))
	var wg sync.WaitGroup

	for i, step := range p.Steps {
		wg.Add(1)
		go func(idx int, s plan.Step) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			r, err := c.runner.Invoke(ctx, ToolRequest{
				SessionID:      req.SessionID,
				ToolName:       s.ToolName,
				Args:           s.ToolArgs,
				IdempotencyKey: s.IdempotencyKey,
				StepID:         s.ID,
			})
			out <- runOut{idx: idx, step: s, result: r, err: err}
		}(i, step)
	}

	// Close `out` after every worker has written its result, then drain.
	go func() {
		wg.Wait()
		close(out)
	}()

	results := make([]runOut, 0, len(p.Steps))
	for r := range out {
		results = append(results, r)
	}

	art.EndedAt = time.Now()
	art.Duration = art.EndedAt.Sub(art.StartedAt)

	// Rank: success first, then by duration ascending, then by EstimatedTokens.
	// For errored results, result.CompletedAt/StartedAt are zero values
	// (runner never set them), so duration is 0 and the secondary sort
	// falls through to EstimatedTokens. This puts errored results with
	// the smallest EstimatedTokens first among errors — a stable,
	// reproducible ordering that lets the caller rely on results[0]
	// when reporting "first failure" downstream.
	sort.SliceStable(results, func(i, j int) bool {
		if (results[i].err == nil) != (results[j].err == nil) {
			return results[i].err == nil
		}
		di := results[i].result.CompletedAt.Sub(results[i].result.StartedAt)
		dj := results[j].result.CompletedAt.Sub(results[j].result.StartedAt)
		if di != dj {
			return di < dj
		}
		return results[i].step.EstimatedTokens < results[j].step.EstimatedTokens
	})

	successCount := 0
	for _, r := range results {
		if r.err == nil {
			successCount++
		}
	}

	// Top result becomes Summary; full result table encoded in Metadata.
	// Top-error reporting prefers the result with the longest error
	// message (most informative for triage) over results[0] which is
	// the lowest-EstimatedTokens error — useful when the actual
	// failure context is buried in a different goroutine.
	if len(results) > 0 && results[0].err == nil {
		art.Summary = fmt.Sprintf("top_result: %s", results[0].result.Output)
		art.ExitCode = results[0].result.ExitCode
	} else if len(results) > 0 {
		art.Error = fmt.Sprintf("all %d explorations failed (top: %v)", len(results), mostInformativeError(results))
		art.ExitCode = 1
	}

	art.Summary += fmt.Sprintf("\n[exploration: %d/%d succeeded, %d total]",
		successCount, len(p.Steps), len(p.Steps))

	return art, nil
}

// mostInformativeError returns the error string with the largest
// payload — best signal for triage when all explorations failed.
// Stable: returns the first match when multiple errors have equal
// length (matches the input order). Errors with nil .err are skipped.
func mostInformativeError(results []runOut) error {
	var best error
	bestLen := -1
	for _, r := range results {
		if r.err == nil {
			continue
		}
		msgLen := len(r.err.Error())
		if msgLen > bestLen {
			bestLen = msgLen
			best = r.err
		}
	}
	return best
}

// sideEffectForScope maps the Plan's PersistScope to the Artifact's
// SideEffectStatus. Transient probes are no-side-effect; session and
// permanent scopes are committed side effects.
func sideEffectForScope(scope plan.PersistScope) types.SideEffectStatus {
	switch scope {
	case plan.PersistTransient:
		return types.SideEffectNone
	case plan.PersistSession, plan.PersistPermanent:
		return types.SideEffectCommitted
	default:
		// Unknown scope → Unknown status (Phase 4 must verify).
		return types.SideEffectUnknown
	}
}
