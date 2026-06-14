package llmbridge

import (
	"errors"
	"fmt"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/configure"
	"github.com/devrix/devrix/internal/layers/llmgateway/stream"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/shared/contracts"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// WireResult holds the wired LLM stack.
type WireResult struct {
	Gateway      *stream.Gateway
	Bridge       llmgateway.ILLMGateway
	TokenCounter contracts.ITokenCounter
}

// WireFromConfig builds gateway + L2 bridge from configuration.
//
// DSAFT: D3-X-A02-F02 FailFastOnObsNil (v1.1 F4, R3 P0 #8).
// A nil observability bridge is a programmer error: every caller must
// construct one (typically via observability.New + observability.NewBridge)
// before wiring the LLM stack. We fail fast with ErrObservabilityRequired
// rather than degrading silently to a no-op telemetry path.
func WireFromConfig(cfg *configure.LLMGatewayConfig, obs *observability.Bridge) (*WireResult, error) {
	if obs == nil {
		return nil, sharederrors.ErrObservabilityRequired
	}
	gw, err := stream.NewFromConfig(cfg, obs)
	if err != nil {
		return nil, fmt.Errorf("llm gateway: %w", err)
	}
	return &WireResult{
		Gateway:      gw,
		Bridge:       New(gw),
		TokenCounter: gw.TokenCounter(),
	}, nil
}

// IsObservabilityRequiredError reports whether err is the v1.1 F4
// fail-fast sentinel, so callers can map it to a clean startup error
// without leaking the internal sentinel via errors.Is duplication.
func IsObservabilityRequiredError(err error) bool {
	return errors.Is(err, sharederrors.ErrObservabilityRequired)
}
