package execute

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/shared/types"
)

// -----------------------------------------------------------------------------
// ScenarioChannel — parallel probes with majority vote
// -----------------------------------------------------------------------------
//
// Doc 44 §3.4: ScenarioChannel produces an ArtifactProbeReport from
// parallel read-only probes. Steps run concurrently (bounded by
// MaxParallel); results are aggregated via a majority-vote policy:
// success_count > len(Steps)/2 → Success; otherwise Failure.
//
// Side-effect classification:
//   - All probes succeed (or majority does) → SideEffectNone (read-only)
//   - Probe failure (majority rejects)     → SideEffectNone + non-nil error
//   - ctx.DeadlineExceeded                  → SideEffectUnknown

// ScenarioChannelConfig tunes ScenarioChannel's behavior.
type ScenarioChannelConfig struct {
	// Timeout caps the entire parallel-probe batch. Default 10s.
	Timeout time.Duration
	// MaxParallel bounds concurrent probe goroutines. Default 5.
	MaxParallel int
}

// ScenarioChannel is the per-ScenarioPlan execution unit.
type ScenarioChannel struct {
	runner ToolRunner
	cfg    ScenarioChannelConfig
}

// NewScenarioChannel constructs a ScenarioChannel.
func NewScenarioChannel(runner ToolRunner, cfg ScenarioChannelConfig) (*ScenarioChannel, error) {
	if runner == nil {
		return nil, NewChannelToolRunnerNilError("scenario")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.MaxParallel <= 0 {
		cfg.MaxParallel = 5
	}
	return &ScenarioChannel{runner: runner, cfg: cfg}, nil
}

// Name returns the stable channel identifier.
func (c *ScenarioChannel) Name() string { return "scenario" }

// Supports reports whether this Channel handles the given PlanKind.
func (c *ScenarioChannel) Supports(pk plan.PlanKind) bool {
	return pk == plan.ScenarioPlan
}

// Execute runs all probes in parallel, bounded by MaxParallel. Returns an
// ArtifactProbeReport. Aggregates results via majority vote.
//
// Side-effect is None (read-only probes) regardless of outcome — the
// difference is conveyed via ExitCode + Summary + Error rather than
// SideEffectStatus (which only varies for state-changing tools).
func (c *ScenarioChannel) Execute(ctx context.Context, p *plan.Plan, req ChannelRequest) (*wavescheduler.Artifact, error) {
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
		TaskID:           req.SessionID + ":scenario",
		SessionID:        req.SessionID,
		WorkerType:       wavescheduler.WorkerSubAgent, // scenario uses subagent workers
		SourcePlanID:     p.ID,
		AnomaliesCount:   p.AnomaliesCount,
		Kind:             types.ArtifactProbeReport,
		StartedAt:        now,
		SideEffectStatus: types.SideEffectNone, // probes are read-only
	}

	sem := make(chan struct{}, c.cfg.MaxParallel)
	results := make([]ToolResult, len(p.Steps))
	errs := make([]error, len(p.Steps))
	var wg sync.WaitGroup

	for i, step := range p.Steps {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, s plan.Step) {
			defer wg.Done()
			defer func() { <-sem }()
			r, err := c.runner.Invoke(ctx, ToolRequest{
				SessionID:      req.SessionID,
				ToolName:       s.ToolName,
				Args:           s.ToolArgs,
				IdempotencyKey: s.IdempotencyKey,
				StepID:         s.ID,
			})
			results[idx] = r
			errs[idx] = err
		}(i, step)
	}
	wg.Wait()

	art.EndedAt = time.Now()
	art.Duration = art.EndedAt.Sub(art.StartedAt)

	successCount := 0
	for _, e := range errs {
		if e == nil {
			successCount++
		}
	}
	failureCount := len(p.Steps) - successCount

	// Majority vote: success_count > len/2 → success.
	threshold := len(p.Steps) / 2
	passed := successCount > threshold

	art.Summary = fmt.Sprintf("scenario: %d/%d probes succeeded (threshold > %d)",
		successCount, len(p.Steps), threshold)
	for i, r := range results {
		if r.Output != "" {
			art.Summary += fmt.Sprintf("\nprobe_%d: %s", i, r.Output)
		}
	}

	if passed {
		art.ExitCode = 0
		art.SideEffectStatus = types.SideEffectNone
		return art, nil
	}

	art.ExitCode = 1
	art.Error = fmt.Sprintf("scenario_channel: majority failed (%d/%d)", failureCount, len(p.Steps))
	if ctx.Err() == context.DeadlineExceeded {
		art.SideEffectStatus = types.SideEffectUnknown
	}
	return art, fmt.Errorf("%w", ErrChannelStepCountMismatch)
}
