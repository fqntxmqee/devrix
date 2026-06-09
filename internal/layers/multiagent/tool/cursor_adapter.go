package tool

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
	"sync"
	"time"
)

// CursorConfig holds configuration for a Cursor Agent tool.
type CursorConfig struct {
	Name         string
	DisplayName  string
	Description  string
	Capabilities []string
	Role         string // LLM role description for tool decision
	Command      string   // CLI binary name, default "cursor"
	Args         []string // optional extra args (for testing with bash etc.)
	Model        string
	Mode         string // "force" | "plan" | "ask" | "default"
	WorkDir      string
	Timeout      time.Duration
}

// CursorAgentTool implements AgentTool for Cursor Agent CLI using
// one-shot processes with --resume for multi-turn conversations.
type CursorAgentTool struct {
	cfg     CursorConfig
	info    Info
	chatIDs map[string]string // sessionID → cursor chatID for --resume
	mu      sync.RWMutex
}

// NewCursorAgentTool creates a Cursor Agent tool.
func NewCursorAgentTool(cfg CursorConfig) *CursorAgentTool {
	if cfg.Command == "" {
		cfg.Command = "cursor"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Minute
	}
	info := Info{
		Name:         cfg.Name,
		DisplayName:  cfg.DisplayName,
		Description:  cfg.Description,
		Capabilities: cfg.Capabilities,
		Role:         cfg.Role,
	}
	return &CursorAgentTool{
		cfg:     cfg,
		info:    info,
		chatIDs: make(map[string]string),
	}
}

// Info returns the tool's identity metadata.
func (t *CursorAgentTool) Info() Info { return t.info }

// Execute sends a task to Cursor Agent and streams events until complete.
// Each call spawns a one-shot process; multi-turn uses --resume <chatID>.
func (t *CursorAgentTool) Execute(ctx context.Context, sessionID string, req Request) (<-chan Event, error) {
	workDir := t.cfg.WorkDir
	if req.WorkDir != "" {
		workDir = req.WorkDir
	}
	if workDir == "" {
		workDir = "."
	}
	workDir = filepath.Clean(workDir)
	if info, err := os.Stat(workDir); err != nil {
		return nil, fmt.Errorf("cursor: invalid workspace %q: %w", workDir, err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("cursor: workspace is not a directory: %q", workDir)
	}

	// Note: we do NOT wrap ctx with a timeout here because
	// exec.CommandContext(ctx, ...) would kill the subprocess when the
	// timeout fires via deferred cancel(). Callers are responsible for
	// passing a context with the desired deadline.

	// Build args
	var args []string
	if len(t.cfg.Args) > 0 {
		// Custom args for testing or non-standard setups
		args = t.cfg.Args
	} else {
		// Standard cursor agent args
		args = []string{"agent", "--print", "--output-format", "stream-json", "--trust"}
		switch t.cfg.Mode {
		case "force":
			args = append(args, "--force")
		case "plan":
			args = append(args, "--mode", "plan")
		case "ask":
			args = append(args, "--mode", "ask")
		}

		// Resume previous chat if available
		t.mu.RLock()
		chatID := t.chatIDs[sessionID]
		t.mu.RUnlock()
		if chatID != "" {
			args = append(args, "--resume", chatID)
		}

		if t.cfg.Model != "" {
			args = append(args, "--model", t.cfg.Model)
		}
		args = append(args, "--workspace", workDir, "--", req.Task)
	}

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
		t.mu.Lock()
		t.chatIDs[sessionID] = sid
		t.mu.Unlock()
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
		if contentType != "text" {
			continue
		}
		text, ok := m["text"].(string)
		if !ok || text == "" {
			continue
		}
		select {
		case ch <- Event{Type: "text", Content: text}:
		case <-ctx.Done():
			return false
		}
	}
	return true
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

// Stop cleans up all tracked sessions.
func (t *CursorAgentTool) Stop() {
	t.mu.Lock()
	t.chatIDs = make(map[string]string)
	t.mu.Unlock()
}

// CloseSession forgets the cursor chatID for the given session.
func (t *CursorAgentTool) CloseSession(sessionID string) {
	t.mu.Lock()
	delete(t.chatIDs, sessionID)
	t.mu.Unlock()
}

// CleanupBySessionID removes all sessions for the given D1 Session ID.
func (t *CursorAgentTool) CleanupBySessionID(sessionID string) {
	t.CloseSession(sessionID)
}
