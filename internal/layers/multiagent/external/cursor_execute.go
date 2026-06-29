package external

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// validateCursorWorkDir resolves req.WorkDir / cfg.WorkDir / ".", verifies it
// exists and is a directory, then returns the cleaned absolute path.
func validateCursorWorkDir(reqWorkDir, cfgWorkDir string) (string, error) {
	workDir := cfgWorkDir
	if reqWorkDir != "" {
		workDir = reqWorkDir
	}
	if workDir == "" {
		workDir = "."
	}
	workDir = filepath.Clean(workDir)
	info, err := os.Stat(workDir)
	if err != nil {
		return "", fmt.Errorf("cursor: invalid workspace %q: %w", workDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("cursor: workspace is not a directory: %q", workDir)
	}
	return workDir, nil
}

// buildCursorArgs assembles the CLI argv for a cursor agent invocation,
// honouring cfg.Args override, --mode flags, --resume chatID, --model, workspace + task.
// Callers run a one-shot process per call.
func buildCursorArgs(t *CursorAgentTool, workDir string, task string, chatID string) []string {
	if len(t.cfg.Args) > 0 {
		return t.cfg.Args
	}
	args := []string{"agent", "--print", "--output-format", "stream-json", "--trust"}
	switch t.cfg.Mode {
	case "force":
		args = append(args, "--force")
	case "plan":
		args = append(args, "--mode", "plan")
	case "ask":
		args = append(args, "--mode", "ask")
	}
	if chatID != "" {
		args = append(args, "--resume", chatID)
	}
	if t.cfg.Model != "" {
		args = append(args, "--model", t.cfg.Model)
	}
	args = append(args, "--workspace", workDir, "--", task)
	return args
}

// Execute sends a task to Cursor Agent and streams events until complete.
// Each call spawns a one-shot process; multi-turn uses --resume <chatID>.
func (t *CursorAgentTool) Execute(ctx context.Context, sessionID string, req Request) (<-chan Event, error) {
	workDir, err := validateCursorWorkDir(req.WorkDir, t.cfg.WorkDir)
	if err != nil {
		return nil, err
	}

	// Note: we do NOT wrap ctx with a timeout here because
	// exec.CommandContext(ctx, ...) would kill the subprocess when the
	// timeout fires via deferred cancel(). Callers are responsible for
	// passing a context with the desired deadline.

	args := buildCursorArgs(t, workDir, req.Task, t.lookupChatID(sessionID))

	cmd := exec.CommandContext(ctx, t.cfg.Command, args...)
	cmd.Dir = workDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("cursor: stdout pipe: %w", err)
	}

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	slog.Debug("cursor agent starting", "args", args)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("cursor: start: %w", err)
	}

	ch := make(chan Event, 8)
	go t.readLoop(ctx, cmd, stdout, &stderrBuf, sessionID, ch)
	return ch, nil
}

func (t *CursorAgentTool) readLoop(ctx context.Context, cmd *exec.Cmd, stdout io.ReadCloser, stderrBuf *bytes.Buffer, sessionID string, ch chan Event) {
	defer close(ch)
	defer stdout.Close()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			slog.Debug("cursor: non-JSON line", "line", line)
			continue
		}

		eventType, _ := raw["type"].(string)
		slog.Debug("cursor: event", "type", eventType)

		switch eventType {
		case "system":
			t.handleSystem(raw, sessionID)
		case "assistant":
			if !t.handleAssistant(raw, ch, ctx) {
				return
			}
		case "thinking":
			if !t.handleThinking(raw, ch, ctx) {
				return
			}
		case "tool_call":
			if !t.handleToolCall(raw, ch, ctx) {
				return
			}
		case "result":
			if t.handleResult(raw, ch, ctx) {
				return
			}
		}
	}

	_ = cmd.Wait()
	stderrMsg := strings.TrimSpace(stderrBuf.String())
	if stderrMsg != "" {
		// Log stderr for debugging but don't send to channel —
		// cursor may print things like auth prompts to stderr.
		slog.Debug("cursor: stderr", "output", stderrMsg)
	}
}

func (t *CursorAgentTool) handleSystem(raw map[string]any, sessionID string) {
	if sid, ok := raw["session_id"].(string); ok && sid != "" {
		t.storeChatID(sessionID, sid)
		slog.Debug("cursor: session started", "chat_id", sid)
	}
}

func (t *CursorAgentTool) handleAssistant(raw map[string]any, ch chan<- Event, ctx context.Context) bool {
	msg, ok := raw["message"].(map[string]any)
	if !ok {
		return true
	}
	contentArr, ok := msg["content"].([]any)
	if !ok {
		return true
	}
	for _, item := range contentArr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		contentType, _ := m["type"].(string)
		switch contentType {
		case "text":
			text, _ := m["text"].(string)
			if text == "" {
				continue
			}
			if !emitCursorEvent(ch, ctx, Event{Type: "text", Content: text}) {
				return false
			}
		case "thinking":
			text, _ := m["thinking"].(string)
			if text == "" {
				text, _ = m["text"].(string)
			}
			if text == "" {
				continue
			}
			if !emitCursorEvent(ch, ctx, Event{Type: "thinking", Content: text}) {
				return false
			}
		case "tool_use":
			label := strings.TrimSpace(fmt.Sprint(m["name"]))
			if label == "" {
				label = strings.TrimSpace(fmt.Sprint(m["text"]))
			}
			if label == "" {
				continue
			}
			if !emitCursorEvent(ch, ctx, Event{Type: "tool_use", Content: "🔧 " + label}) {
				return false
			}
		}
	}
	return true
}

func (t *CursorAgentTool) handleThinking(raw map[string]any, ch chan<- Event, ctx context.Context) bool {
	subtype, _ := raw["subtype"].(string)
	if subtype != "delta" {
		return true
	}
	text, _ := raw["text"].(string)
	text = strings.TrimSpace(text)
	if text == "" {
		return true
	}
	return emitCursorEvent(ch, ctx, Event{Type: "thinking", Content: text})
}

func (t *CursorAgentTool) handleToolCall(raw map[string]any, ch chan<- Event, ctx context.Context) bool {
	subtype, _ := raw["subtype"].(string)
	if subtype != "started" {
		return true
	}
	toolCall, ok := raw["tool_call"].(map[string]any)
	if !ok {
		return true
	}
	label := formatCursorToolCallLabel(toolCall)
	if label == "" {
		return true
	}
	return emitCursorEvent(ch, ctx, Event{Type: "tool_use", Content: label})
}

func emitCursorEvent(ch chan<- Event, ctx context.Context, evt Event) bool {
	select {
	case ch <- evt:
		return true
	case <-ctx.Done():
		return false
	}
}

func formatCursorToolCallLabel(toolCall map[string]any) string {
	for key, val := range toolCall {
		if key == "hookAdditionalContexts" || key == "toolCallId" {
			continue
		}
		if !strings.HasSuffix(key, "ToolCall") {
			continue
		}
		toolName := strings.TrimSuffix(key, "ToolCall")
		detail := cursorToolCallDetail(toolName, val)
		if detail != "" {
			return "🔧 " + toolName + ": " + detail
		}
		return "🔧 " + toolName
	}
	return ""
}

func cursorToolCallDetail(toolName string, raw any) string {
	payload, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	if desc, ok := payload["description"].(string); ok && strings.TrimSpace(desc) != "" {
		return strings.TrimSpace(desc)
	}
	args, _ := payload["args"].(map[string]any)
	if args == nil {
		return ""
	}
	switch toolName {
	case "shell":
		if cmd, ok := args["command"].(string); ok {
			return truncateCursorDetail(cmd, 120)
		}
	case "read":
		if path, ok := args["path"].(string); ok {
			return truncateCursorDetail(path, 120)
		}
	case "write", "edit", "delete":
		if path, ok := args["path"].(string); ok {
			return truncateCursorDetail(path, 120)
		}
	case "glob":
		pattern, _ := args["globPattern"].(string)
		dir, _ := args["targetDirectory"].(string)
		if pattern != "" && dir != "" {
			return truncateCursorDetail(pattern+" @ "+dir, 120)
		}
		if pattern != "" {
			return truncateCursorDetail(pattern, 120)
		}
	case "grep":
		if pattern, ok := args["pattern"].(string); ok {
			return truncateCursorDetail(pattern, 120)
		}
	}
	for _, key := range []string{"path", "command", "pattern", "query", "globPattern", "targetDirectory"} {
		if v, ok := args[key].(string); ok && strings.TrimSpace(v) != "" {
			return truncateCursorDetail(v, 120)
		}
	}
	return ""
}

func truncateCursorDetail(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// handleResult returns true if the caller should return (stream finished).
func (t *CursorAgentTool) handleResult(raw map[string]any, ch chan<- Event, ctx context.Context) bool {
	isError, _ := raw["is_error"].(bool)
	if isError {
		result, _ := raw["result"].(string)
		if result == "" {
			result = "cursor agent returned an error"
		}
		select {
		case ch <- Event{Type: "error", Content: result}:
		case <-ctx.Done():
			return true
		}
	}

	select {
	case ch <- Event{Type: "complete", Content: ""}:
	case <-ctx.Done():
	}
	return true // stream finished
}
