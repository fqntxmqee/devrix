package errors

import (
	"errors"
	"net/http"
	"strings"
)

// APIErrorCode is the closed-set enumeration of LLM provider API error categories.
//
// DM-20260628-001 (devrix-api-error-classification) — V4 layer:
// 7 classes aligned with clawcode v2.1.88's categorizeRetryableAPIError
// (src/services/api/errors.ts:1163-1182) + OpenAI/Anthropic HTTP status semantics.
//
// Closed-set invariant: new codes can ONLY be added by appending to the
// const block below. The (iota) integer values are stable and form part of
// the public contract (consumed by D7 emitError → Event.Metadata and by
// D1 IM adapters for differentiated copy).
type APIErrorCode int

const (
	// APICodeUnknown is the zero value, JSON-friendly default.
	APICodeUnknown APIErrorCode = iota
	// APICodeRateLimit — HTTP 429 / provider-specific rate limit responses.
	APICodeRateLimit
	// APICodeAuthenticationFailed — HTTP 401 / 403 (invalid API key, expired token).
	APICodeAuthenticationFailed
	// APICodeServerError — HTTP 5xx (500/502/503/504/529).
	APICodeServerError
	// APICodeMediaSize — Anthropic media_too_large / OpenAI file too large.
	APICodeMediaSize
	// APICodePromptTooLong — HTTP 408 / 413 (request timeout / payload too large).
	APICodePromptTooLong
	// APICodeImageSize — Image-specific size limit (Anthropic image_too_large).
	APICodeImageSize
)

// apiCodeNames is the canonical String() lookup. Indexed by APIErrorCode value.
// Keep in sync with the const block above.
var apiCodeNames = [...]string{
	APICodeUnknown:             "unknown",
	APICodeRateLimit:           "rate_limit",
	APICodeAuthenticationFailed: "authentication_failed",
	APICodeServerError:          "server_error",
	APICodeMediaSize:            "media_size",
	APICodePromptTooLong:        "prompt_too_long",
	APICodeImageSize:            "image_size",
}

// String returns the canonical lowercase name (e.g. "rate_limit").
// Used for log, Event.Metadata, and IM adapter switch dispatch.
func (c APIErrorCode) String() string {
	if c < 0 || int(c) >= len(apiCodeNames) {
		return apiCodeNames[APICodeUnknown]
	}
	return apiCodeNames[c]
}

// ParseAPIErrorCode is the inverse of String. Unknown values map to APICodeUnknown.
// Case-insensitive matching for IM adapter resilience.
func ParseAPIErrorCode(s string) APIErrorCode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "unknown":
		return APICodeUnknown
	case "rate_limit":
		return APICodeRateLimit
	case "authentication_failed":
		return APICodeAuthenticationFailed
	case "server_error":
		return APICodeServerError
	case "media_size":
		return APICodeMediaSize
	case "prompt_too_long":
		return APICodePromptTooLong
	case "image_size":
		return APICodeImageSize
	default:
		return APICodeUnknown
	}
}

// NewAPIErrorCodeFromStatus maps an HTTP status code to the closed-set APIErrorCode.
//
// Mapping table (DM-20260628-001 §AC1):
//
//	401, 403                          → APICodeAuthenticationFailed
//	408, 413                          → APICodePromptTooLong
//	429                               → APICodeRateLimit
//	500, 502, 503, 504, 529, 5xx      → APICodeServerError
//	anything else (incl. 4xx unknown)  → APICodeUnknown
//
// Provider-specific errors (Anthropic media_too_large etc.) are mapped by
// the adapter layer (not by this function) because they do not correspond
// to a single HTTP status code.
func NewAPIErrorCodeFromStatus(status int) APIErrorCode {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return APICodeAuthenticationFailed
	case status == http.StatusRequestTimeout || status == http.StatusRequestEntityTooLarge:
		return APICodePromptTooLong
	case status == http.StatusTooManyRequests:
		return APICodeRateLimit
	case status >= 500 && status <= 599:
		return APICodeServerError
	default:
		return APICodeUnknown
	}
}

// apiCodePrefix is the prefix used when wrapping an error via WithCode/WithAPIErrorCode.
// Kept distinct from existing CodeLLM* / CodeCOMM* / CodeMULTI* namespaces so the
// sharederrors.ErrorCode(err) string column does not collide with sentinel codes.
const apiCodePrefix = "API_"

// codeToString returns the prefixed string used inside SentinelError.Code.
// Inverse of apiCodeFromString.
func codeToString(c APIErrorCode) string {
	return apiCodePrefix + c.String()
}

// apiCodeFromString reverses codeToString; returns (code, true) on hit.
// Returns (APICodeUnknown, false) if the string is not a recognized API code.
func apiCodeFromString(s string) (APIErrorCode, bool) {
	if !strings.HasPrefix(s, apiCodePrefix) {
		return APICodeUnknown, false
	}
	name := strings.TrimPrefix(s, apiCodePrefix)
	parsed := ParseAPIErrorCode(name)
	if parsed == APICodeUnknown && name != "" && name != "unknown" {
		return APICodeUnknown, false
	}
	return parsed, true
}

// WithAPIErrorCode wraps cause in a SentinelError carrying the closed-set APIErrorCode.
// Mirrors WithCode but with typed code parameter. Existing WithCode string API
// is preserved (DM-20260620-003 backward compat).
//
// The returned error chain unwraps to cause; SentinelError.Code carries
// "API_<code.String()>" so sharederrors.ErrorCode(err) does not collide
// with the LLM_/COMM_/MULTI_ sentinel namespaces.
func WithAPIErrorCode(code APIErrorCode, msg string, cause error) error {
	return WithCode(codeToString(code), msg, cause)
}

// APICodeProvider is an optional interface implemented by error types that
// carry an APIErrorCode directly (e.g. llmgateway.APIError).
//
// sharederrors.Code() walks the error chain looking for an APICodeProvider
// implementation; this allows downstream packages to surface their typed
// code without forcing sharederrors to import them (which would create
// import cycles).
type APICodeProvider interface {
	APICode() APIErrorCode
}

// Code extracts the APIErrorCode from an error chain.
//
// Resolution order:
//  1. Walk the chain; first error implementing APICodeProvider wins.
//  2. Else walk the chain; first SentinelError whose Code is in the API_
//     namespace (set by WithAPIErrorCode) wins.
//  3. Else APICodeUnknown.
//
// O(chain depth); safe for nil; thread-safe (no mutation).
func Code(err error) APIErrorCode {
	if err == nil {
		return APICodeUnknown
	}
	for cur := err; cur != nil; cur = errors.Unwrap(cur) {
		if p, ok := cur.(APICodeProvider); ok {
			return p.APICode()
		}
	}
	var se *SentinelError
	for cur := err; cur != nil; cur = errors.Unwrap(cur) {
		if errors.As(cur, &se) {
			if code, ok := apiCodeFromString(se.Code); ok {
				return code
			}
		}
	}
	return APICodeUnknown
}

// IsCode reports whether err or any error in its chain has the given APIErrorCode.
//
// O(chain depth); safe for nil; thread-safe.
func IsCode(err error, code APIErrorCode) bool {
	return Code(err) == code
}
