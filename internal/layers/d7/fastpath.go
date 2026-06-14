package d7

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// FastPath is a zero-allocation proxy that forwards a ProcessRequest to
// D2.RunQueryLoop and returns its event stream.
//
// The "≤2ms added latency" target is the P99 of Classify completion → first
// emit of any EngineEvent from the D2.RunQueryLoop channel. The proxy itself
// does not introduce allocations beyond a single channel buffer.
//
// v1.0 metrics: orchestration.fast_path.count (counter).
type FastPath struct {
	cfg      *Config
	executor D2Executor
	sink     D1EventSink
}

// NewFastPath builds the proxy. executor is required; sink is optional
// (nil sink → events flow only to the returned channel).
func NewFastPath(cfg *Config, executor D2Executor, sink D1EventSink) *FastPath {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &FastPath{cfg: cfg, executor: executor, sink: sink}
}

// Run executes the fast path: build a minimal QueryRequest, invoke D2, and
// return the streaming channel. The D1EventSink, if present, receives a
// mirrored copy of every event.
//
// Run does NOT call ClassifyIntent — the orchestrator does that. The
// orchestrator decides whether FastPath is appropriate; FastPath executes
// the decision.
func (fp *FastPath) Run(ctx context.Context, req ProcessRequest, systemPrompt string) (<-chan *contracts.EngineEvent, error) {
	if fp.executor == nil {
		return nil, fmt.Errorf("d7: FastPath requires a D2Executor")
	}
	qreq := QueryRequest{
		SessionID:    req.SessionID,
		SystemPrompt: systemPrompt,
		Messages:     []types.Message{{Role: "user", Content: req.Message}},
		MaxTurns:     8,
	}
	out, err := fp.executor.RunQueryLoop(ctx, qreq)
	if err != nil {
		return nil, fmt.Errorf("d7: FastPath.RunQueryLoop: %w", err)
	}
	if fp.sink == nil {
		return out, nil
	}
	// Mirror through the sink (best-effort, non-blocking). Per the D7-D1
	// contract, the sink Publish must be safe to call from a goroutine.
	mirrored := make(chan *contracts.EngineEvent, 32)
	go func() {
		defer close(mirrored)
		for ev := range out {
			if ev != nil {
				fp.sink.Publish(ctx, ev)
			}
			select {
			case mirrored <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()
	return mirrored, nil
}
