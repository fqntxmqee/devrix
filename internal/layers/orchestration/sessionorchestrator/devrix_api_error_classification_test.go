package sessionorchestrator

import (
	"errors"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/contracts"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// T: D7-S2-A50-T05 + D7-S2-A50-T06 — emitErrorWithErr propagation + fallback trigger.
//
// DM-20260628-001 (devrix-api-error-classification) regression tests.

func TestEmitErrorWithErr_APIErrorCodePropagation(t *testing.T) {
	o := NewOrchestrator(OrchestratorDeps{
		LLM:     &stubLLM{},
		Context: &stubContext{},
		Tools:   &stubTools{},
		Persist: &stubPersist{},
	})
	out := make(chan *contracts.EngineEvent, 1)

	apiErr := sharederrors.NewLLMAuthFailedError(
		llmgateway.NewAPIErrorWithCause(401, "auth failed", sharederrors.ErrLLMAuthFailed))
	o.emitErrorWithErr(out, "sess_1", "boom", apiErr)

	ev := <-out
	if got := ev.Metadata["error_code"]; got != "authentication_failed" {
		t.Errorf("error_code = %q, want authentication_failed (LLMAuthFailed → APICodeAuthenticationFailed)", got)
	}
}

func TestEmitErrorWithErr_BareErrorUnknown(t *testing.T) {
	o := NewOrchestrator(OrchestratorDeps{
		LLM:     &stubLLM{},
		Context: &stubContext{},
		Tools:   &stubTools{},
		Persist: &stubPersist{},
	})
	out := make(chan *contracts.EngineEvent, 1)

	o.emitErrorWithErr(out, "sess_1", "boom", errors.New("plain"))
	ev := <-out
	if got := ev.Metadata["error_code"]; got != "unknown" {
		t.Errorf("error_code = %q, want unknown", got)
	}
}

func TestObserveFallbackTrigger_TwoConsecutiveRateLimit(t *testing.T) {
	o := NewOrchestrator(OrchestratorDeps{
		LLM:     &stubLLM{},
		Context: &stubContext{},
		Tools:   &stubTools{},
		Persist: &stubPersist{},
	})
	rateLimitErr := sharederrors.NewProviderUnavailableError(
		llmgateway.NewAPIErrorWithCause(429, "rate limited", nil))

	o.observeFallbackTrigger(rateLimitErr)
	if o.consecutiveServerErrors != 1 {
		t.Errorf("after 1st RateLimit: consecutive = %d, want 1", o.consecutiveServerErrors)
	}
	o.observeFallbackTrigger(rateLimitErr)
	if o.consecutiveServerErrors != 2 {
		t.Errorf("after 2nd RateLimit: consecutive = %d, want 2", o.consecutiveServerErrors)
	}
}

func TestObserveFallbackTrigger_ResetOnNonRetryable(t *testing.T) {
	o := NewOrchestrator(OrchestratorDeps{
		LLM:     &stubLLM{},
		Context: &stubContext{},
		Tools:   &stubTools{},
		Persist: &stubPersist{},
	})
	rateLimitErr := sharederrors.NewProviderUnavailableError(
		llmgateway.NewAPIErrorWithCause(429, "rate limited", nil))
	authErr := sharederrors.NewLLMAuthFailedError(
		llmgateway.NewAPIErrorWithCause(401, "auth failed", sharederrors.ErrLLMAuthFailed))

	o.observeFallbackTrigger(rateLimitErr) // counter → 1
	o.observeFallbackTrigger(rateLimitErr) // counter → 2
	o.observeFallbackTrigger(authErr)      // counter → 0 (non-retryable)
	if o.consecutiveServerErrors != 0 {
		t.Errorf("after AuthFailed: consecutive = %d, want 0 (reset)", o.consecutiveServerErrors)
	}
}

func TestObserveFallbackTrigger_FallbackModelSet(t *testing.T) {
	o := NewOrchestrator(OrchestratorDeps{
		LLM:           &stubLLM{},
		Context:       &stubContext{},
		Tools:         &stubTools{},
		Persist:       &stubPersist{},
		FallbackModel: "claude-haiku-4",
	})
	if o.fallbackModel == "" {
		t.Fatal("precondition: FallbackModel set")
	}
	rateLimitErr := sharederrors.NewProviderUnavailableError(
		llmgateway.NewAPIErrorWithCause(429, "rate limited", nil))
	o.observeFallbackTrigger(rateLimitErr)
	o.observeFallbackTrigger(rateLimitErr)
	if o.consecutiveServerErrors != 2 {
		t.Errorf("consecutive = %d, want 2", o.consecutiveServerErrors)
	}
}
