// Package errors — redact.go
//
// SanitizeForUser produces a user-safe rendering of an error by redacting
// known sensitive patterns (API keys, bearer tokens, absolute file paths,
// long base64/hex blobs) and truncating to sanitizeMaxLen characters.
//
// Use this at every D1 IM adapter render boundary (feishu cards, CLI
// messages, future adapters) and at D7 emitError source boundary as
// defense-in-depth. See demand.md DM-20260620-003 (AC1 + AC2).
package errors

import (
	"regexp"
	"strings"
)

// sanitizeMaxLen is the maximum length of the sanitized string returned
// to user-facing render paths. After redaction, any content beyond this
// length is truncated with a trailing "...".
const sanitizeMaxLen = 240

// sanitizeRedacted is the placeholder substituted for redacted content.
const sanitizeRedacted = "[REDACTED]"

// sanitizePatterns lists regexes applied in order. Order matters: longer
// patterns (e.g. bearer tokens with prefix) must precede shorter ones
// (e.g. bare hex blobs) to avoid premature match consumption.
var sanitizePatterns = []*regexp.Regexp{
	// Bearer tokens: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.xxx"
	regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._\-]{20,}`),
	// OpenAI/Anthropic/MiniMax style API keys: sk-xxx, sk-ant-xxx, sk-xxx
	regexp.MustCompile(`\bsk-[A-Za-z0-9_\-]{16,}\b`),
	// GitHub tokens: ghp_, gho_, ghu_, ghs_, ghr_
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}\b`),
	// Slack tokens: xoxb-, xoxp-, xoxa-, xoxr-, xoxs-
	regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9\-]{10,}\b`),
	// AWS access keys: AKIA prefix
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
	// Absolute file paths under macOS /Users/ or Linux /home/
	regexp.MustCompile(`/(?:Users|home)/[^\s:'"` + "`" + `]+`),
	// Long hex/base64 blobs (>= 64 chars, no spaces) — likely keys/hashes
	regexp.MustCompile(`\b[A-Za-z0-9+/]{64,}={0,2}\b`),
}

// SanitizeForUser returns a user-safe string from err.Error().
// Redacts known sensitive patterns (API keys, tokens, file paths) and
// truncates to sanitizeMaxLen (240) characters.
//
// Returns "" when err is nil. Idempotent: sanitizing an already-sanitized
// string (containing "[REDACTED]") is a no-op.
//
// The function does NOT walk the wrapped error chain — only the outermost
// message is rendered. Callers that need deeper context should log the
// full chain via slog.Error and pass only the outermost to SanitizeForUser.
func SanitizeForUser(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	for _, re := range sanitizePatterns {
		s = re.ReplaceAllString(s, sanitizeRedacted)
	}
	s = strings.TrimSpace(s)
	if len(s) > sanitizeMaxLen {
		s = s[:sanitizeMaxLen] + "..."
	}
	return s
}
