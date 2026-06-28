package sessionorchestrator

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devrix/devrix/internal/layers/communication/capture/transcript"
)

// TranscriptReader reads recent final-text entries from a session's
// transcript jsonl so the orchestrator can inject prior-output-summary
// into the next turn's directive.
//
// dm-20260628-003 (D7 multi-turn session state): the *write* side has been
// in place since DM-20260617-008 (transcript.Writer) and DM-20260617-002
// (appendTranscriptEvent). This is the *read* side — it filters for
// Kind=="complete" events (which carry the finalText in Body) and returns
// the most recent n entries to seed the next turn's context.
//
// Schema: transcript.Event = {t time.Time, kind string, role string,
// body string}. capture/gateway.go:880 maps EngineEvent.Type="complete"
// to Kind="complete".
//
// We mirror transcript.Writer.LoadReader's sanitize logic for the
// sessionID ([A-Za-z0-9._-] only) and the IsNotExist-returns-empty
// contract. We intentionally do NOT use transcript.NewWriter because
// the reader should be read-only and NewWriter creates the dir as a
// side effect.
type TranscriptReader struct {
	dir string
}

// NewTranscriptReader constructs a reader bound to dir. Empty dir →
// resolve to default (~/.devrix/transcripts), matching
// bootstrap.NewTranscriptWriter's fallback chain. Tests pass t.TempDir().
func NewTranscriptReader(dir string) *TranscriptReader {
	if dir == "" {
		dir = defaultTranscriptDir()
	}
	return &TranscriptReader{dir: dir}
}

// ReadRecent returns the Body of the last n events with Kind=="complete"
// from the session's transcript file. Returns ([]string{}, nil) when the
// file does not exist (fresh session). Order: chronological (oldest first
// within the returned slice), so the caller can render "turn N → text".
//
// n <= 0 → returns nil immediately (caller chose to disable injection).
func (r *TranscriptReader) ReadRecent(ctx context.Context, sessionID string, n int) ([]string, error) {
	if r == nil || sessionID == "" || n <= 0 {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.dir == "" {
		return nil, nil // no transcript dir available; inject nothing
	}
	path := filepath.Join(r.dir, sanitizeSessionID(sessionID)+".jsonl")
	events, err := readTranscriptFile(path)
	if err != nil {
		return nil, fmt.Errorf("transcript reader: load %s: %w", path, err)
	}
	var finals []string
	for _, ev := range events {
		if ev.Kind == "complete" && ev.Body != "" {
			finals = append(finals, ev.Body)
		}
	}
	if len(finals) > n {
		finals = finals[len(finals)-n:]
	}
	return finals, nil
}

// readTranscriptFile scans a jsonl file into []transcript.Event. Returns
// (nil, nil) when the file does not exist. Malformed lines are silently
// skipped (one bad event shouldn't break context injection).
func readTranscriptFile(path string) ([]transcript.Event, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var out []transcript.Event
	sc := bufio.NewScanner(f)
	// Allow long lines (default bufio.Scanner buffer is 64KB; LLM
	// finalText can exceed this for very long reviews).
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev transcript.Event
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		out = append(out, ev)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// BuildPriorOutputSummary formats texts as a <prior-output-summary> block.
// The label syntax mirrors internal/contextengine/prepare/persist/turn_output_store.go
// FoldAssistantOutput (PR #149 iter3 confirms D1 strips the label before
// IM card render, so this is safe to inject into the LLM prompt without
// leaking to user-visible output).
//
// texts[0] = oldest of the kept slice, texts[len-1] = most recent.
//
// Output shape:
//
//	<prior-output-summary>
//	  [turn 1] <texts[0]>
//	  [turn 2] <texts[1]>
//	  ...
//	</prior-output-summary>
func (r *TranscriptReader) BuildPriorOutputSummary(texts []string) string {
	if len(texts) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<prior-output-summary>\n")
	for i, t := range texts {
		fmt.Fprintf(&b, "  [turn %d] %s\n", i+1, t)
	}
	b.WriteString("</prior-output-summary>")
	return b.String()
}

// Dir returns the resolved transcript directory (post-default-fallback).
// Useful for diagnostics + tests.
func (r *TranscriptReader) Dir() string {
	if r == nil {
		return ""
	}
	return r.dir
}

// defaultTranscriptDir mirrors bootstrap.NewTranscriptWriter's fallback
// chain: ~/.devrix/transcripts when no override. Returns "" if even
// UserHomeDir fails (rare; caller treats as "no transcript dir available"
// and ReadRecent returns an empty slice).
func defaultTranscriptDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".devrix", "transcripts")
	}
	return ""
}

// sanitizeSessionID mirrors capture.transcript.sanitize (private as of
// DM-20260628-003; kept duplicated here to avoid widening the transcript
// package's exported API for a single caller). MUST be kept in sync if
// transcript.sanitize ever changes. Permitted chars: [A-Za-z0-9._-].
// Empty result → "session" (matching the writer).
func sanitizeSessionID(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_', c == '.':
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return "session"
	}
	return string(out)
}