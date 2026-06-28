package errors

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

// T: D3-S1-A01-T04 — NewAPIErrorCodeFromStatus HTTP status mapping coverage.
// Verifies AC1 closed-set mapping table.

func TestNewAPIErrorCodeFromStatus(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   APIErrorCode
	}{
		{"401 → AuthFailed", http.StatusUnauthorized, APICodeAuthenticationFailed},
		{"403 → AuthFailed", http.StatusForbidden, APICodeAuthenticationFailed},
		{"408 → PromptTooLong", http.StatusRequestTimeout, APICodePromptTooLong},
		{"413 → PromptTooLong", http.StatusRequestEntityTooLarge, APICodePromptTooLong},
		{"413 alt (literal) → PromptTooLong", 413, APICodePromptTooLong},
		{"429 → RateLimit", http.StatusTooManyRequests, APICodeRateLimit},
		{"500 → ServerError", http.StatusInternalServerError, APICodeServerError},
		{"502 → ServerError", http.StatusBadGateway, APICodeServerError},
		{"503 → ServerError", http.StatusServiceUnavailable, APICodeServerError},
		{"504 → ServerError", http.StatusGatewayTimeout, APICodeServerError},
		{"529 → ServerError", 529, APICodeServerError},
		{"599 → ServerError", 599, APICodeServerError},
		{"400 → Unknown", http.StatusBadRequest, APICodeUnknown},
		{"404 → Unknown", http.StatusNotFound, APICodeUnknown},
		{"0 → Unknown", 0, APICodeUnknown},
		{"999 → Unknown", 999, APICodeUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewAPIErrorCodeFromStatus(tc.status)
			if got != tc.want {
				t.Errorf("NewAPIErrorCodeFromStatus(%d) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}

// T: D3-S1-A01-T05 — IsCode / Code wrapper-chain identification.

func TestAPIErrorCodeStringRoundTrip(t *testing.T) {
	// All 7 enum values must round-trip through String() + ParseAPIErrorCode().
	codes := []APIErrorCode{
		APICodeUnknown,
		APICodeRateLimit,
		APICodeAuthenticationFailed,
		APICodeServerError,
		APICodeMediaSize,
		APICodePromptTooLong,
		APICodeImageSize,
	}
	for _, c := range codes {
		t.Run(c.String(), func(t *testing.T) {
			got := ParseAPIErrorCode(c.String())
			if got != c {
				t.Errorf("round-trip: %v → %q → %v", c, c.String(), got)
			}
		})
	}
}

func TestParseAPIErrorCodeUnknown(t *testing.T) {
	cases := []struct {
		in   string
		want APIErrorCode
	}{
		{"", APICodeUnknown},
		{"unknown", APICodeUnknown},
		{"UNKNOWN", APICodeUnknown},
		{"  unknown  ", APICodeUnknown},
		{"garbage", APICodeUnknown},
		{"rate", APICodeUnknown}, // not "rate_limit"
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := ParseAPIErrorCode(tc.in); got != tc.want {
				t.Errorf("ParseAPIErrorCode(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestCodeNil(t *testing.T) {
	if got := Code(nil); got != APICodeUnknown {
		t.Errorf("Code(nil) = %v, want APICodeUnknown", got)
	}
	if got := IsCode(nil, APICodeRateLimit); got {
		t.Errorf("IsCode(nil, RateLimit) = true, want false")
	}
}

func TestCodeBareError(t *testing.T) {
	// errors.New without sharederrors wrap → Unknown.
	bare := errors.New("plain error")
	if got := Code(bare); got != APICodeUnknown {
		t.Errorf("Code(bare) = %v, want Unknown", got)
	}
	if IsCode(bare, APICodeServerError) {
		t.Errorf("IsCode(bare, ServerError) = true, want false")
	}
}

func TestCodeWrappedAPIError(t *testing.T) {
	// AC2: WithAPIErrorCode wrap chain identification.
	inner := errors.New("connection reset")
	wrapped := WithAPIErrorCode(APICodeRateLimit, "rate limited", inner)

	if got := Code(wrapped); got != APICodeRateLimit {
		t.Errorf("Code(wrapped) = %v, want APICodeRateLimit", got)
	}
	if !IsCode(wrapped, APICodeRateLimit) {
		t.Errorf("IsCode(wrapped, RateLimit) = false, want true")
	}
	if IsCode(wrapped, APICodeServerError) {
		t.Errorf("IsCode(wrapped, ServerError) = true, want false")
	}

	// Unwrap chain integrity.
	if errors.Unwrap(wrapped) != inner {
		t.Errorf("Unwrap did not return inner error")
	}
}

func TestCodeDeeplyNested(t *testing.T) {
	// 3-level deep wrap: %w chain must still resolve to the API code.
	inner := errors.New("tcp timeout")
	l1 := fmt.Errorf("provider: %w", inner)
	l2 := WithAPIErrorCode(APICodeServerError, "upstream down", l1)

	if got := Code(l2); got != APICodeServerError {
		t.Errorf("Code(3-level wrap) = %v, want ServerError", got)
	}
}

func TestCodeNonAPISentinelError(t *testing.T) {
	// Existing LLMError / COMM_* sentinels must not collide with API codes
	// (they live in distinct namespaces; APIErrorCode is a separate axis).
	llmErr := NewLLMAuthFailedError(ErrLLMAuthFailed)
	if got := Code(llmErr); got != APICodeUnknown {
		t.Errorf("Code(LLMAuthFailed) = %v, want Unknown (different namespace)", got)
	}
	commErr := NewSessionNotFoundError("sess_xxx")
	if got := Code(commErr); got != APICodeUnknown {
		t.Errorf("Code(SessionNotFound) = %v, want Unknown", got)
	}
}

func TestWithAPIErrorCodePreservesLegacyWithCode(t *testing.T) {
	// DM-20260620-003 backward compat: WithCode string API still works.
	legacyErr := WithCode("CUSTOM_9999", "legacy", nil)
	if got := Code(legacyErr); got != APICodeUnknown {
		t.Errorf("Code(legacy CUSTOM_9999) = %v, want Unknown", got)
	}
}
