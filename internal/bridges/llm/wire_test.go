package llmbridge_test

import (
	"errors"
	"testing"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// DSAFT: D3-X-A02-T01 (FailFastOnObsNil, v1.1 F4, R3 P0 #8).
// obs == nil must return ErrObservabilityRequired; no silent fallback to
// mock gateway, no panic, no partial wire.
func TestWireFromConfig_obs_nil_returns_ErrObservabilityRequired(t *testing.T) {
	cfg := sharedconfig.DefaultLLMGatewayConfig()
	got, err := llmbridge.WireFromConfig(cfg, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got != nil {
		t.Errorf("expected nil result on error, got %+v", got)
	}
	if !errors.Is(err, sharederrors.ErrObservabilityRequired) {
		t.Errorf("err = %v, want ErrObservabilityRequired", err)
	}
	if !llmbridge.IsObservabilityRequiredError(err) {
		t.Error("IsObservabilityRequiredError(err) = false, want true")
	}
}

// DSAFT: D3-X-A02-T01 (negative control: obsBridge == nil at the
// top-level context wiring must also fail fast with the same sentinel).
func TestWireContextLLM_obs_nil_returns_ErrObservabilityRequired(t *testing.T) {
	stack, err := llmbridge.WireContextLLM("", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !llmbridge.IsObservabilityRequiredError(err) {
		t.Errorf("err = %v, want ErrObservabilityRequired", err)
	}
	_ = stack // intentionally unused; we only assert on the error path.
}
