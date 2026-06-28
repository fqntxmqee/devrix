package llmgateway

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// T: D3-S3-A01-T17 — llmgateway.APIError struct + NewAPIError factory.

func TestNewAPIErrorAutoMapping(t *testing.T) {
	cases := []struct {
		status int
		want   sharederrors.APIErrorCode
	}{
		{http.StatusUnauthorized, sharederrors.APICodeAuthenticationFailed},
		{http.StatusForbidden, sharederrors.APICodeAuthenticationFailed},
		{http.StatusTooManyRequests, sharederrors.APICodeRateLimit},
		{http.StatusRequestEntityTooLarge, sharederrors.APICodePromptTooLong},
		{http.StatusInternalServerError, sharederrors.APICodeServerError},
		{http.StatusBadGateway, sharederrors.APICodeServerError},
		{529, sharederrors.APICodeServerError},
		{http.StatusBadRequest, sharederrors.APICodeUnknown},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			got := NewAPIError(tc.status, "msg")
			if got.Status != tc.status {
				t.Errorf("Status = %d, want %d", got.Status, tc.status)
			}
			if got.Code != tc.want {
				t.Errorf("Code = %v, want %v", got.Code, tc.want)
			}
			if got.Message != "msg" {
				t.Errorf("Message = %q, want %q", got.Message, "msg")
			}
		})
	}
}

func TestAPIErrorErrorPriority(t *testing.T) {
	t.Run("Message wins", func(t *testing.T) {
		e := &APIError{Status: 500, Message: "boom", Code: sharederrors.APICodeServerError, Cause: errors.New("underlying")}
		if got := e.Error(); got != "boom" {
			t.Errorf("Error() = %q, want %q", got, "boom")
		}
	})
	t.Run("Cause when Message empty", func(t *testing.T) {
		e := &APIError{Status: 500, Cause: errors.New("underlying")}
		if got := e.Error(); got != "underlying" {
			t.Errorf("Error() = %q, want %q", got, "underlying")
		}
	})
	t.Run("Code when both empty", func(t *testing.T) {
		e := &APIError{Status: 500, Code: sharederrors.APICodeServerError}
		if got := e.Error(); got != "server_error" {
			t.Errorf("Error() = %q, want %q", got, "server_error")
		}
	})
}

func TestAPIErrorUnwrap(t *testing.T) {
	cause := errors.New("provider tcp reset")
	e := NewAPIErrorWithCause(500, "upstream down", cause)
	if errors.Unwrap(e) != cause {
		t.Errorf("Unwrap did not return Cause")
	}
	// errors.Is must traverse to the cause.
	if !errors.Is(e, cause) {
		t.Errorf("errors.Is(e, cause) = false, want true")
	}
}

func TestIsAPICodeWrapChain(t *testing.T) {
	inner := NewAPIErrorWithCause(429, "rate limited", errors.New("rate"))
	wrapped := fmt.Errorf("provider call failed: %w", inner)

	if !IsAPICode(wrapped, sharederrors.APICodeRateLimit) {
		t.Errorf("IsAPICode(wrapped, RateLimit) = false, want true")
	}
	if IsAPICode(wrapped, sharederrors.APICodeServerError) {
		t.Errorf("IsAPICode(wrapped, ServerError) = true, want false")
	}
	if IsAPICode(nil, sharederrors.APICodeRateLimit) {
		t.Errorf("IsAPICode(nil, RateLimit) = true, want false")
	}
}

func TestSharederrorsCodeEndToEnd(t *testing.T) {
	// AC7: End-to-end code propagation APIError + WithAPIErrorCode → sharederrors.Code.
	apiErr := NewAPIError(429, "rate limited")
	wrapped := sharederrors.WithAPIErrorCode(sharederrors.APICodeRateLimit, "rate limited", apiErr)
	if got := sharederrors.Code(wrapped); got != sharederrors.APICodeRateLimit {
		t.Errorf("sharederrors.Code(wrapped) = %v, want RateLimit", got)
	}
	if !sharederrors.IsCode(wrapped, sharederrors.APICodeRateLimit) {
		t.Errorf("IsCode(wrapped, RateLimit) = false, want true")
	}
}
