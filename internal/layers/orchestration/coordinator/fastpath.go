package coordinator

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// FastPath is a zero-allocation proxy that forwards a ProcessRequest to
// TurnExecutor.RunTurn and returns its event stream.
//
// The "≤2ms added latency" target is the P99 of Classify completion → first
// emit of any EngineEvent from the RunTurn channel. The proxy itself
// does not introduce allocations beyond a single channel buffer.
//
// v1.0 metrics: orchestration.fast_path.count (counter).
type FastPath struct {
	cfg      *Config
	executor TurnExecutor
	sink     EventPublisher
}

// NewFastPath builds the proxy. executor is required; sink is optional
// (nil sink → events flow only to the returned channel).
func NewFastPath(cfg *Config, executor TurnExecutor, sink EventPublisher) *FastPath {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &FastPath{cfg: cfg, executor: executor, sink: sink}
}

// Run executes the fast path: build a minimal QueryRequest, invoke D2, and
// return the streaming channel. Events are not mirrored to sink — D1 gateway
// consumes the returned channel as the sole delivery path for this turn.
//
// Run does NOT call ClassifyIntent — the orchestrator does that. The
// orchestrator decides whether FastPath is appropriate; FastPath executes
// the decision.
func (fp *FastPath) Run(ctx context.Context, req ProcessRequest, systemPrompt string) (<-chan *contracts.EngineEvent, error) {
	if fp.executor == nil {
		return nil, fmt.Errorf("orchestrator: FastPath requires a TurnExecutor")
	}
	qreq := QueryRequest{
		SessionID:    req.SessionID,
		SystemPrompt: systemPrompt,
		Messages:     []types.Message{{Role: "user", Content: req.Message}},
		MaxTurns:     8,
	}
	out, err := fp.executor.RunTurn(ctx, qreq)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: FastPath.RunTurn: %w", err)
	}
	return out, nil
}
