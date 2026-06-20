package errors

import (
	"fmt"
	"strings"
	"testing"
)

// T: SHARED-ERRORS-A01-T04 (DM-20260620-003 AC9) — SanitizeForUser unit tests
// covering redacted patterns, truncation, edge cases, and idempotency.

func TestSanitizeForUser_NilReturnsEmpty(t *testing.T) {
	if got := SanitizeForUser(nil); got != "" {
		t.Errorf("SanitizeForUser(nil) = %q, want empty", got)
	}
}

func TestSanitizeForUser_BearerTokenRedacted(t *testing.T) {
	err := fmt.Errorf("auth failed: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.xxxxxxx")
	got := SanitizeForUser(err)
	if strings.Contains(got, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9") {
		t.Errorf("SanitizeForUser did not redact Bearer token: %q", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Errorf("SanitizeForUser did not insert [REDACTED] marker: %q", got)
	}
	// "Bearer <token>" is fully redacted to "[REDACTED]" — the prefix
	// "Bearer" is part of the redaction so the user cannot infer that a
	// bearer token was used.
	if !strings.HasPrefix(got, "auth failed: [REDACTED]") {
		t.Errorf("SanitizeForUser unexpected prefix: %q", got)
	}
}

func TestSanitizeForUser_SkKeyRedacted(t *testing.T) {
	err := fmt.Errorf("LLM returned status 401, key=sk-abc123def456ghi789jkl012mno")
	got := SanitizeForUser(err)
	if strings.Contains(got, "sk-abc123def456ghi789jkl012mno") {
		t.Errorf("SanitizeForUser did not redact sk- key: %q", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Errorf("SanitizeForUser did not insert [REDACTED] marker: %q", got)
	}
}

func TestSanitizeForUser_GitHubTokenRedacted(t *testing.T) {
	err := fmt.Errorf("git auth failed with token ghp_abc123def456ghi789jkl0123456789")
	got := SanitizeForUser(err)
	if strings.Contains(got, "ghp_abc123def456ghi789jkl0123456789") {
		t.Errorf("SanitizeForUser did not redact GitHub token: %q", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Errorf("SanitizeForUser did not insert [REDACTED] marker: %q", got)
	}
}

func TestSanitizeForUser_AWSAccessKeyRedacted(t *testing.T) {
	err := fmt.Errorf("aws access denied: AKIAIOSFODNN7EXAMPLE")
	got := SanitizeForUser(err)
	if strings.Contains(got, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("SanitizeForUser did not redact AWS key: %q", got)
	}
}

func TestSanitizeForUser_FilePathRedacted(t *testing.T) {
	err := fmt.Errorf("tool round failed: read /Users/fukai/.ssh/id_rsa: no such file")
	got := SanitizeForUser(err)
	if strings.Contains(got, "/Users/fukai/.ssh/id_rsa") {
		t.Errorf("SanitizeForUser did not redact /Users/ path: %q", got)
	}
	if !strings.Contains(got, "read [REDACTED]: no such file") {
		t.Errorf("SanitizeForUser did not preserve path context: %q", got)
	}
}

func TestSanitizeForUser_LinuxHomePathRedacted(t *testing.T) {
	err := fmt.Errorf("config not found at /home/admin/.config/devrix.yaml")
	got := SanitizeForUser(err)
	if strings.Contains(got, "/home/admin/.config/devrix.yaml") {
		t.Errorf("SanitizeForUser did not redact /home/ path: %q", got)
	}
}

func TestSanitizeForUser_TruncatesAt240(t *testing.T) {
	// Use space-separated words so the long-base64 regex doesn't match
	// (the regex requires no spaces) — we want to exercise the truncation
	// path, not the redaction path.
	long := strings.Repeat("lorem ipsum dolor sit amet ", 100)
	err := fmt.Errorf("huge: %s", long)
	got := SanitizeForUser(err)
	if len(got) > sanitizeMaxLen+3 { // 240 + "..."
		t.Errorf("SanitizeForUser result too long: %d chars (max %d+3)", len(got), sanitizeMaxLen)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("SanitizeForUser truncated result missing '...' suffix: %q", got)
	}
}

func TestSanitizeForUser_AlreadySanitizedIdempotent(t *testing.T) {
	err := fmt.Errorf("tool round failed: [REDACTED]: no such file")
	got := SanitizeForUser(err)
	if got != "tool round failed: [REDACTED]: no such file" {
		t.Errorf("SanitizeForUser not idempotent: %q", got)
	}
	// [REDACTED] is < 64 chars so the base64 regex doesn't match it.
	count := strings.Count(got, "[REDACTED]")
	if count != 1 {
		t.Errorf("SanitizeForUser changed [REDACTED] count: got %d, want 1 (in %q)", count, got)
	}
}

func TestSanitizeForUser_PreservesNormalEnglish(t *testing.T) {
	err := fmt.Errorf("tool round failed: read_file returned no such file or directory")
	got := SanitizeForUser(err)
	if strings.Contains(got, "[REDACTED]") {
		t.Errorf("SanitizeForUser over-redacted normal English: %q", got)
	}
	if got != "tool round failed: read_file returned no such file or directory" {
		t.Errorf("SanitizeForUser altered normal English: %q", got)
	}
}

func TestSanitizeForUser_TrimsLeadingTrailingSpace(t *testing.T) {
	err := fmt.Errorf("   auth failed: sk-abc123def456ghi789jkl012mno   ")
	got := SanitizeForUser(err)
	if strings.HasPrefix(got, " ") || strings.HasSuffix(got, " ") {
		t.Errorf("SanitizeForUser did not trim whitespace: %q", got)
	}
}

func TestSanitizeForUser_EmptyMessage(t *testing.T) {
	// errors.New("") is unusual but should not crash.
	err := fmt.Errorf("")
	got := SanitizeForUser(err)
	if got != "" {
		t.Errorf("SanitizeForUser(empty) = %q, want empty", got)
	}
}

func TestSanitizeForUser_LongBase64BlobRedacted(t *testing.T) {
	// 80-char base64-like blob (no spaces) — should be redacted by the
	// long-blob regex pattern.
	blob := strings.Repeat("A", 80)
	err := fmt.Errorf("decoding failed: %s invalid", blob)
	got := SanitizeForUser(err)
	if strings.Contains(got, blob) {
		t.Errorf("SanitizeForUser did not redact long base64 blob: %q", got)
	}
}

func TestSanitizeForUser_PreservesShortIdentifiers(t *testing.T) {
	// Short tokens (< 20 chars) should NOT be redacted.
	err := fmt.Errorf("variable name=foo, key=sk-short (only 8 chars)")
	got := SanitizeForUser(err)
	if !strings.Contains(got, "foo") {
		t.Errorf("SanitizeForUser over-redacted short identifier: %q", got)
	}
	// "sk-short" is 8 chars, well below the 16-char threshold, so it
	// should survive the sk- pattern.
	if !strings.Contains(got, "sk-short") {
		t.Errorf("SanitizeForUser over-redacted short sk-: %q", got)
	}
}
