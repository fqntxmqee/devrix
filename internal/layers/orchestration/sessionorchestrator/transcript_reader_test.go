package sessionorchestrator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/capture/transcript"
)

// TestTranscriptReader_ReadRecent_EmptyFile covers a fresh session (no
// transcript yet) → empty slice, no error.
func TestTranscriptReader_ReadRecent_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	r := NewTranscriptReader(dir)
	got, err := r.ReadRecent(context.Background(), "sess_fresh", 3)
	if err != nil {
		t.Fatalf("ReadRecent: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ReadRecent on empty file = %v, want empty", got)
	}
}

// TestTranscriptReader_ReadRecent_NoCompletions covers a session that has
// events but none of Kind=complete (e.g., a session that errored before
// completing).
func TestTranscriptReader_ReadRecent_NoCompletions(t *testing.T) {
	dir := t.TempDir()
	writeTranscript(t, dir, "sess_no_complete", []transcript.Event{
		{Time: time.Unix(1, 0), Kind: "user", Body: "hi"},
		{Time: time.Unix(2, 0), Kind: "assistant", Body: "thinking..."},
		{Time: time.Unix(3, 0), Kind: "tool_call", Body: "bash"},
	})
	r := NewTranscriptReader(dir)
	got, err := r.ReadRecent(context.Background(), "sess_no_complete", 3)
	if err != nil {
		t.Fatalf("ReadRecent: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ReadRecent = %v, want empty (no complete events)", got)
	}
}

// TestTranscriptReader_ReadRecent_SingleComplete covers the basic case:
// one complete event, return its Body.
func TestTranscriptReader_ReadRecent_SingleComplete(t *testing.T) {
	dir := t.TempDir()
	writeTranscript(t, dir, "sess_one", []transcript.Event{
		{Time: time.Unix(1, 0), Kind: "user", Body: "hello"},
		{Time: time.Unix(2, 0), Kind: "complete", Body: "world"},
	})
	r := NewTranscriptReader(dir)
	got, err := r.ReadRecent(context.Background(), "sess_one", 3)
	if err != nil {
		t.Fatalf("ReadRecent: %v", err)
	}
	if len(got) != 1 || got[0] != "world" {
		t.Errorf("ReadRecent = %v, want [world]", got)
	}
}

// TestTranscriptReader_ReadRecent_TakesLastN covers the N-cap behavior:
// 5 complete events in chronological order, n=3 → last 3 in order.
func TestTranscriptReader_ReadRecent_TakesLastN(t *testing.T) {
	dir := t.TempDir()
	events := make([]transcript.Event, 0, 5)
	for i := 1; i <= 5; i++ {
		events = append(events, transcript.Event{
			Time: time.Unix(int64(i), 0),
			Kind: "complete",
			Body: "turn-" + string(rune('0'+i)),
		})
	}
	writeTranscript(t, dir, "sess_five", events)
	r := NewTranscriptReader(dir)
	got, err := r.ReadRecent(context.Background(), "sess_five", 3)
	if err != nil {
		t.Fatalf("ReadRecent: %v", err)
	}
	want := []string{"turn-3", "turn-4", "turn-5"}
	if !equalStrings(got, want) {
		t.Errorf("ReadRecent = %v, want %v", got, want)
	}
}

// TestTranscriptReader_ReadRecent_SkipsEmptyBody covers the case where a
// complete event has empty Body (defensive: writer doesn't currently emit
// empty Bodies but the schema allows it; future events might).
func TestTranscriptReader_ReadRecent_SkipsEmptyBody(t *testing.T) {
	dir := t.TempDir()
	writeTranscript(t, dir, "sess_empty_body", []transcript.Event{
		{Time: time.Unix(1, 0), Kind: "complete", Body: ""}, // empty → skip
		{Time: time.Unix(2, 0), Kind: "complete", Body: "kept"},
	})
	r := NewTranscriptReader(dir)
	got, err := r.ReadRecent(context.Background(), "sess_empty_body", 5)
	if err != nil {
		t.Fatalf("ReadRecent: %v", err)
	}
	if len(got) != 1 || got[0] != "kept" {
		t.Errorf("ReadRecent = %v, want [kept]", got)
	}
}

// TestTranscriptReader_ReadRecent_SkipsMalformedLines covers the
// "malformed line shouldn't break injection" invariant.
func TestTranscriptReader_ReadRecent_SkipsMalformedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sess_malformed.jsonl")
	// Mix of valid and malformed lines.
	content := strings.Join([]string{
		`{"t":"2026-06-28T10:00:00Z","kind":"complete","body":"first"}`,
		`this is not valid json`,
		`{"t":"2026-06-28T10:01:00Z","kind":"complete","body":"second"}`,
		``, // empty line
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	r := NewTranscriptReader(dir)
	got, err := r.ReadRecent(context.Background(), "sess_malformed", 5)
	if err != nil {
		t.Fatalf("ReadRecent: %v", err)
	}
	want := []string{"first", "second"}
	if !equalStrings(got, want) {
		t.Errorf("ReadRecent = %v, want %v", got, want)
	}
}

// TestTranscriptReader_ReadRecent_NZero_DisablesInjection covers the
// backward-compat default (PriorContextRounds=0 → no injection).
func TestTranscriptReader_ReadRecent_NZero_DisablesInjection(t *testing.T) {
	dir := t.TempDir()
	writeTranscript(t, dir, "sess_n0", []transcript.Event{
		{Kind: "complete", Body: "should-not-return"},
	})
	r := NewTranscriptReader(dir)
	got, err := r.ReadRecent(context.Background(), "sess_n0", 0)
	if err != nil {
		t.Fatalf("ReadRecent: %v", err)
	}
	if got != nil {
		t.Errorf("ReadRecent with n=0 = %v, want nil", got)
	}
}

// TestTranscriptReader_ReadRecent_NilReceiver covers the disabled path
// (orchestrator without WithTranscriptDir / WithPriorContextRounds).
func TestTranscriptReader_ReadRecent_NilReceiver(t *testing.T) {
	var r *TranscriptReader
	got, err := r.ReadRecent(context.Background(), "sess_nil", 3)
	if err != nil {
		t.Fatalf("nil.ReadRecent: %v", err)
	}
	if got != nil {
		t.Errorf("nil.ReadRecent = %v, want nil", got)
	}
}

// TestTranscriptReader_ReadRecent_CtxCancelled covers the ctx cancel path.
func TestTranscriptReader_ReadRecent_CtxCancelled(t *testing.T) {
	dir := t.TempDir()
	writeTranscript(t, dir, "sess_ctx", []transcript.Event{
		{Kind: "complete", Body: "ok"},
	})
	r := NewTranscriptReader(dir)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.ReadRecent(ctx, "sess_ctx", 3)
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("ReadRecent on cancelled ctx = %v, want context.Canceled", err)
	}
}

// TestTranscriptReader_ReadRecent_EmptySessionID is the defensive no-op.
func TestTranscriptReader_ReadRecent_EmptySessionID(t *testing.T) {
	dir := t.TempDir()
	r := NewTranscriptReader(dir)
	got, err := r.ReadRecent(context.Background(), "", 3)
	if err != nil {
		t.Fatalf("ReadRecent(empty): %v", err)
	}
	if got != nil {
		t.Errorf("ReadRecent(empty) = %v, want nil", got)
	}
}

// TestTranscriptReader_ReadRecent_LongLine covers the case where a single
// finalText exceeds the default 64KB bufio.Scanner buffer. (LLM reviews
// can easily be 200KB+.) We override the buffer to 1MB to handle.
func TestTranscriptReader_ReadRecent_LongLine(t *testing.T) {
	dir := t.TempDir()
	longBody := strings.Repeat("a", 200*1024) // 200 KB finalText
	writeTranscript(t, dir, "sess_long", []transcript.Event{
		{Kind: "complete", Body: longBody},
	})
	r := NewTranscriptReader(dir)
	got, err := r.ReadRecent(context.Background(), "sess_long", 1)
	if err != nil {
		t.Fatalf("ReadRecent: %v", err)
	}
	if len(got) != 1 || len(got[0]) != len(longBody) {
		t.Errorf("ReadRecent long body length = %d, want %d", len(got[0]), len(longBody))
	}
}

// TestTranscriptReader_BuildPriorOutputSummary_Format covers the label
// syntax exactly as documented in design.md §2.2.
func TestTranscriptReader_BuildPriorOutputSummary_Format(t *testing.T) {
	r := NewTranscriptReader("")
	got := r.BuildPriorOutputSummary([]string{"first", "second", "third"})
	want := "<prior-output-summary>\n  [turn 1] first\n  [turn 2] second\n  [turn 3] third\n</prior-output-summary>"
	if got != want {
		t.Errorf("BuildPriorOutputSummary =\n%q\nwant\n%q", got, want)
	}
}

// TestTranscriptReader_BuildPriorOutputSummary_Empty covers the empty
// input → empty string (caller checks len() before using).
func TestTranscriptReader_BuildPriorOutputSummary_Empty(t *testing.T) {
	r := NewTranscriptReader("")
	if got := r.BuildPriorOutputSummary(nil); got != "" {
		t.Errorf("BuildPriorOutputSummary(nil) = %q, want empty", got)
	}
	if got := r.BuildPriorOutputSummary([]string{}); got != "" {
		t.Errorf("BuildPriorOutputSummary([]) = %q, want empty", got)
	}
}

// TestTranscriptReader_BuildPriorOutputSummary_NilReceiver covers the
// nil-receiver path: BuildPriorOutputSummary doesn't touch r (it's a
// pure string formatter), so nil receiver must still produce output.
// This is intentional — keeps the formatter usable from anywhere.
func TestTranscriptReader_BuildPriorOutputSummary_NilReceiver(t *testing.T) {
	var r *TranscriptReader
	got := r.BuildPriorOutputSummary([]string{"x"})
	want := "<prior-output-summary>\n  [turn 1] x\n</prior-output-summary>"
	if got != want {
		t.Errorf("nil.BuildPriorOutputSummary = %q, want %q", got, want)
	}
}

// TestTranscriptReader_SanitizeSessionID_PermittedChars covers the
// allowlist matches transcript.sanitize exactly (no drift allowed).
func TestTranscriptReader_SanitizeSessionID_PermittedChars(t *testing.T) {
	cases := []struct{ in, want string }{
		{"sess_1782638991113_5000", "sess_1782638991113_5000"},
		{"sess-with-dashes", "sess-with-dashes"},
		{"sess.with.dots", "sess.with.dots"},
		{"sess/with/slashes", "sesswithslashes"},
		{"", "session"},
		{"中文", "session"},
	}
	for _, tc := range cases {
		if got := sanitizeSessionID(tc.in); got != tc.want {
			t.Errorf("sanitizeSessionID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestTranscriptReader_DefaultTranscriptDir covers the default fallback
// resolves to ~/.devrix/transcripts (we only verify the suffix since
// home dir is platform-dependent).
func TestTranscriptReader_DefaultTranscriptDir(t *testing.T) {
	got := defaultTranscriptDir()
	if got == "" {
		// UserHomeDir failed; acceptable on exotic platforms.
		t.Skip("UserHomeDir returned empty; defaultTranscriptDir is empty")
	}
	if !strings.HasSuffix(got, filepath.Join(".devrix", "transcripts")) {
		t.Errorf("defaultTranscriptDir = %q, want suffix .devrix/transcripts", got)
	}
}

// --- helpers ---

// writeTranscript appends events as NDJSON to <dir>/<sessionID>.jsonl.
// Does NOT call transcript.Writer — bypasses dir creation since reader
// is read-only.
func writeTranscript(t *testing.T, dir, sessionID string, events []transcript.Event) {
	t.Helper()
	path := filepath.Join(dir, sessionID+".jsonl")
	var sb strings.Builder
	for _, ev := range events {
		b, err := json.Marshal(ev)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		sb.Write(b)
		sb.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}