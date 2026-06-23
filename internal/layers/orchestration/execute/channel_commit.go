package execute

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/shared/types"
)

// -----------------------------------------------------------------------------
// CommitChannel — synchronous 1-Step direct tool call
// -----------------------------------------------------------------------------
//
// Doc 44 §3.2: CommitChannel produces an ArtifactStateChangeCert that
// represents a real, observable side effect (DB write / HTTP POST /
// file create). The Channel requires exactly 1 Step (PP-3 blast-radius
// guard) and runs it synchronously with a short timeout.
//
// Side-effect classification:
//   - exitCode == 0       → SideEffectCommitted
//   - ctx.DeadlineExceeded → SideEffectInflight (compensate on retry)
//   - non-zero exitCode   → SideEffectRolledBack (if compensable) or Unknown
//
// IdempotencyKey is REQUIRED for any side-effecting tool — the Channel
// enforces this and returns ErrChannelStepCountMismatch if missing.
// (Full IdempotencyCache enforcement is PR-C7.)

// CommitChannelConfig tunes CommitChannel's behavior.
type CommitChannelConfig struct {
	// Timeout caps the tool call duration. Default 5s.
	Timeout time.Duration
}

// CommitChannel is the per-CommitmentPlan execution unit.
type CommitChannel struct {
	runner ToolRunner
	cfg    CommitChannelConfig
}

// NewCommitChannel constructs a CommitChannel. Returns
// NewChannelToolRunnerNilError if runner is nil.
func NewCommitChannel(runner ToolRunner, cfg CommitChannelConfig) (*CommitChannel, error) {
	if runner == nil {
		return nil, NewChannelToolRunnerNilError("commit")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	return &CommitChannel{runner: runner, cfg: cfg}, nil
}

// Name returns the stable channel identifier.
func (c *CommitChannel) Name() string { return "commit" }

// Supports reports whether this Channel handles the given PlanKind.
func (c *CommitChannel) Supports(pk plan.PlanKind) bool {
	return pk == plan.CommitmentPlan
}

// Execute runs the single Step and produces a StateChangeCert artifact.
//
// Pre-conditions:
//   - p.Kind == plan.CommitmentPlan (caller / Router enforces via Supports)
//   - len(p.Steps) == 1 (Channel-level invariant, enforced here)
//   - step.ToolName != ""
//   - step.IdempotencyKey != "" (PR-C2 AC: side-effecting tools must have it)
//
// On success returns an Artifact with SideEffectCommitted.
// On timeout returns the partial Artifact with SideEffectInflight so the
// StrategyDecider (PR-C3) can route to StrategyAskNow.
func (c *CommitChannel) Execute(ctx context.Context, p *plan.Plan, req ChannelRequest) (*wavescheduler.Artifact, error) {
	if p == nil {
		return nil, fmt.Errorf("%w", ErrChannelPlanNil)
	}
	if len(p.Steps) != 1 {
		return nil, NewChannelStepCountMismatchError(c.Name(), len(p.Steps), 1, 1)
	}
	step := p.Steps[0]
	if step.ToolName == "" {
		return nil, fmt.Errorf("%w: commit step has empty ToolName", ErrChannelStepCountMismatch)
	}
	if step.IdempotencyKey == "" {
		// Side-effecting tool without idempotency key — fail-fast.
		// PR-C7 will move this to the IdempotencyCache check, but PR-C2
		// already enforces the contract at the Channel level.
		return nil, fmt.Errorf("%w: commit step (tool=%s) requires IdempotencyKey", ErrChannelStepCountMismatch, step.ToolName)
	}

	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	result, err := c.runner.Invoke(ctx, ToolRequest{
		SessionID:      req.SessionID,
		ToolName:       step.ToolName,
		Args:           step.ToolArgs,
		IdempotencyKey: step.IdempotencyKey,
		StepID:         step.ID,
	})

	now := time.Now()
	art := &wavescheduler.Artifact{
		TaskID:         step.ID,
		SessionID:      req.SessionID,
		WorkerType:     wavescheduler.WorkerCursor, // commit uses the synchronous cursor worker
		SourcePlanID:   p.ID,
		AnomaliesCount: p.AnomaliesCount,
		Kind:           types.ArtifactStateChangeCert,
		StartedAt:      now,
		EndedAt:        now,
	}

	if err != nil {
		// Distinguish timeout from other errors so the StrategyDecider can
		// route to StrategyAskNow on inflight side effects. Two cases:
		//   - ctx already expired → outer deadline blew
		//   - runner returned context.DeadlineExceeded → inner deadline blew
		// Both indicate an inflight side effect (tool may or may not have
		// completed) so we conservatively mark SideEffectInflight.
		if ctx.Err() == context.DeadlineExceeded || errors.Is(err, context.DeadlineExceeded) {
			art.SideEffectStatus = types.SideEffectInflight
			art.Error = "commit_channel: tool call timed out (side-effect status uncertain)"
			art.SideEffectDetail = &types.SideEffectDetail{
				IdempotencyKey: step.IdempotencyKey,
				SentAt:         result.StartedAt.UnixNano(),
			}
			// Return the artifact (with inflight marker) + error so the
			// caller can decide whether to retry / compensate.
			return art, fmt.Errorf("%w: commit tool call timed out", ErrChannelStepCountMismatch)
		}
		art.SideEffectStatus = types.SideEffectUnknown
		art.Error = err.Error()
		return art, err
	}

	art.Summary = result.Output
	art.ExitCode = result.ExitCode
	art.Duration = result.CompletedAt.Sub(result.StartedAt)
	art.SideEffectStatus = types.SideEffectCommitted
	art.SideEffectDetail = &types.SideEffectDetail{
		IdempotencyKey: step.IdempotencyKey,
		SentAt:         result.StartedAt.UnixNano(),
		ConfirmedAt:    result.CompletedAt.UnixNano(),
	}
	return art, nil
}
