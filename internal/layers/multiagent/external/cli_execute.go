package external

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// buildCLIPrompt marshals the user task into the stream-json wire format that
// Claude/Cursor CLI subprocesses accept on stdin. Pure helper — no I/O, no state.
func buildCLIPrompt(task string) ([]byte, error) {
	msg := map[string]any{
		"type": "user",
		"message": map[string]string{
			"role":    "user",
			"content": task,
		},
	}
	return json.Marshal(msg)
}

// writeCLIStdin sends one line to the session's stdin pipe under the session
// mutex and bumps lastUsedAt. Caller handles retry on error.
func writeCLIStdin(sess *CLISession, data []byte) error {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if _, err := fmt.Fprintln(sess.stdin, string(data)); err != nil {
		return err
	}
	sess.lastUsedAt = time.Now()
	return nil
}

// Execute sends a task to the CLI agent and streams events until the subprocess
// reports done. Owns one turn's lifetime; subsequent turns reuse the cached session.
func (t *CLIAgentTool) Execute(ctx context.Context, sessionID string, req Request) (<-chan Event, error) {
	sess, err := t.ensureSession(ctx, sessionID, req.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", t.cfg.Name, err)
	}

	data, _ := buildCLIPrompt(req.Task)

	var writeErr error
	for attempt := 0; attempt < 2; attempt++ {
		writeErr = writeCLIStdin(sess, data)
		if writeErr == nil {
			break
		}
		// Stale session (prior ctx cancel closed stdin, or --print subprocess exited).
		t.dropSession(sessionID)
		sess, err = t.ensureSession(ctx, sessionID, req.WorkDir)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", t.cfg.Name, err)
		}
	}
	if writeErr != nil {
		return nil, fmt.Errorf("%s: write stdin: %w", t.cfg.Name, writeErr)
	}

	ch := make(chan Event)
	go t.runCLIStream(ctx, ch, sessionID, sess)
	return ch, nil
}

// runCLIStream scans the session's stdout stream until done or ctx cancel. On
// abnormal exit (ctx cancel before done), drops the session so the next turn
// starts fresh. Spawns a small helper to close stdin on ctx cancel so the
// scanner unblocks.
func (t *CLIAgentTool) runCLIStream(ctx context.Context, ch chan<- Event, sessionID string, sess *CLISession) {
	defer close(ch)

	t.mu.RLock()
	scanSess := t.sessions[sessionID]
	t.mu.RUnlock()
	if scanSess == nil {
		ch <- Event{Type: "error", Content: "session terminated"}
		return
	}

	scanSess.mu.Lock()
	scanner := scanSess.stdout
	scanSess.mu.Unlock()
	if scanner == nil {
		ch <- Event{Type: "error", Content: "session terminated"}
		return
	}

	// Close stdin on context cancellation to unblock the scanner.
	go func() {
		<-ctx.Done()
		scanSess.mu.Lock()
		defer scanSess.mu.Unlock()
		if scanSess.stdin != nil {
			_ = scanSess.stdin.Close()
		}
	}()

	normalDone := false
	defer func() {
		if !normalDone {
			t.dropSession(sessionID)
		}
	}()

	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parsed := ParseStreamJSONLine(line)
		for _, evt := range parsed.Events {
			if evt.Type == "thinking" || evt.Type == "text" || evt.Type == "tool_use" {
				slog.Debug("agent tool stream event",
					"tool", t.cfg.Name,
					"session", sessionID,
					"type", evt.Type,
					"len", len(evt.Content),
				)
			}
			select {
			case ch <- evt:
			case <-ctx.Done():
				return
			}
		}
		if parsed.Done {
			normalDone = true
			if t.cfg.OneShot || !scanSess.isAlive() {
				t.dropSession(sessionID)
			}
			return
		}
	}
	if err := scanner.Err(); err != nil {
		ch <- Event{Type: "error", Content: fmt.Sprintf("stdout error: %v", err)}
	}
}
