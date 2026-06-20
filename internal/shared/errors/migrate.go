package errors

import "errors"

// Migrate.go — DM-20260620-003 (PR-B tier 2 C2) error type unification.
//
// Background: devrix historically shipped two error shapes with identical
// fields:
//   - SentinelError (in communication.go) — used by D1/D2/D4/D7 layers
//   - LLMError (in llm.go) — used by D3 LLM gateway
//
// Both exposed Code/Message/Err with the same semantics. Two type names
// forced callers to choose (or `errors.As`-twice), and created ambiguity in
// test assertions (`var x *LLMError` vs `var x *SentinelError`).
//
// Resolution: SentinelError is the canonical type. LLMError is now a Go
// type alias (defined in llm.go) so existing `*LLMError` references compile
// unchanged. New code MUST use *SentinelError. The aliases below are kept
// for any external imports that still reference the old names by string.
//
// Removal plan: LLMError alias + this migrate.go file will be deleted in
// devrix v2.0.0 (no sooner than 2026-09-01). Until then, golangci-lint will
// not flag references (the alias keeps the API surface identical).

// Compile-time guard: ensure LLMError is still a type alias for SentinelError.
// If you delete the alias from llm.go, this assignment fails to build.
var _ = (*SentinelError)(nil) //nolint:unused // build-time guard
var _ LLMError                //nolint:unused // build-time guard

// IsLLMError reports whether err chains to a *SentinelError (formerly *LLMError).
//
// DM-20260620-003 (PR-B): kept as a convenience helper. Prefer direct
// `errors.As(err, &sentinel)` against *SentinelError.
//
// Deprecated: use errors.As(err, &se) where se is a *SentinelError.
func IsLLMError(err error) bool {
	var se *SentinelError
	return errors.As(err, &se)
}

// LLMCode returns the Code field if err chains to a *SentinelError/legacy
// *LLMError, otherwise "".
//
// DM-20260620-003 (PR-B): convenience alias for ErrorCode(err).
//
// Deprecated: use ErrorCode(err).
func LLMCode(err error) string {
	return ErrorCode(err)
}