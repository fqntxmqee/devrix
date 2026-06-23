package execute

import (
	"context"
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/shared/types"
)

// -----------------------------------------------------------------------------
// ProtocolChannel — sequential multi-Step with rollback on failure
// -----------------------------------------------------------------------------
//
// Doc 44 §3.3: ProtocolChannel produces an ArtifactResponseRecord for
// multi-step deterministic protocols (e.g. login → fetch → parse). Steps
// run in order; if any step fails, previously-completed steps are rolled
// back in reverse order via the ToolRunner's compensator (if available).
//
// Side-effect classification (per step):
//   - step.ExitCode == 0       → step SideEffectCommitted (continues chain)
//   - step failed              → rollback executedSteps in reverse → SideEffectRolledBack
//                                (if any compensation tool is registered)
//
// Note on compensation: PR-C2 ships without a built-in compensation
// registry. Compensation is a PR-C4 ToolSpec v3 concern (CompensationTool
// field). PR-C2 rolls back by re-invoking the same tool with a "rollback"
// hint via ToolArgs["__rollback"] = true (test stub); production will
// replace this with the real compensation registry.

// ProtocolChannelConfig tunes ProtocolChannel's behavior.
type ProtocolChannelConfig struct {
	// Timeout caps the entire multi-step protocol. Default 30s.
	Timeout time.Duration
	// AllowPartialCompletion: if true, return the Artifact with completed
	// steps even if a later step failed (SideEffectInflight). If false
	// (default), rollback is attempted and the Artifact reports
	// SideEffectRolledBack or SideEffectUnknown.
	AllowPartialCompletion bool
}

// ProtocolChannel is the per-ProtocolPlan execution unit.
type ProtocolChannel struct {
	runner ToolRunner
	cfg    ProtocolChannelConfig
}

// NewProtocolChannel constructs a ProtocolChannel.
func NewProtocolChannel(runner ToolRunner, cfg ProtocolChannelConfig) (*ProtocolChannel, error) {
	if runner == nil {
		return nil, NewChannelToolRunnerNilError("protocol")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &ProtocolChannel{runner: runner, cfg: cfg}, nil
}

// Name returns the stable channel identifier.
func (c *ProtocolChannel) Name() string { return "protocol" }

// Supports reports whether this Channel handles the given PlanKind.
func (c *ProtocolChannel) Supports(pk plan.PlanKind) bool {
	return pk == plan.ProtocolPlan
}

// Execute runs Steps in order. On any step failure, executes the rollback
// sequence (in reverse) for already-completed steps before returning.
//
// Returns:
//   - Artifact + nil error: all steps succeeded → SideEffectCommitted
//   - Artifact + error:     rollback ran but error path triggered
//   - nil + error:          pre-flight validation failed (e.g. 0 Steps)
func (c *ProtocolChannel) Execute(ctx context.Context, p *plan.Plan, req ChannelRequest) (*wavescheduler.Artifact, error) {
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
		TaskID:         req.SessionID + ":protocol",
		SessionID:      req.SessionID,
		WorkerType:     wavescheduler.WorkerClaudeCode, // protocol uses claude_code worker
		SourcePlanID:   p.ID,
		AnomaliesCount: p.AnomaliesCount,
		Kind:           types.ArtifactResponseRecord,
		StartedAt:      now,
		SideEffectStatus: types.SideEffectUnknown,
	}

	executedSteps := make([]int, 0, len(p.Steps))
	stepResults := make([]ToolResult, 0, len(p.Steps))
	stepErrs := make([]error, 0, len(p.Steps))

	for i, step := range p.Steps {
		result, err := c.runner.Invoke(ctx, ToolRequest{
			SessionID:      req.SessionID,
			ToolName:       step.ToolName,
			Args:           step.ToolArgs,
			IdempotencyKey: step.IdempotencyKey,
			StepID:         step.ID,
		})
		stepResults = append(stepResults, result)
		stepErrs = append(stepErrs, err)
		if err != nil {
			art.Error = fmt.Sprintf("step %d (%s) failed: %v", i, step.ToolName, err)
			break
		}
		executedSteps = append(executedSteps, i)
	}

	art.EndedAt = time.Now()
	art.Duration = art.EndedAt.Sub(art.StartedAt)

	// Aggregate ExitCode: 0 if all steps succeeded, else the first non-zero.
	art.ExitCode = 0
	for _, e := range stepErrs {
		if e != nil {
			art.ExitCode = 1
			break
		}
	}

	// Build a summary of the multi-step response. We concatenate the
	// outputs with a step separator. This is intentionally simple —
	// real summarization is PR-C7's concern.
	for i, r := range stepResults {
		if r.Output != "" {
			if art.Summary != "" {
				art.Summary += "\n--\n"
			}
			art.Summary += fmt.Sprintf("step_%d: %s", i, r.Output)
		}
	}

	// Failure path: rollback executed steps in reverse.
	if len(executedSteps) < len(p.Steps) {
		c.rollback(ctx, p, executedSteps)
		art.SideEffectStatus = types.SideEffectRolledBack
		return art, fmt.Errorf("protocol_channel: %d/%d steps failed (rolled back)",
			len(p.Steps)-len(executedSteps), len(p.Steps))
	}

	art.SideEffectStatus = types.SideEffectCommitted
	return art, nil
}

// rollback re-invokes the completed steps in reverse order with a
// __rollback hint. PR-C2 uses this minimal scheme; PR-C4 ToolSpec v3
// will add the explicit CompensationTool field.
func (c *ProtocolChannel) rollback(ctx context.Context, p *plan.Plan, executedSteps []int) {
	for i := len(executedSteps) - 1; i >= 0; i-- {
		idx := executedSteps[i]
		step := p.Steps[idx]
		rollbackArgs := make(map[string]any, len(step.ToolArgs)+1)
		for k, v := range step.ToolArgs {
			rollbackArgs[k] = v
		}
		rollbackArgs["__rollback"] = true
		_, _ = c.runner.Invoke(ctx, ToolRequest{
			SessionID:      "",
			ToolName:       step.ToolName,
			Args:           rollbackArgs,
			IdempotencyKey: step.IdempotencyKey + ":rollback",
			StepID:         step.ID + ":rollback",
		})
	}
}
