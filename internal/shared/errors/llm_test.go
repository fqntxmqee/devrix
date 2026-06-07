package errors_test

import (
	"errors"
	"testing"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

func TestIsRetryable_should_classify_llm_errors(t *testing.T) {
	cases := []struct {
		err      error
		retryable bool
	}{
		{sharederrors.NewLLMTimeoutError(nil), true},
		{sharederrors.NewProviderUnavailableError(nil), true},
		{sharederrors.NewLLMParseError(nil), true},
		{sharederrors.NewCircuitOpenError("deepseek"), false},
		{sharederrors.NewLLMAuthFailedError(nil), false},
		{sharederrors.NewTokenBudgetExceededError(10, 5), false},
	}
	for _, tc := range cases {
		if got := sharederrors.IsRetryable(tc.err); got != tc.retryable {
			t.Errorf("IsRetryable(%v): got %v want %v", tc.err, got, tc.retryable)
		}
	}
}

func TestLLMError_should_wrap_sentinel(t *testing.T) {
	err := sharederrors.NewCircuitOpenError("minimax")
	if !errors.Is(err, sharederrors.ErrCircuitOpen) {
		t.Error("expected ErrCircuitOpen")
	}
}
